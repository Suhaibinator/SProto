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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	publishModuleName string
	publishVersion    string
)

// publishCmd represents the publish command
var publishCmd = &cobra.Command{
	Use:   "publish <directory>",
	Short: "Publish a new module version artifact",
	Long: `Zips the contents of the specified directory (containing .proto files),
calculates its SHA256 digest, and uploads it to the registry as a new module version.

Requires --module and --version flags.
Authentication via API token is required.

Example:
  protoreg-cli publish ./path/to/protos --module mycompany/user --version v1.0.0`,
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
		if publishModuleName == "" {
			log.Fatal("--module flag is required")
		}
		if publishVersion == "" {
			log.Fatal("--version flag is required")
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

		parts := strings.SplitN(publishModuleName, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Fatal("Invalid module name format. Expected 'namespace/module_name'.", zap.String("module", publishModuleName))
		}
		namespace := parts[0]
		moduleName := parts[1]

		semVer, err := semver.NewVersion(publishVersion)
		if err != nil {
			log.Fatal("Invalid semantic version format for --version flag", zap.String("version", publishVersion), zap.Error(err))
		}
		// Ensure 'v' prefix
		versionStr := "v" + semVer.String()

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

		// Create form file field
		part, err := multipartWriter.CreateFormFile("artifact", fmt.Sprintf("%s.zip", versionStr))
		if err != nil {
			log.Fatal("Failed to create form file part", zap.Error(err))
		}

		// Write zip data to the form file field
		_, err = io.Copy(part, zipBuffer) // Copy from the zipBuffer
		if err != nil {
			log.Fatal("Failed to write zip data to multipart form", zap.Error(err))
		}

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
				fmt.Printf("  Digest: %s\n", successResp.ArtifactDigest)
				fmt.Printf("  Created At: %s\n", successResp.CreatedAt.Format(time.RFC3339))
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

	// Required flags for publish command
	publishCmd.Flags().StringVarP(&publishModuleName, "module", "m", "", "Full module name (namespace/name) (required)")
	publishCmd.Flags().StringVarP(&publishVersion, "version", "v", "", "Semantic version for the artifact (e.g., v1.2.3) (required)")
	_ = publishCmd.MarkFlagRequired("module")
	_ = publishCmd.MarkFlagRequired("version")

	// Inherits --registry-url and --api-token from root persistent flags
}
