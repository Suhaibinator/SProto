package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Suhaibinator/SProto/internal/cache"
	"github.com/Suhaibinator/SProto/internal/compiler"
	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/Suhaibinator/SProto/internal/resolver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"go.uber.org/zap"
)

var (
	compileOutputDir  string
	compileProtocPath string
	compileProtocOpts []string
	compileSourceDir  string
	compileTemplate   string // Added flag for template name
)

// compileCmd represents the compile command
var compileCmd = &cobra.Command{
	Use:   "compile [proto_files...]",
	Short: "Compile proto files using resolved dependencies",
	Long: `Compile specified proto files or all proto files in the current module.

This command performs the following steps:
1. Resolves dependencies for the current module (using sproto.yaml or module ref).
2. Ensures all required dependencies are fetched and extracted in the cache.
3. Generates the necessary --proto_path string including the cache directories.
4. Executes the 'protoc' command with the generated paths and provided options.

If no specific proto files are provided as arguments, it attempts to compile all
.proto files found within the module's source directory (defined by sproto.yaml or './').

Examples:
  # Compile all protos in the current module (using sproto.yaml)
  protoreg-cli compile --go_out=./gen/go --go_opt=paths=source_relative

  # Compile specific proto files
  protoreg-cli compile user/v1/user.proto common/v1/types.proto --go_out=./gen/go

  # Specify a different protoc binary
  protoreg-cli compile --protoc-path=/usr/local/bin/protoc --go_out=./gen/go

  # Pass additional options directly to protoc
  protoreg-cli compile --protoc-opt="--experimental_allow_proto3_optional" --go_out=./gen/go

  # Use a generation template named 'go' defined in sproto.yaml
  protoreg-cli compile --template go
`,
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()
		registryURL := viper.GetString("registry_url")
		apiToken := viper.GetString("api_token")

		if registryURL == "" {
			log.Fatal("Registry URL is not configured.")
		}

		// --- Initialize Cache ---
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// --- Identify Root Module and Source Directory ---
		var namespace, moduleName, version string
		var sprotoConfig *config.SProtoConfig
		projectProtoDir := compileSourceDir // Use flag if provided

		// Look for sproto.yaml first
		configFilePath := "sproto.yaml" // Default path
		if _, err := os.Stat(configFilePath); err == nil {
			log.Info("Loading module information from sproto.yaml", zap.String("path", configFilePath))
			cfg, err := config.ParseConfig(configFilePath)
			if err != nil {
				log.Fatal("Failed to parse sproto.yaml", zap.Error(err))
			}
			sprotoConfig = cfg // Assign config

			parts := strings.Split(cfg.Name, "/")
			if len(parts) != 2 {
				log.Fatal("Invalid module name format in sproto.yaml", zap.String("name", cfg.Name))
			}
			namespace = parts[0]
			moduleName = parts[1]
			version = cfg.Version // Use version from config

			// Use source directory from config if not overridden by flag
			// NOTE: SourceDir is not currently part of SProtoConfig, assuming '.' for now
			// if projectProtoDir == "" && cfg.SourceDir != "" {
			// 	projectProtoDir = cfg.SourceDir
			// }
		} else if os.IsNotExist(err) {
			// If sproto.yaml is not found, we cannot proceed as we need it for module context.
			log.Fatal("sproto.yaml not found in the current directory. The 'compile' command requires sproto.yaml to determine the module context and dependencies.")
		} else {
			log.Fatal("Error accessing sproto.yaml", zap.String("path", configFilePath), zap.Error(err))
		}

		// Default project proto directory if still not set
		if projectProtoDir == "" {
			projectProtoDir = "." // Default to current directory
		}
		absProjectProtoDir, err := filepath.Abs(projectProtoDir)
		if err != nil {
			log.Fatal("Failed to get absolute path for source directory", zap.String("dir", projectProtoDir), zap.Error(err))
		}
		log.Info("Using source directory", zap.String("path", absProjectProtoDir))

		// --- Resolve Dependencies ---
		log.Info("Resolving dependencies", zap.String("module", fmt.Sprintf("%s/%s@%s", namespace, moduleName, version)))
		p := mpb.New(mpb.WithWidth(60))
		bar := p.New(1, mpb.BarStyle().Lbound("[").Filler("=").Tip(">").Padding("-").Rbound("]"),
			mpb.PrependDecorators(decor.Name("Resolving")),
			mpb.AppendDecorators(decor.Percentage()),
		)

		client := NewRegistryClient(registryURL, apiToken, log)
		depResolver := resolver.NewDependencyResolver(client, log, p)
		resolvedDeps, err := depResolver.ResolveRootModule(namespace, moduleName, version)
		if err != nil {
			log.Fatal("Dependency resolution failed", zap.Error(err))
		}
		bar.Increment()
		p.Wait()
		log.Info("Dependencies resolved successfully", zap.Int("count", len(resolvedDeps)))

		// --- Ensure Dependencies are Fetched and Extracted ---
		log.Info("Ensuring dependencies are available in cache...")
		fetchBar := p.New(int64(len(resolvedDeps)), mpb.BarStyle().Lbound("[").Filler("=").Tip(">").Padding("-").Rbound("]"),
			mpb.PrependDecorators(decor.Name("Fetching/Extracting")),
			mpb.AppendDecorators(decor.CountersNoUnit("%d / %d")),
		)

		for moduleID, resolvedVersion := range resolvedDeps {
			depNamespace, depName, err := compiler.ParseModuleID(moduleID) // Use exported function
			if err != nil {
				log.Fatal("Internal error: Invalid module ID from resolver", zap.String("module_id", moduleID), zap.Error(err))
			}

			// 1. Check if artifact exists
			artifactPath, exists, err := c.GetArtifactPath(depNamespace, depName, resolvedVersion)
			if err != nil {
				log.Fatal("Error checking cache for artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
			}

			if !exists {
				// Fetch artifact
				log.Info("Fetching artifact from registry", zap.String("module", moduleID), zap.String("version", resolvedVersion))
				artifactStream, err := client.FetchArtifact(depNamespace, depName, resolvedVersion)
				if err != nil {
					log.Fatal("Failed to fetch artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
				}
				// Store in cache
				err = c.PutArtifact(depNamespace, depName, resolvedVersion, artifactStream)
				artifactStream.Close() // Close after PutArtifact reads it
				if err != nil {
					log.Fatal("Failed to store artifact in cache", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
				}
				log.Info("Artifact stored in cache", zap.String("path", artifactPath))
			} else {
				log.Debug("Artifact found in cache", zap.String("path", artifactPath))
			}

			// 2. Check if extracted files exist
			_, extractedExists, err := c.GetExtractedPath(depNamespace, depName, resolvedVersion)
			if err != nil {
				log.Fatal("Error checking cache for extracted files", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
			}

			if !extractedExists {
				// Extract artifact
				log.Info("Extracting artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion))
				err = c.ExtractArtifact(depNamespace, depName, resolvedVersion)
				if err != nil {
					log.Fatal("Failed to extract artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
				}
			} else {
				log.Debug("Extracted files found in cache", zap.String("module", moduleID), zap.String("version", resolvedVersion))
			}
			fetchBar.Increment()
		}
		p.Wait()
		log.Info("All required dependencies are available in cache.")

		// --- Generate Proto Path ---
		protoPath, err := compiler.GenerateProtoPath(resolvedDeps, c, absProjectProtoDir)
		if err != nil {
			log.Fatal("Failed to generate --proto_path", zap.Error(err))
		}
		log.Info("Generated --proto_path", zap.String("path", protoPath))

		// --- Identify Proto Files to Compile ---
		var filesToCompile []string
		if len(args) > 0 {
			// Use files provided as arguments
			filesToCompile = args
			log.Info("Using specified proto files", zap.Strings("files", filesToCompile))
		} else {
			// Find all .proto files in the source directory
			log.Info("Finding proto files in source directory", zap.String("dir", absProjectProtoDir))
			foundFiles, err := compiler.FindProtoFiles(absProjectProtoDir)
			if err != nil {
				log.Fatal("Failed to find proto files", zap.Error(err))
			}
			if len(foundFiles) == 0 {
				log.Fatal("No .proto files found in source directory", zap.String("dir", absProjectProtoDir))
			}
			filesToCompile = foundFiles
			log.Info("Found proto files to compile", zap.Int("count", len(filesToCompile)))
		}

		// --- Construct and Execute Protoc Command ---
		protocCmdPath := compileProtocPath
		if protocCmdPath == "" {
			protocCmdPath = "protoc" // Default to assuming protoc is in PATH
		}

		protocArgs := []string{}
		protocArgs = append(protocArgs, fmt.Sprintf("--proto_path=%s", protoPath))

		// --- Determine protoc options ---
		finalProtocOpts := compileProtocOpts // Start with command-line options

		// If a template is specified, load options from sproto.yaml
		if compileTemplate != "" {
			if sprotoConfig == nil {
				log.Fatal("Cannot use --template without a valid sproto.yaml file.")
			}

			var templateConfig *config.Generate
			for _, gen := range sprotoConfig.Generate {
				if gen.Name == compileTemplate {
					templateConfig = &gen
					break
				}
			}

			if templateConfig == nil {
				log.Fatal("Generation template not found in sproto.yaml", zap.String("template", compileTemplate))
			}

			log.Info("Using generation template", zap.String("template", compileTemplate))

			// Construct options from template
			templateOpts := []string{}
			if templateConfig.Output != "" {
				// Assume options map contains the flag name (e.g., "go_out") and value is implicit
				for flagName := range templateConfig.Options {
					// Construct flag like --go_out=./gen/go
					templateOpts = append(templateOpts, fmt.Sprintf("--%s=%s", flagName, templateConfig.Output))
				}
				// Add specific options like --go_opt=paths=source_relative
				for optKey, optVal := range templateConfig.Options {
					if !strings.HasSuffix(optKey, "_out") { // Avoid duplicating output flags
						templateOpts = append(templateOpts, fmt.Sprintf("--%s=%s", optKey, optVal))
					}
				}
			} else {
				// Handle options without a single output dir (less common)
				for key, val := range templateConfig.Options {
					templateOpts = append(templateOpts, fmt.Sprintf("--%s=%s", key, val))
				}
			}

			// Add plugin options
			for _, plugin := range templateConfig.Plugins {
				// Assuming plugin format is like "--plugin=protoc-gen-go=path/to/plugin"
				templateOpts = append(templateOpts, fmt.Sprintf("--plugin=%s", plugin))
			}

			// Prepend template options so command-line options can override
			finalProtocOpts = append(templateOpts, finalProtocOpts...)
		}

		// Add final options to protocArgs
		for _, opt := range finalProtocOpts {
			// Simple split for flags like --go_out=./gen
			parts := strings.SplitN(opt, "=", 2)
			if len(parts) == 2 {
				protocArgs = append(protocArgs, fmt.Sprintf("%s=%s", parts[0], parts[1]))
			} else {
				protocArgs = append(protocArgs, opt)
			}
		}

		// Add files to compile (relative to the source dir)
		protocArgs = append(protocArgs, filesToCompile...)

		log.Info("Executing protoc",
			zap.String("command", protocCmdPath),
			zap.Strings("args", protocArgs))

		// Execute the command
		cmdExec := exec.Command(protocCmdPath, protocArgs...)
		cmdExec.Dir = absProjectProtoDir // Run protoc from the source directory
		output, err := cmdExec.CombinedOutput()

		if err != nil {
			log.Error("protoc execution failed",
				zap.Error(err),
				zap.String("output", string(output)))
			fmt.Fprintf(os.Stderr, "\n--- protoc Output ---\n%s\n--------------------\n", string(output))
			os.Exit(1)
		}

		log.Info("protoc executed successfully")
		if len(output) > 0 {
			fmt.Printf("\n--- protoc Output ---\n%s\n--------------------\n", string(output))
		}
		fmt.Println("Compilation successful.")
	},
}

func init() {
	rootCmd.AddCommand(compileCmd)

	// Flags for protoc execution
	compileCmd.Flags().StringVar(&compileOutputDir, "output_dir", ".", "Base output directory for generated files (passed via specific --*_out flags)") // Note: This is conceptual, actual output is via --*_out
	compileCmd.Flags().StringVar(&compileProtocPath, "protoc-path", "", "Path to the protoc binary (defaults to finding 'protoc' in PATH)")
	compileCmd.Flags().StringArrayVar(&compileProtocOpts, "protoc-opt", []string{}, "Options to pass directly to protoc (e.g., --go_out=./gen, --go_opt=paths=source_relative). Overrides template options.")
	compileCmd.Flags().StringVar(&compileSourceDir, "source-dir", "", "Directory containing the module's proto source files (defaults to '.' or sproto.yaml)")
	compileCmd.Flags().StringVarP(&compileTemplate, "template", "t", "", "Name of the generation template defined in sproto.yaml to use")

	// Note: We don't mark protoc-opt as required because options might come from a template.
}
