package cache

import (
	"archive/zip"
	"bytes"
	"crypto/sha256" // Added import
	"encoding/hex"  // Added import
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArtifactMetadata contains information about a stored module artifact.
type ArtifactMetadata struct {
	Namespace      string    `json:"namespace"`
	Name           string    `json:"name"`
	Version        string    `json:"version"`
	ImportPath     string    `json:"import_path,omitempty"`
	DownloadedAt   time.Time `json:"downloaded_at"`
	Digest         string    `json:"digest,omitempty"` // SHA256 digest of artifact
	Size           int64     `json:"size"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	AccessCount    int       `json:"access_count"`
}

// PutArtifact stores an artifact in the cache.
func (c *Cache) PutArtifact(namespace, name, version string, reader io.Reader) error {
	if err := c.Lock(); err != nil {
		return err
	}
	defer c.Unlock()

	modulePath := c.GetModulePath(namespace, name, version)
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		return fmt.Errorf("failed to create module directory: %w", err)
	}

	// Create artifact file
	artifactPath := filepath.Join(modulePath, ArtifactFile)
	outFile, err := os.Create(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to create artifact file: %w", err)
	}
	defer outFile.Close()

	// Calculate SHA256 while writing to file
	hasher := sha256.New()
	teeReader := io.TeeReader(reader, hasher) // Read from input, write to hasher

	// Copy data from input reader to file via TeeReader
	size, err := io.Copy(outFile, teeReader)
	if err != nil {
		// Clean up partially written file on error
		outFile.Close()
		os.Remove(artifactPath)
		return fmt.Errorf("failed to write artifact data: %w", err)
	}

	// Get the SHA256 digest
	digest := hex.EncodeToString(hasher.Sum(nil))

	// Create metadata
	metadata := ArtifactMetadata{
		Namespace:      namespace,
		Name:           name,
		Version:        version,
		DownloadedAt:   time.Now(),
		Size:           size,
		LastAccessedAt: time.Now(),
		AccessCount:    0,
		Digest:         digest, // Store the calculated digest
	}

	// Try to extract import path from sproto.yaml in the zip if present
	// Need to read the artifact data again for this, or pass it along
	// Let's read the file we just wrote
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		// Log error but continue, metadata will lack import path
		fmt.Fprintf(os.Stderr, "Warning: failed to re-read artifact for import path extraction: %v\n", err)
	}
	importPath, err := extractImportPathFromZip(artifactData)
	if err == nil && importPath != "" {
		metadata.ImportPath = importPath
	}

	// Write metadata
	if err := c.saveArtifactMetadata(namespace, name, version, &metadata); err != nil {
		// Non-fatal error, just log it
		fmt.Fprintf(os.Stderr, "Warning: failed to save artifact metadata: %v\n", err)
	}

	return nil
}

// ExtractArtifact extracts an artifact from the cache to the extracted directory,
// preserving the import path structure.
func (c *Cache) ExtractArtifact(namespace, name, version string) error {
	if err := c.Lock(); err != nil {
		return err
	}
	defer c.Unlock()

	// Get artifact path
	artifactPath, exists, err := c.GetArtifactPath(namespace, name, version)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: %s/%s@%s", ErrModuleNotFound, namespace, name, version)
	}

	// Open the zip file
	zipData, err := os.ReadFile(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to read artifact file: %w", err)
	}

	// Read zip file
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %w", err)
	}

	// Get the extraction base path
	extractedPath, _, err := c.GetExtractedPath(namespace, name, version)
	if err != nil {
		return err
	}

	// Ensure the extraction directory exists
	if err := os.MkdirAll(extractedPath, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// Create a path builder to help with paths
	pathBuilder := NewPathBuilder(c)

	// Extract the config file first to get the import path
	importPath := ""
	for _, f := range zipReader.File {
		if f.Name == ConfigFile || f.Name == "./"+ConfigFile {
			// Extract the config file to the module directory
			configPath := filepath.Join(c.GetModulePath(namespace, name, version), ConfigFile)
			if err := extractZipFile(f, configPath); err != nil {
				// Non-fatal error for config extraction
				fmt.Fprintf(os.Stderr, "Warning: failed to extract config file: %v\n", err)
				continue
			}

			// Try to read the import_path from the extracted config
			configData, err := os.ReadFile(configPath)
			if err == nil {
				importPathFromConfig, err := extractImportPathFromConfig(configData)
				if err == nil && importPathFromConfig != "" {
					importPath = pathBuilder.NormalizeImportPath(importPathFromConfig)
				}
			}
			break
		}
	}

	// Default import path structure if not found in config
	if importPath == "" {
		// Use the module name as the import path base
		importPath = pathBuilder.GenerateDefaultImportPath(namespace, name)
	}

	// Create the main import path directory where files will be extracted
	importPathRoot := filepath.Join(extractedPath, importPath)
	if err := os.MkdirAll(importPathRoot, 0755); err != nil {
		return fmt.Errorf("failed to create import path directory: %w", err)
	}

	// Extract all files to the appropriate paths
	extractedCount := 0
	for _, f := range zipReader.File {
		// Skip the already extracted config file
		if f.Name == ConfigFile || f.Name == "./"+ConfigFile {
			continue
		}

		// Clean and normalize the zip entry path
		entryPath := f.Name
		if strings.HasPrefix(entryPath, "./") {
			entryPath = entryPath[2:] // Remove leading ./
		}

		// Verify the path doesn't contain traversal attempts
		if err := pathBuilder.VerifyPath("", entryPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping suspicious path: %s: %v\n", entryPath, err)
			continue
		}

		// For extracted files, place them under the import path structure
		targetPath := filepath.Join(importPathRoot, entryPath)

		// Create directory if it's a directory entry
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Extract the file
		if err := extractZipFile(f, targetPath); err != nil {
			return fmt.Errorf("failed to extract file %s: %w", f.Name, err)
		}
		extractedCount++
	}

	// Update metadata to reflect extraction and access
	metadata, err := c.getArtifactMetadata(namespace, name, version)
	if err == nil && metadata != nil {
		metadata.LastAccessedAt = time.Now()
		metadata.AccessCount++
		metadata.ImportPath = importPath // Store the determined import path
		if err := c.saveArtifactMetadata(namespace, name, version, metadata); err != nil {
			// Non-fatal error, just log it
			fmt.Fprintf(os.Stderr, "Warning: failed to update artifact metadata: %v\n", err)
		}
	}

	return nil
}

// Invalidate removes a module version or an entire module from the cache.
// If version is empty, all versions of the module are removed.
func (c *Cache) Invalidate(namespace, name, version string) error {
	if err := c.Lock(); err != nil {
		return err
	}
	defer c.Unlock()

	var path string
	if version == "" {
		// Remove entire module
		path = filepath.Join(c.RootDir, ModulesDir, namespace, name)
	} else {
		// Remove specific version
		path = c.GetModulePath(namespace, name, version)
	}

	// Check if the path exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			versionStr := ""
			if version != "" {
				versionStr = "@" + version
			}
			return fmt.Errorf("%w: %s/%s%s", ErrModuleNotFound, namespace, name, versionStr)
		}
		return err
	}

	// Remove the directory
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove module from cache: %w", err)
	}

	// If we removed the entire module and the namespace is now empty, clean it up
	if version == "" {
		// Check if namespace directory is empty
		namespacePath := filepath.Join(c.RootDir, ModulesDir, namespace)
		entries, err := os.ReadDir(namespacePath)
		if err == nil && len(entries) == 0 {
			// Remove empty namespace directory
			_ = os.Remove(namespacePath)
		}
	}

	return nil
}

// ListModules returns a list of all modules in the cache.
func (c *Cache) ListModules() ([]string, error) {
	if err := c.Lock(); err != nil {
		return nil, err
	}
	defer c.Unlock()

	modulesDir := filepath.Join(c.RootDir, ModulesDir)
	var result []string

	// Walk the modules directory
	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root modules directory
		if path == modulesDir {
			return nil
		}

		// We're only interested in directories at the namespace/name/version level
		relPath, err := filepath.Rel(modulesDir, path)
		if err != nil {
			return err
		}

		parts := filepath.SplitList(relPath)
		if len(parts) == 3 && info.IsDir() {
			// This is a version directory
			namespace := parts[0]
			name := parts[1]
			version := parts[2]

			// Check if it has an artifact.zip
			artifactPath := filepath.Join(path, ArtifactFile)
			if _, err := os.Stat(artifactPath); err == nil {
				result = append(result, fmt.Sprintf("%s/%s@%s", namespace, name, version))
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list cached modules: %w", err)
	}

	return result, nil
}

// GetCacheSize returns the total size of the cache in bytes.
func (c *Cache) GetCacheSize() (int64, error) {
	if err := c.Lock(); err != nil {
		return 0, err
	}
	defer c.Unlock()

	var totalSize int64

	err := filepath.Walk(c.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate cache size: %w", err)
	}

	return totalSize, nil
}

// Helper functions

// extractZipFile extracts a single file from a zip archive to the specified path.
func extractZipFile(f *zip.File, destPath string) error {
	// Open the file within the zip
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Create the destination file
	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer dest.Close()

	// Copy the contents
	_, err = io.Copy(dest, rc)
	return err
}

// extractImportPathFromZip tries to extract the import_path from a sproto.yaml file within the zip.
func extractImportPathFromZip(zipData []byte) (string, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", err
	}

	for _, f := range zipReader.File {
		if f.Name == ConfigFile || f.Name == "./"+ConfigFile {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			configData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}

			return extractImportPathFromConfig(configData)
		}
	}

	return "", fmt.Errorf("config file not found in zip")
}

// extractImportPathFromConfig extracts the import_path from a sproto.yaml file content.
// This is a simplified implementation and should be replaced with proper YAML parsing.
func extractImportPathFromConfig(configData []byte) (string, error) {
	// Very simplified parsing - in a real implementation you would use a YAML parser
	// This just looks for a line starting with "import_path:" and extracts the value

	// Import the config package to properly parse the sproto.yaml file
	// In a real implementation, you would:
	// cfg, err := config.ParseConfigBytes(configData)
	// if err != nil {
	//     return "", err
	// }
	// return cfg.ImportPath, nil

	// Simplified implementation for now:
	lines := bytes.Split(configData, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("import_path:")) {
			parts := bytes.SplitN(line, []byte(":"), 2)
			if len(parts) == 2 {
				importPath := string(bytes.TrimSpace(parts[1]))
				// Remove quotes if present
				if len(importPath) >= 2 && (importPath[0] == '"' || importPath[0] == '\'') &&
					(importPath[len(importPath)-1] == '"' || importPath[len(importPath)-1] == '\'') {
					importPath = importPath[1 : len(importPath)-1]
				}
				return importPath, nil
			}
		}
	}

	return "", fmt.Errorf("import_path not found in config")
}

// saveArtifactMetadata saves the metadata for an artifact.
func (c *Cache) saveArtifactMetadata(namespace, name, version string, metadata *ArtifactMetadata) error {
	metadataPath := filepath.Join(c.GetModulePath(namespace, name, version), "metadata.json")

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// getArtifactMetadata gets the metadata for an artifact.
func (c *Cache) getArtifactMetadata(namespace, name, version string) (*ArtifactMetadata, error) {
	metadataPath := filepath.Join(c.GetModulePath(namespace, name, version), "metadata.json")

	metadataJSON, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata ArtifactMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}
