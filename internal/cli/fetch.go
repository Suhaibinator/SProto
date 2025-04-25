package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"

	// "net/http" // Replaced by RegistryClient
	// "net/url" // Replaced by RegistryClient
	"os"
	"path/filepath"
	"strings"

	"github.com/Suhaibinator/SProto/internal/cache"    // Import cache
	"github.com/Suhaibinator/SProto/internal/compiler" // Import compiler
	"github.com/Suhaibinator/SProto/internal/resolver" // Import resolver
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v8" // Import mpb
	"github.com/vbauerster/mpb/v8/decor"
	"go.uber.org/zap"
)

var (
	fetchOutputDir string
	fetchWithDeps  bool // Flag to fetch dependencies
)

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch <namespace/module_name> <version>",
	Short: "Fetch and extract a module version artifact",
	Long: `Downloads the artifact (zip file) for a specific module version from the registry
and extracts its contents into a specified output directory.

If the module has an 'import_path' defined in the registry, files will be extracted
relative to that path within the output directory:
<output_dir>/<import_path>/...

Otherwise, it falls back to the previous structure:
<output_dir>/<namespace>/<module_name>/<version>/...

Examples:
  # Fetch assuming import_path is github.com/mycompany/user
  protoreg-cli fetch mycompany/user v1.0.0 --output ./protos
  # Files extracted to ./protos/github.com/mycompany/user/...

  # Fetch module without import_path
  protoreg-cli fetch legacy/utils v1.1.0 --output ./protos
  # Files extracted to ./protos/legacy/utils/v1.1.0/...

  # Fetch module and resolve/fetch its dependencies into the cache
  protoreg-cli fetch mycompany/user v1.0.0 --output ./protos --with-deps
`,
	Args: cobra.ExactArgs(2), // Requires module name and version
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()
		registryURL := viper.GetString("registry_url")
		apiToken := viper.GetString("api_token") // Needed for client
		if registryURL == "" {
			log.Fatal("Registry URL is not configured. Use --registry-url flag, PROTOREG_REGISTRY_URL env var, or 'protoreg-cli configure'.")
		}
		if fetchOutputDir == "" {
			log.Fatal("--output flag is required")
		}

		// --- Initialize Cache ---
		c, err := cache.NewCache()
		if err != nil {
			log.Fatal("Failed to initialize cache", zap.Error(err))
		}

		moduleFullName := args[0]
		version := args[1]

		parts := strings.SplitN(moduleFullName, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Fatal("Invalid module name format. Expected 'namespace/module_name'.", zap.String("module", moduleFullName))
		}
		namespace := parts[0]
		moduleName := parts[1]

		// Validate version format (basic check)
		if !strings.HasPrefix(version, "v") {
			log.Fatal("Invalid version format: must start with 'v'", zap.String("version", version))
		}
		// More robust SemVer validation could be added here

		// Create registry client
		client := NewRegistryClient(registryURL, apiToken, log)

		// --- Fetch Module Metadata for Import Path ---
		log.Info("Fetching module metadata for import path", zap.String("module", moduleFullName))
		moduleInfo, err := client.FetchModuleMetadata(namespace, moduleName)
		if err != nil {
			log.Fatal("Failed to fetch module metadata", zap.Error(err))
		}

		importPath := ""
		useImportPath := false
		if moduleInfo != nil && moduleInfo.ImportPath != nil && *moduleInfo.ImportPath != "" {
			importPath = *moduleInfo.ImportPath
			useImportPath = true
			log.Info("Using import path for extraction", zap.String("import_path", importPath))
		} else {
			log.Warn("Module does not have an import_path defined, falling back to namespace/name/version structure")
		}

		// --- Fetch Artifact (with Cache Check) ---
		var zipData []byte
		artifactPath, exists, err := c.GetArtifactPath(namespace, moduleName, version)
		if err != nil {
			log.Fatal("Error checking cache for artifact", zap.Error(err))
		}

		// TODO: Add --update flag handling for fetch command
		fetchUpdateFlag := false // Placeholder for fetch --update flag

		if !exists || fetchUpdateFlag {
			if exists && fetchUpdateFlag {
				log.Info("Updating artifact in cache (--update specified)", zap.String("module", moduleFullName), zap.String("version", version))
			} else {
				log.Info("Fetching artifact from registry", zap.String("module", moduleFullName), zap.String("version", version))
			}
			// Fetch artifact
			artifactStream, err := client.FetchArtifact(namespace, moduleName, version)
			if err != nil {
				log.Fatal("Failed to fetch artifact", zap.Error(err))
			}
			// Read into memory to store in cache and use for extraction
			artifactBytes, readErr := io.ReadAll(artifactStream)
			artifactStream.Close() // Close immediately after reading
			if readErr != nil {
				log.Fatal("Failed to read artifact stream", zap.Error(readErr))
			}
			zipData = artifactBytes // Use fetched data

			// Store in cache
			err = c.PutArtifact(namespace, moduleName, version, bytes.NewReader(zipData))
			if err != nil {
				log.Fatal("Failed to store artifact in cache", zap.Error(err))
			}
			log.Info("Artifact stored in cache", zap.String("path", artifactPath))
		} else {
			log.Info("Using cached artifact", zap.String("path", artifactPath))
			// Read from cache
			cachedData, err := os.ReadFile(artifactPath)
			if err != nil {
				log.Fatal("Failed to read cached artifact", zap.Error(err))
			}
			zipData = cachedData // Use cached data
		}

		// --- Extraction Logic ---
		var extractionBasePath string
		if useImportPath {
			// Use filepath.Join which handles OS-specific separators
			// Clean the import path to prevent issues with leading/trailing slashes or dots
			cleanedImportPath := filepath.Clean(importPath)
			extractionBasePath = filepath.Join(fetchOutputDir, cleanedImportPath)
		} else {
			// Fallback path
			extractionBasePath = filepath.Join(fetchOutputDir, namespace, moduleName, version)
		}
		log.Info("Extracting artifact", zap.String("path", extractionBasePath))

		zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			log.Fatal("Failed to open zip archive reader", zap.Error(err))
		}

		// Ensure base directory exists
		if err := os.MkdirAll(extractionBasePath, 0755); err != nil {
			log.Fatal("Failed to create extraction directory", zap.String("path", extractionBasePath), zap.Error(err))
		}

		extractedCount := 0
		for _, f := range zipReader.File {
			fpath := filepath.Join(extractionBasePath, f.Name)

			// Basic path traversal check
			if !strings.HasPrefix(fpath, filepath.Clean(extractionBasePath)+string(os.PathSeparator)) {
				log.Fatal("Invalid file path in zip archive (potential traversal attack)", zap.String("path", f.Name))
			}

			log.Debug("Extracting file", zap.String("path", fpath))

			if f.FileInfo().IsDir() {
				// Create directory
				if err := os.MkdirAll(fpath, os.ModePerm); err != nil { // Use ModePerm for simplicity, could use f.Mode()
					log.Fatal("Failed to create directory from zip", zap.String("path", fpath), zap.Error(err))
				}
				continue
			}

			// Create containing directory if needed
			if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				log.Fatal("Failed to create directory for file", zap.String("path", fpath), zap.Error(err))
			}

			// Open the file within the zip archive
			rc, err := f.Open()
			if err != nil {
				log.Fatal("Failed to open file in zip archive", zap.String("name", f.Name), zap.Error(err))
			}

			// Create the destination file
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				rc.Close()
				log.Fatal("Failed to create destination file", zap.String("path", fpath), zap.Error(err))
			}

			// Copy contents
			_, err = io.Copy(outFile, rc)

			// Close files
			rc.Close()
			outFile.Close() // Close immediately after copy

			if err != nil {
				log.Fatal("Failed to copy file contents", zap.String("path", fpath), zap.Error(err))
			}
			extractedCount++
		}

		log.Info("Artifact extracted successfully", zap.Int("files_extracted", extractedCount), zap.String("output_dir", extractionBasePath))
		fmt.Printf("Successfully fetched and extracted %d files to %s\n", extractedCount, extractionBasePath)

		// --- Fetch Dependencies if requested ---
		if fetchWithDeps {
			log.Info("Fetching dependencies (--with-deps specified)")

			// Initialize progress bar container for dependency resolution
			p := mpb.New(mpb.WithWidth(60))
			totalSteps := 1 // Placeholder
			bar := p.New(int64(totalSteps),
				mpb.BarStyle().Lbound("[\u001b[32m").Filler("=").Tip(">").Padding("-").Rbound("\u001b[0m]"),
				mpb.PrependDecorators(
					decor.Name("Deps", decor.WC{W: 5}), // Shorter name
					decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
				),
				mpb.AppendDecorators(
					decor.Percentage(decor.WC{W: 5}),
					decor.Elapsed(decor.ET_STYLE_GO, decor.WC{W: 8}),
				),
			)

			// Create resolver
			depResolver := resolver.NewDependencyResolver(client, log, p)

			// Resolve dependencies
			resolvedDeps, err := depResolver.ResolveRootModule(namespace, moduleName, version)
			if err != nil {
				// Log error but don't necessarily fail the whole fetch command?
				// Or should we fail? Let's log an error for now.
				log.Error("Failed to resolve dependencies", zap.Error(err))
			} else {
				log.Info("Dependencies resolved", zap.Int("count", len(resolvedDeps)))
				// Iterate and fetch (placeholder)
				for moduleID, resolvedVersion := range resolvedDeps {
					// Skip the root module itself, only fetch actual dependencies
					if moduleID == moduleFullName {
						continue
					}
					depNamespace, depName, err := compiler.ParseModuleID(moduleID)
					if err != nil {
						log.Error("Internal error: Invalid module ID from resolver", zap.String("module_id", moduleID), zap.Error(err))
						continue // Skip this dependency
					}

					// Check cache for dependency artifact
					depArtifactPath, depExists, err := c.GetArtifactPath(depNamespace, depName, resolvedVersion)
					if err != nil {
						log.Error("Error checking cache for dependency artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
						continue // Skip this dependency
					}

					if !depExists || fetchUpdateFlag { // Use the same update flag for now
						if depExists && fetchUpdateFlag {
							log.Info("Updating dependency artifact in cache (--update specified)", zap.String("module", moduleID), zap.String("version", resolvedVersion))
						} else {
							log.Info("Fetching dependency artifact from registry", zap.String("module", moduleID), zap.String("version", resolvedVersion))
						}
						// Fetch artifact
						depArtifactStream, err := client.FetchArtifact(depNamespace, depName, resolvedVersion)
						if err != nil {
							log.Error("Failed to fetch dependency artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
							continue // Skip this dependency
						}
						// Store in cache
						err = c.PutArtifact(depNamespace, depName, resolvedVersion, depArtifactStream)
						depArtifactStream.Close()
						if err != nil {
							log.Error("Failed to store dependency artifact in cache", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
							continue // Skip this dependency
						}
						log.Info("Dependency artifact stored in cache", zap.String("path", depArtifactPath))
					} else {
						log.Debug("Dependency artifact found in cache", zap.String("path", depArtifactPath))
					}

					// Ensure dependency is extracted
					_, depExtractedExists, err := c.GetExtractedPath(depNamespace, depName, resolvedVersion)
					if err != nil {
						log.Error("Error checking cache for extracted dependency files", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
						continue // Skip
					}

					if !depExtractedExists || fetchUpdateFlag {
						log.Info("Extracting dependency artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion))
						err = c.ExtractArtifact(depNamespace, depName, resolvedVersion)
						if err != nil {
							log.Error("Failed to extract dependency artifact", zap.String("module", moduleID), zap.String("version", resolvedVersion), zap.Error(err))
							continue // Skip
						}
					} else {
						log.Debug("Extracted dependency files found in cache", zap.String("module", moduleID), zap.String("version", resolvedVersion))
					}
				}
			}

			// Mark progress complete and wait
			bar.Increment()
			p.Wait()
			fmt.Println("Dependency resolution and fetching (placeholder) complete.")
		}
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// Required flag for output directory
	fetchCmd.Flags().StringVarP(&fetchOutputDir, "output", "o", "", "Base directory to extract proto files into (required)")
	_ = fetchCmd.MarkFlagRequired("output")

	// Optional flag to fetch dependencies
	fetchCmd.Flags().BoolVar(&fetchWithDeps, "with-deps", false, "Resolve and fetch all dependencies into the local cache")
	// TODO: Add --update flag specific to fetch command
}
