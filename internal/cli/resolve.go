package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/Suhaibinator/SProto/internal/cache"    // Import cache
	"github.com/Suhaibinator/SProto/internal/compiler" // Import compiler
	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/Suhaibinator/SProto/internal/resolver" // Import resolver package
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v8" // Import mpb
	"github.com/vbauerster/mpb/v8/decor"
	"go.uber.org/zap"
)

var (
	resolveOutputDir   string
	resolveUpdateFlag  bool
	resolveNoCacheFlag bool
	resolveVersionFlag string
	resolveModuleRef   string
	resolveConfigPath  string
	resolveVerboseFlag bool
)

// resolveCmd represents the resolve command
var resolveCmd = &cobra.Command{
	Use:   "resolve [module_ref]",
	Short: "Resolve and fetch dependencies for a module",
	Long: `Resolves the dependency tree for a given module (or the module in the current directory
if sproto.yaml exists) and fetches the required artifacts into the local cache.

If a module reference is provided (in the format "namespace/name@version"), it will be used as the root module.
Otherwise, if sproto.yaml exists in the current directory, it will be used to identify the root module.

Examples:
  # Resolve dependencies for the module in the current directory (using sproto.yaml)
  protoreg-cli resolve

  # Resolve dependencies for a specific module version
  protoreg-cli resolve myorg/common@v1.0.0

  # Resolve dependencies and write resolution info to a specific directory
  protoreg-cli resolve --output ./deps-info

  # Force re-fetching dependencies even if they exist in the cache
  protoreg-cli resolve --update

  # Disable using the local cache entirely
  protoreg-cli resolve --no-cache`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()
		registryURL := viper.GetString("registry_url")
		apiToken := viper.GetString("api_token")

		if registryURL == "" {
			log.Fatal("Registry URL is not configured.")
		}

		startTime := time.Now()

		// Initialize progress bar container
		p := mpb.New(mpb.WithWidth(60))

		// --- Initialize Cache ---
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		// --- Identify the root module ---
		var namespace, moduleName, version string
		var sprotoConfig *config.SProtoConfig

		// Check if module reference is provided as argument
		if len(args) > 0 {
			resolveModuleRef = args[0]
			parts := strings.Split(resolveModuleRef, "@")
			if len(parts) != 2 {
				log.Fatal("Invalid module reference format. Expected 'namespace/name@version'.")
			}

			moduleNameParts := strings.Split(parts[0], "/")
			if len(moduleNameParts) != 2 {
				log.Fatal("Invalid module name format. Expected 'namespace/name'.")
			}

			namespace = moduleNameParts[0]
			moduleName = moduleNameParts[1]
			version = parts[1]

			// Validate version format
			_, err := semver.NewVersion(version)
			if err != nil {
				log.Fatal("Invalid version format", zap.String("version", version), zap.Error(err))
			}

			log.Info("Using module reference from command line",
				zap.String("namespace", namespace),
				zap.String("module", moduleName),
				zap.String("version", version))
		} else {
			// Look for sproto.yaml in the current directory or specified path
			configFilePath := resolveConfigPath
			if configFilePath == "" {
				configFilePath = "sproto.yaml"
			}

			if _, err := os.Stat(configFilePath); err != nil {
				if os.IsNotExist(err) {
					log.Fatal("No module reference provided and sproto.yaml not found.")
				}
				log.Fatal("Error accessing sproto.yaml", zap.String("path", configFilePath), zap.Error(err))
			}

			log.Info("Loading module information from sproto.yaml", zap.String("path", configFilePath))
			cfg, err := config.ParseConfig(configFilePath)
			if err != nil {
				log.Fatal("Failed to parse sproto.yaml", zap.Error(err))
			}

			sprotoConfig = cfg
			parts := strings.Split(cfg.Name, "/")
			if len(parts) != 2 {
				log.Fatal("Invalid module name format in sproto.yaml", zap.String("name", cfg.Name))
			}

			namespace = parts[0]
			moduleName = parts[1]

			// Use version from flag if provided, otherwise from sproto.yaml
			if resolveVersionFlag != "" {
				semVer, err := semver.NewVersion(resolveVersionFlag)
				if err != nil {
					log.Fatal("Invalid version format", zap.String("version", resolveVersionFlag), zap.Error(err))
				}
				version = "v" + semVer.String()
				log.Info("Using version from --version flag", zap.String("version", version))
			} else if cfg.Version != "" {
				version = cfg.Version
			} else {
				log.Fatal("No version specified in sproto.yaml and no --version flag provided.")
			}

			log.Info("Using module from sproto.yaml",
				zap.String("namespace", namespace),
				zap.String("module", moduleName),
				zap.String("version", version))
		}

		// --- Create registry client ---
		client := NewRegistryClient(registryURL, apiToken, log)

		// Log the client creation and URL (to avoid unused variable error)
		log.Debug("Created registry client for dependency resolution", zap.String("registry_url", client.RegistryURL))

		// Use the sprotoConfig if available for extra context
		if sprotoConfig != nil {
			log.Debug("Using sproto.yaml configuration for resolution",
				zap.String("import_path", sprotoConfig.ImportPath),
				zap.Int("declared_dependencies", len(sprotoConfig.Dependencies)))
		}

		// --- Resolve dependencies ---
		log.Info("Starting dependency resolution",
			zap.String("module", fmt.Sprintf("%s/%s@%s", namespace, moduleName, version)))

		// Add overall progress bar
		totalSteps := 1 // Placeholder: Will increase as we add steps like fetching
		bar := p.New(int64(totalSteps),
			mpb.BarStyle().Lbound("[\u001b[32m").Filler("=").Tip(">").Padding("-").Rbound("\u001b[0m]"),
			mpb.PrependDecorators(
				decor.Name("Resolving", decor.WC{W: 10}), // Removed alignment parameter C
				decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(
				decor.Percentage(decor.WC{W: 5}),
				decor.Elapsed(decor.ET_STYLE_GO, decor.WC{W: 8}),
			),
		)

		// Create a new dependency resolver, passing the progress container
		depResolver := resolver.NewDependencyResolver(client, log, p) // Pass p

		// Resolve dependencies starting from the root module
		resolvedDeps, err := depResolver.ResolveRootModule(namespace, moduleName, version)
		if err != nil {
			log.Fatal("Dependency resolution failed", zap.Error(err))
		}
		bar.Increment() // Complete the resolving bar

		// --- Fetch and Extract Dependencies ---
		log.Info("Ensuring dependencies are available in cache...")
		modulesResolved := len(resolvedDeps)
		modulesDownloaded := 0
		cacheHits := 0
		fetchBar := p.New(int64(modulesResolved),
			mpb.BarStyle().Lbound("[").Filler("=").Tip(">").Padding("-").Rbound("]"),
			mpb.PrependDecorators(
				decor.Name("Cache", decor.WC{W: 5}),
				decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(
				decor.Percentage(decor.WC{W: 5}),
			),
		)

		for moduleID, resolvedVersion := range resolvedDeps {
			depNamespace, depName, err := compiler.ParseModuleID(moduleID)
			if err != nil {
				log.Fatal("Internal error: Invalid module ID from resolver", zap.String("module_id", moduleID), zap.Error(err))
			}

			// 1. Check cache for artifact
			artifactPath, exists, err := c.GetArtifactPath(depNamespace, depName, resolvedVersion)
			if err != nil {
				log.Fatal("Error checking cache for artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
			}

			if !exists || resolveUpdateFlag {
				if exists && resolveUpdateFlag {
					log.Info("Updating artifact in cache (--update specified)", zap.String("module", moduleID), zap.String("version", resolvedVersion))
				} else {
					log.Info("Fetching artifact from registry", zap.String("module", moduleID), zap.String("version", resolvedVersion))
				}
				// Fetch artifact
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
				modulesDownloaded++
			} else {
				log.Debug("Artifact found in cache", zap.String("path", artifactPath))
				cacheHits++
			}

			// 2. Ensure files are extracted
			_, extractedExists, err := c.GetExtractedPath(depNamespace, depName, resolvedVersion)
			if err != nil {
				log.Fatal("Error checking cache for extracted files", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
			}

			if !extractedExists || resolveUpdateFlag { // Re-extract if updating
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

		// Wait for all bars to complete
		p.Wait()
		log.Info("Dependency fetching/extraction complete.")

		// --- Output resolution info if requested ---
		if resolveOutputDir != "" {
			// Create output directory if it doesn't exist
			if err := os.MkdirAll(resolveOutputDir, 0755); err != nil {
				log.Fatal("Failed to create output directory",
					zap.String("path", resolveOutputDir),
					zap.Error(err))
			}

			// Write a placeholder lock file - will be expanded in Task 3.2.2
			lockFilePath := filepath.Join(resolveOutputDir, "sproto.lock")
			lockFileContent := fmt.Sprintf("# SProto dependency lock file\n"+
				"# Generated: %s\n\n"+
				"root: %s/%s@%s\n"+
				"# Dependencies will be listed here in the full implementation\n",
				time.Now().Format(time.RFC3339),
				namespace, moduleName, version)

			if err := os.WriteFile(lockFilePath, []byte(lockFileContent), 0644); err != nil {
				log.Fatal("Failed to write lock file",
					zap.String("path", lockFilePath),
					zap.Error(err))
			}

			log.Info("Wrote placeholder lock file", zap.String("path", lockFilePath))
		}

		// --- Summary ---
		duration := time.Since(startTime).Round(time.Millisecond)
		log.Info("Dependency resolution completed",
			zap.Int("modules_resolved", modulesResolved),
			zap.Int("modules_downloaded", modulesDownloaded),
			zap.Int("cache_hits", cacheHits),
			zap.Duration("duration", duration))

		fmt.Printf("\nDependency Resolution Summary\n")
		fmt.Printf("-----------------------------\n")
		fmt.Printf("Root module: %s/%s@%s\n", namespace, moduleName, version)
		fmt.Printf("Modules resolved: %d\n", modulesResolved)
		fmt.Printf("Downloads: %d\n", modulesDownloaded)
		fmt.Printf("Cache hits: %d\n", cacheHits)
		fmt.Printf("Duration: %v\n", duration)

		if resolveOutputDir != "" {
			fmt.Printf("Resolution info written to: %s\n", resolveOutputDir)
		}
	},
}

func init() {
	rootCmd.AddCommand(resolveCmd)

	// Add flags specific to the resolve command
	resolveCmd.Flags().StringVarP(&resolveOutputDir, "output", "o", "", "Directory to write resolved dependency information")
	resolveCmd.Flags().BoolVarP(&resolveUpdateFlag, "update", "u", false, "Force re-fetching dependencies even if they exist in the cache")
	resolveCmd.Flags().BoolVar(&resolveNoCacheFlag, "no-cache", false, "Disable using the cache entirely")
	resolveCmd.Flags().StringVarP(&resolveVersionFlag, "version", "v", "", "Version to resolve (only used when resolving from sproto.yaml)")
	resolveCmd.Flags().StringVar(&resolveConfigPath, "config-path", "", "Path to sproto.yaml (defaults to ./sproto.yaml)")
	resolveCmd.Flags().BoolVarP(&resolveVerboseFlag, "verbose", "V", false, "Show verbose output during resolution")

	// Inherits --registry-url and --api-token from root persistent flags
}
