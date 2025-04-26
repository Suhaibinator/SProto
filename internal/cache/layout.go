package cache

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PathBuilder provides utilities for constructing standardized paths within the
// SProto cache and output directories, following import path conventions.
type PathBuilder struct {
	cache *Cache // Reference to the cache for root paths
}

// NewPathBuilder creates a new instance of the PathBuilder.
func NewPathBuilder(cache *Cache) *PathBuilder {
	return &PathBuilder{
		cache: cache,
	}
}

// GetExtractedFilePath returns the path where a proto file should be located in
// the extracted directory, respecting the module's import path structure.
// Parameters:
//   - namespace, name, version: Module identifiers
//   - importPath: Base import path for the module (e.g., "github.com/myorg/proto")
//   - relativeFilePath: File path within the module (e.g., "user/v1/user.proto")
func (p *PathBuilder) GetExtractedFilePath(namespace, name, version, importPath, relativeFilePath string) (string, error) {
	// Get the extraction base path
	extractedPath, _, err := p.cache.GetExtractedPath(namespace, name, version)
	if err != nil {
		return "", fmt.Errorf("failed to get extracted path: %w", err)
	}

	// Clean the import path to prevent issues with malformed paths
	cleanedImportPath := filepath.Clean(importPath)

	// Clean the relative file path to prevent path traversal issues
	cleanedRelPath := filepath.Clean(relativeFilePath)

	// If relativeFilePath starts with path separators, trim them
	cleanedRelPath = strings.TrimPrefix(cleanedRelPath, "/")
	cleanedRelPath = strings.TrimPrefix(cleanedRelPath, "\\")

	// Construct the full path
	return filepath.Join(extractedPath, cleanedImportPath, cleanedRelPath), nil
}

// GetModuleExtractedRoot returns the root directory for a module's extracted files,
// including the import path.
func (p *PathBuilder) GetModuleExtractedRoot(namespace, name, version, importPath string) (string, error) {
	extractedPath, _, err := p.cache.GetExtractedPath(namespace, name, version)
	if err != nil {
		return "", fmt.Errorf("failed to get extracted path: %w", err)
	}

	cleanedImportPath := filepath.Clean(importPath)
	return filepath.Join(extractedPath, cleanedImportPath), nil
}

// GetOutputFilePath constructs a path for a file within an output directory,
// maintaining the same structure as the module's import path.
// This is useful for `fetch` command when extracting files to a user-specified directory.
func (p *PathBuilder) GetOutputFilePath(outputDir, importPath, relativeFilePath string) string {
	// Clean the paths
	cleanedImportPath := filepath.Clean(importPath)
	cleanedRelPath := filepath.Clean(relativeFilePath)

	// Trim leading path separators
	cleanedRelPath = strings.TrimPrefix(cleanedRelPath, "/")
	cleanedRelPath = strings.TrimPrefix(cleanedRelPath, "\\")

	// Construct the full path
	return filepath.Join(outputDir, cleanedImportPath, cleanedRelPath)
}

// NormalizeImportPath ensures an import path has a standard format.
// It handles cases like missing or extra leading/trailing slashes
// and potentially invalid characters.
func (p *PathBuilder) NormalizeImportPath(importPath string) string {
	if importPath == "" {
		return ""
	}

	// Clean to ensure correct path format with consistent separators
	normalized := filepath.Clean(importPath)

	// Convert backslashes to forward slashes for consistency
	// (especially important for import paths which are usually Unix-style)
	normalized = strings.ReplaceAll(normalized, "\\", "/")

	// Remove leading and trailing slashes
	normalized = strings.Trim(normalized, "/")

	// Remove leading ./ or ../
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "../")

	return normalized
}

// GenerateDefaultImportPath creates a default import path from namespace and name
// when a module doesn't specify one explicitly.
func (p *PathBuilder) GenerateDefaultImportPath(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// SanitizePathComponent ensures a path component contains only valid characters.
// This helps prevent path traversal or system-specific issues.
func (p *PathBuilder) SanitizePathComponent(component string) string {
	// Replace potentially problematic characters
	replacer := strings.NewReplacer(
		"\\", "_", // Backslash
		"/", "_", // Forward slash
		":", "_", // Colon (problematic on Windows)
		"*", "_", // Asterisk
		"?", "_", // Question mark
		"\"", "_", // Double quote
		"<", "_", // Less than
		">", "_", // Greater than
		"|", "_", // Pipe
	)
	return replacer.Replace(component)
}

// VerifyPath checks if a path appears valid and doesn't contain
// path traversal attempts.
func (p *PathBuilder) VerifyPath(base, path string) error {
	// Clean both paths for accurate comparison
	cleanBase := filepath.Clean(base)
	fullPath := filepath.Join(cleanBase, path)
	cleanFull := filepath.Clean(fullPath)

	// Check if the full path starts with the base path
	if !strings.HasPrefix(cleanFull, cleanBase) {
		return fmt.Errorf("path traversal detected: %s is outside of %s", cleanFull, cleanBase)
	}

	return nil
}
