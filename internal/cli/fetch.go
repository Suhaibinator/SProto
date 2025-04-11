package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var fetchOutputDir string

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch <namespace/module_name> <version>",
	Short: "Fetch and extract a module version artifact",
	Long: `Downloads the artifact (zip file) for a specific module version from the registry
and extracts its contents into a specified output directory.

The extracted files will be placed under the directory structure:
<output_dir>/<namespace>/<module_name>/<version>/...

Example:
  protoreg-cli fetch mycompany/user v1.0.0 --output ./protos`,
	Args: cobra.ExactArgs(2), // Requires module name and version
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()
		registryURL := viper.GetString("registry_url")
		if registryURL == "" {
			log.Fatal("Registry URL is not configured. Use --registry-url flag, PROTOREG_REGISTRY_URL env var, or 'protoreg-cli configure'.")
		}
		if fetchOutputDir == "" {
			log.Fatal("--output flag is required")
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

		client := &http.Client{}

		// Construct URL
		encodedNamespace := url.PathEscape(namespace)
		encodedModuleName := url.PathEscape(moduleName)
		encodedVersion := url.PathEscape(version) // Version might contain special chars in pre-release/build metadata
		targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s/%s/artifact", strings.TrimSuffix(registryURL, "/"), encodedNamespace, encodedModuleName, encodedVersion)
		log.Info("Fetching artifact", zap.String("url", targetURL))

		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			log.Fatal("Failed to create request", zap.Error(err))
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Failed to execute request", zap.Error(err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body) // Read body for error reporting
			handleApiError(resp.StatusCode, bodyBytes, log)
			os.Exit(1)
		}

		// Read the entire zip file into memory (for simplicity with archive/zip)
		// For very large files, streaming extraction might be better, but more complex.
		zipData, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal("Failed to read artifact zip data", zap.Error(err))
		}

		// --- Extraction Logic ---
		extractionBasePath := filepath.Join(fetchOutputDir, namespace, moduleName, version)
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
				os.MkdirAll(fpath, os.ModePerm) // Use ModePerm for simplicity, could use f.Mode()
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
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// Required flag for output directory
	fetchCmd.Flags().StringVarP(&fetchOutputDir, "output", "o", "", "Base directory to extract proto files into (required)")
	fetchCmd.MarkFlagRequired("output")
}
