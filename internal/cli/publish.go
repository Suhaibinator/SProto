package cli

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/Suhaibinator/SProto/internal/api"
	"github.com/Suhaibinator/SProto/internal/config"
	"github.com/Suhaibinator/SProto/internal/proto" // Import proto scanner
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	publishModuleName           string
	publishVersion              string
	publishConfigPath           string // New flag variable
	publishSkipImportValidation bool   // Skip import path validation
	publishValidateDeps         bool   // Validate dependencies exist in registry
	publishSkipDepsValidation   bool   // Skip dependency validation if needed
)

// publishCmd represents the publish command
var publishCmd = &cobra.Command{
	Use:   "publish <directory>",
	Short: "Publish a new module version artifact",
	Long: `Zips the contents of the specified directory (containing .proto files),
calculates its SHA256 digest, and uploads it to the registry as a new module version.

Module name, version, and dependencies can be specified via a 'sproto.yaml' file
in the root of the directory being published, or explicitly via --module and --version flags.
If 'sproto.yaml' is present, --module and --version flags are optional and override the file.

Requires authentication via API token.

Examples:
  # Publish using sproto.yaml in the current directory
  protoreg-cli publish .

  # Publish using sproto.yaml in a specific directory
  protoreg-cli publish ./path/to/protos

  # Publish overriding version from sproto.yaml
  protoreg-cli publish . --version v1.1.0

  # Publish without sproto.yaml (requires --module and --version)
  protoreg-cli publish ./path/to/protos --module mycompany/user --version v1.0.0

  # Publish using a sproto.yaml file at a custom path
  protoreg-cli publish . --config-path ./config/my-sproto.yaml

  # Validate dependency versions against the registry
  protoreg-cli publish . --validate-deps

  # Skip dependency validation
  protoreg-cli publish . --skip-deps-validation
`,
	Args: cobra.ExactArgs(1), // Requires directory path
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()
		registryURL := viper.GetString("registry_url")
		apiToken := viper.GetString("api_token") // Get token from viper (flag > env > config)

		if registryURL == "" {
			log.Fatal("Registry URL is not configured.")
		}
		if apiToken == "" {
			log.Fatal("API token is required for publishing. Use --api-token flag, PROTOREG_API_TOKEN env var, or 'protoreg-cli configure'.")
		}

		protoDir := args[0]

		// --- Validate Inputs ---
		dirInfo, err := os.Stat(protoDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatal("Input directory does not exist", zap.String("path", protoDir))
			}
			log.Fatal("Failed to stat input directory", zap.String("path", protoDir), zap.Error(err))
		}
		if !dirInfo.IsDir() {
			log.Fatal("Input path is not a directory", zap.String("path", protoDir))
		}

		// --- Load and Validate sproto.yaml ---
		var sprotoConfig *config.SProtoConfig
		configFilePath := publishConfigPath // Start with flag value
		if configFilePath == "" {
			// If flag is not set, check for sproto.yaml in the root of the protoDir
			defaultConfigPath := filepath.Join(protoDir, "sproto.yaml")
			if _, err := os.Stat(defaultConfigPath); err == nil {
				configFilePath = defaultConfigPath
				log.Debug("Found default sproto.yaml", zap.String("path", configFilePath))
			} else if !os.IsNotExist(err) {
				log.Fatal("Error checking for default sproto.yaml", zap.String("path", defaultConfigPath), zap.Error(err))
			}
		}

		if configFilePath != "" {
			log.Info("Attempting to load sproto.yaml", zap.String("path", configFilePath))
			cfg, parseErr := config.ParseConfig(configFilePath) // ParseConfig includes validation
			if parseErr != nil {
				log.Fatal("Failed to parse or validate sproto.yaml", zap.String("path", configFilePath), zap.Error(parseErr))
			}
			sprotoConfig = cfg
			log.Info("Successfully loaded and validated sproto.yaml")
		} else {
			log.Info("No sproto.yaml found or specified, relying on flags.")
		}

		// --- Determine Module Name and Version ---
		var namespace, moduleName string
		var versionStr string

		if sprotoConfig != nil {
			// Use values from config, overridden by flags if set
			parts := strings.SplitN(sprotoConfig.Name, "/", 2)
			if len(parts) != 2 {
				// Should be caught by config validation, but safety check
				log.Fatal("Invalid module name format in sproto.yaml", zap.String("name", sprotoConfig.Name))
			}
			namespace = parts[0]
			moduleName = parts[1]
			versionStr = sprotoConfig.Version // Use version from config

			// Use dependencies from config

			// Override version from flag if provided
			if publishVersion != "" {
				semVer, err := semver.NewVersion(publishVersion)
				if err != nil {
					log.Fatal("Invalid semantic version format for --version flag override", zap.String("version", publishVersion), zap.Error(err))
				}
				versionStr = "v" + semVer.String() // Ensure 'v' prefix
				log.Info("Overriding version from sproto.yaml with flag value", zap.String("version", versionStr))
			} else if versionStr == "" {
				// Version is required either in config or flag
				log.Fatal("Module version is required but not specified in sproto.yaml or via --version flag.")
			}

			// Override module name from flag if provided (less common, but support)
			if publishModuleName != "" {
				parts := strings.SplitN(publishModuleName, "/", 2)
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					log.Fatal("Invalid module name format for --module flag override. Expected 'namespace/module_name'.", zap.String("module", publishModuleName))
				}
				namespace = parts[0]
				moduleName = parts[1]
				log.Info("Overriding module name from sproto.yaml with flag value", zap.String("module", publishModuleName))
			}

		} else {
			// No sproto.yaml, --module and --version flags are required
			if publishModuleName == "" {
				log.Fatal("--module flag is required when no sproto.yaml is found.")
			}
			if publishVersion == "" {
				log.Fatal("--version flag is required when no sproto.yaml is found.")
			}

			parts := strings.SplitN(publishModuleName, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				log.Fatal("Invalid module name format. Expected 'namespace/module_name'.", zap.String("module", publishModuleName))
			}
			namespace = parts[0]
			moduleName = parts[1]

			semVer, err := semver.NewVersion(publishVersion)
			if err != nil {
				log.Fatal("Invalid semantic version format for --version flag", zap.String("version", publishVersion), zap.Error(err))
			}
			versionStr = "v" + semVer.String() // Ensure 'v' prefix

		}

		// Final check on determined values
		if namespace == "" || moduleName == "" || versionStr == "" {
			// This should ideally not be reached due to checks above, but as a safeguard
			log.Fatal("Internal error: Module name, namespace, or version could not be determined.")
		}

		// --- Validate Dependencies in Registry ---
		if sprotoConfig != nil && len(sprotoConfig.Dependencies) > 0 && !publishSkipDepsValidation {
			log.Info("Validating dependencies against registry",
				zap.Int("count", len(sprotoConfig.Dependencies)),
				zap.Bool("validate_versions", publishValidateDeps))

			// Create registry client
			client := NewRegistryClient(registryURL, apiToken, log)

			// Track validation results
			validCount := 0
			invalidDeps := make([]string, 0)

			for _, dep := range sprotoConfig.Dependencies {
				depFullName := fmt.Sprintf("%s/%s", dep.Namespace, dep.Name)
				log.Debug("Validating dependency", zap.String("dependency", depFullName), zap.String("version", dep.Version))

				// Check if the module exists in the registry
				_, err := client.FetchModuleMetadata(dep.Namespace, dep.Name)
				if err != nil {
					log.Error("Dependency not found in registry",
						zap.String("dependency", depFullName),
						zap.Error(err))
					invalidDeps = append(invalidDeps, fmt.Sprintf("%s: not found in registry", depFullName))
					continue
				}

				// Module exists, check versions if requested
				if publishValidateDeps {
					versions, err := client.FetchModuleVersions(dep.Namespace, dep.Name)
					if err != nil {
						log.Error("Failed to fetch versions for dependency",
							zap.String("dependency", depFullName),
							zap.Error(err))
						invalidDeps = append(invalidDeps, fmt.Sprintf("%s: failed to fetch versions", depFullName))
						continue
					}

					// Parse version constraint
					constraint, err := semver.NewConstraint(dep.Version)
					if err != nil {
						log.Error("Invalid version constraint",
							zap.String("dependency", depFullName),
							zap.String("constraint", dep.Version),
							zap.Error(err))
						invalidDeps = append(invalidDeps, fmt.Sprintf("%s: invalid version constraint '%s'", depFullName, dep.Version))
						continue
					}

					// Check if any available version satisfies the constraint
					satisfied := false
					for _, versionStr := range versions {
						// Remove 'v' prefix if present for semver parsing
						version := versionStr
						if strings.HasPrefix(version, "v") {
							version = versionStr[1:]
						}

						semVer, err := semver.NewVersion(version)
						if err != nil {
							log.Debug("Invalid version in registry",
								zap.String("dependency", depFullName),
								zap.String("version", versionStr),
								zap.Error(err))
							continue
						}

						if constraint.Check(semVer) {
							satisfied = true
							log.Debug("Dependency version constraint satisfied",
								zap.String("dependency", depFullName),
								zap.String("constraint", dep.Version),
								zap.String("matching_version", versionStr))
							break
						}
					}

					if !satisfied {
						log.Error("No matching version found for dependency",
							zap.String("dependency", depFullName),
							zap.String("constraint", dep.Version),
							zap.Strings("available_versions", versions))
						invalidDeps = append(invalidDeps, fmt.Sprintf("%s: no version found matching '%s'", depFullName, dep.Version))
						continue
					}
				}

				// Module exists and version constraint is satisfied (if checked)
				validCount++
				log.Info("Dependency validation successful", zap.String("dependency", depFullName))
			}

			// Report results
			log.Info("Dependency validation complete",
				zap.Int("valid", validCount),
				zap.Int("invalid", len(invalidDeps)))

			// Fail if any dependencies are invalid
			if len(invalidDeps) > 0 {
				log.Error("Dependency validation failed", zap.Strings("errors", invalidDeps))
				log.Info("To bypass dependency validation, use --skip-deps-validation")
				log.Fatal("Cannot publish module with invalid dependencies")
			}
		} else if sprotoConfig != nil && len(sprotoConfig.Dependencies) > 0 && publishSkipDepsValidation {
			log.Info("Skipping dependency validation as requested", zap.Int("dependencies", len(sprotoConfig.Dependencies)))
		}

		// --- Validate Proto Import Paths ---
		if !publishSkipImportValidation && sprotoConfig != nil {
			if sprotoConfig.ImportPath == "" {
				log.Fatal("import_path is required in sproto.yaml for import validation")
			}
			log.Info("Scanning .proto files for import validation")
			scanner := proto.NewImportScanner(protoDir)
			importsMap, err := scanner.ScanDirectory(protoDir)
			if err != nil {
				log.Fatal("Failed to scan proto directory for imports", zap.Error(err))
			}

			// Track statistics for reporting
			totalImports := 0
			resolvedImports := 0
			unresolvedImports := make([]string, 0)
			unresolvedFiles := make(map[string][]string) // Map of file -> unresolved imports

			for filePath, imps := range importsMap {
				for _, imp := range imps {
					totalImports++
					cleaned := proto.NormalizeImportPath(imp)

					// Check module import path
					if strings.HasPrefix(cleaned, sprotoConfig.ImportPath) {
						resolvedImports++
						continue
					}

					// Check dependencies
					resolved := false
					for _, dep := range sprotoConfig.Dependencies {
						if dep.ImportPath != "" && strings.HasPrefix(cleaned, dep.ImportPath) {
							resolved = true
							resolvedImports++
							break
						}
					}

					if !resolved {
						unresolvedImports = append(unresolvedImports, cleaned)
						if _, exists := unresolvedFiles[filePath]; !exists {
							unresolvedFiles[filePath] = make([]string, 0)
						}
						unresolvedFiles[filePath] = append(unresolvedFiles[filePath], cleaned)
					}
				}
			}

			// Report unresolved imports if any
			if len(unresolvedImports) > 0 {
				log.Error("Found unresolved imports",
					zap.Int("total", totalImports),
					zap.Int("resolved", resolvedImports),
					zap.Int("unresolved", len(unresolvedImports)))

				// Print details of unresolved imports by file
				for file, imports := range unresolvedFiles {
					log.Error("Unresolved imports in file",
						zap.String("file", file),
						zap.Strings("imports", imports))
				}

				// Provide hint for resolution
				log.Error("To resolve these imports:",
					zap.String("hint1", "Add required dependencies to sproto.yaml"),
					zap.String("hint2", "Run with --skip-import-validation to bypass this check"))

				log.Fatal("Cannot publish module with unresolved imports")
			}

			log.Info("All import paths validated successfully",
				zap.Int("total_imports", totalImports),
				zap.Int("resolved_imports", resolvedImports))
		} else if !publishSkipImportValidation && sprotoConfig == nil {
			log.Info("Skipping import validation: no sproto.yaml found")
		} else {
			log.Info("Skipping import validation as requested")
		}

		// --- Zip Directory & Calculate Hash ---
		log.Info("Zipping directory contents", zap.String("directory", protoDir))
		zipBuffer := new(bytes.Buffer)
		hasher := sha256.New()
		// Create a multiwriter to write to both the zip buffer and the hasher
		multiWriter := io.MultiWriter(zipBuffer, hasher)
		zipWriter := zip.NewWriter(multiWriter)

		err = filepath.Walk(protoDir, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error accessing path %q: %w", filePath, err)
			}

			// Skip the root directory itself
			if filePath == protoDir {
				return nil
			}

			// Create a relative path for the file header
			relPath, err := filepath.Rel(protoDir, filePath)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %q: %w", filePath, err)
			}
			// Use forward slashes for zip header names
			headerName := filepath.ToSlash(relPath)

			// Get header from file info
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return fmt.Errorf("failed to create zip header for %q: %w", filePath, err)
			}
			header.Name = headerName
			header.Method = zip.Deflate // Use compression

			// If it's a directory, add the trailing slash
			if info.IsDir() {
				header.Name += "/"
				// No need to write content for directories
				_, err = zipWriter.CreateHeader(header)
				if err != nil {
					return fmt.Errorf("failed to write zip directory header for %q: %w", headerName, err)
				}
				log.Debug("Added directory to zip", zap.String("path", headerName))
				return nil // Don't try to open/copy directory content
			}

			// It's a file, create the header
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return fmt.Errorf("failed to write zip file header for %q: %w", headerName, err)
			}

			// Open the original file
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file %q: %w", filePath, err)
			}
			defer file.Close()

			// Copy the file content into the zip writer
			_, err = io.Copy(writer, file)
			if err != nil {
				return fmt.Errorf("failed to copy file content for %q: %w", headerName, err)
			}
			log.Debug("Added file to zip", zap.String("path", headerName))
			return nil
		})

		if err != nil {
			log.Fatal("Failed during directory walk/zip creation", zap.Error(err))
		}

		// Close the zip writer *before* getting the hash
		err = zipWriter.Close()
		if err != nil {
			log.Fatal("Failed to close zip writer", zap.Error(err))
		}

		// Get the final hash
		artifactDigestHex := hex.EncodeToString(hasher.Sum(nil))
		log.Info("Artifact zipped and digest calculated", zap.String("sha256", artifactDigestHex))

		// --- Prepare HTTP Request ---
		// Use the zipBuffer containing the zipped data
		body := &bytes.Buffer{}
		multipartWriter := multipart.NewWriter(body)

		// Create form file field for the artifact
		part, err := multipartWriter.CreateFormFile("artifact", fmt.Sprintf("%s.zip", versionStr))
		if err != nil {
			log.Fatal("Failed to create form file part for artifact", zap.Error(err))
		}

		// Write zip data to the form file field
		_, err = io.Copy(part, zipBuffer) // Copy from the zipBuffer
		if err != nil {
			log.Fatal("Failed to write zip data to multipart form", zap.Error(err))
		}

		// Add sproto.yaml content as a separate form field if available
		// NOTE: The API handler expects the artifact itself to contain sproto.yaml
		// It does not currently read a separate form field for the config.
		// We will rely on the handler extracting it from the zip.
		// If we needed to send it separately, the API handler would need modification.
		// if sprotoConfig != nil {
		// 	configPart, err := multipartWriter.CreateFormFile("sproto_config", "sproto.yaml")
		// 	if err != nil {
		// 		log.Fatal("Failed to create form file part for sproto.yaml", zap.Error(err))
		// 	}
		// 	// Marshal the config back to YAML bytes
		// 	configBytes, err := json.Marshal(sprotoConfig) // Use json.Marshal for simplicity, API expects JSON in body
		// 	if err != nil {
		// 		log.Fatal("Failed to marshal sproto config to JSON", zap.Error(err))
		// 	}
		// 	_, err = configPart.Write(configBytes)
		// 	if err != nil {
		// 		log.Fatal("Failed to write sproto config to multipart form", zap.Error(err))
		// 	}
		// 	log.Debug("Added sproto.yaml to multipart form")
		// }

		// Close multipart writer to finalize boundary
		err = multipartWriter.Close()
		if err != nil {
			log.Fatal("Failed to close multipart writer", zap.Error(err))
		}

		// Construct URL
		encodedNamespace := url.PathEscape(namespace)
		encodedModuleName := url.PathEscape(moduleName)
		encodedVersion := url.PathEscape(versionStr)
		targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s/%s", strings.TrimSuffix(registryURL, "/"), encodedNamespace, encodedModuleName, encodedVersion)
		log.Info("Publishing artifact", zap.String("url", targetURL))

		req, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			log.Fatal("Failed to create request", zap.Error(err))
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

		// --- Execute Request ---
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Failed to execute request", zap.Error(err))
		}
		defer resp.Body.Close()

		respBodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal("Failed to read response body", zap.Error(err))
		}

		// --- Handle Response ---
		if resp.StatusCode == http.StatusCreated {
			var successResp api.PublishModuleVersionResponse // Use struct from api package if accessible, otherwise redefine
			if err := json.Unmarshal(respBodyBytes, &successResp); err != nil {
				log.Error("Published successfully, but failed to parse success response", zap.Error(err), zap.ByteString("body", respBodyBytes))
				fmt.Printf("Successfully published %s/%s@%s (Digest: sha256:%s)\n", namespace, moduleName, versionStr, artifactDigestHex)
			} else {
				fmt.Printf("Successfully published %s/%s@%s\n", successResp.Namespace, successResp.ModuleName, successResp.Version)
				if successResp.ImportPath != nil {
					fmt.Printf("  Import Path: %s\n", *successResp.ImportPath)
				}
				fmt.Printf("  Digest: %s\n", successResp.ArtifactDigest)
				fmt.Printf("  Created At: %s\n", successResp.CreatedAt.Format(time.RFC3339))
				if len(successResp.Dependencies) > 0 {
					fmt.Println("  Dependencies:")
					for _, dep := range successResp.Dependencies {
						fmt.Printf("    - %s/%s (%s) [Import Path: %s]\n", dep.Namespace, dep.Name, dep.VersionConstraint, dep.ImportPath)
					}
				}
			}
		} else {
			log.Error("Publish request failed", zap.Int("status_code", resp.StatusCode))
			handleApiError(resp.StatusCode, respBodyBytes, log) // Use the helper
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(publishCmd)

	// Flags are now optional if sproto.yaml is used
	publishCmd.Flags().StringVarP(&publishModuleName, "module", "m", "", "Full module name (namespace/name) (optional if sproto.yaml is used)")
	publishCmd.Flags().StringVarP(&publishVersion, "version", "v", "", "Semantic version for the artifact (e.g., v1.2.3) (optional if sproto.yaml is used)")
	publishCmd.Flags().StringVar(&publishConfigPath, "config-path", "", "Path to a sproto.yaml configuration file (defaults to ./sproto.yaml if exists)")
	publishCmd.Flags().BoolVar(&publishSkipImportValidation, "skip-import-validation", false, "Skip scanning and validating .proto import paths")
	publishCmd.Flags().BoolVar(&publishValidateDeps, "validate-deps", false, "Validate dependency version constraints against registry")
	publishCmd.Flags().BoolVar(&publishSkipDepsValidation, "skip-deps-validation", false, "Skip validating dependencies against registry")

	// We no longer mark module/version as required here; validation happens in Run based on config presence.

	// Inherits --registry-url and --api-token from root persistent flags
}
