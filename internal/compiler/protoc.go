package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Suhaibinator/SProto/internal/cache"
	"github.com/Suhaibinator/SProto/internal/resolver"
)

// GenerateProtoPath generates the --proto_path string for protoc.
// It includes the extracted paths of all resolved dependencies and the project's proto directory.
func GenerateProtoPath(resolvedDeps resolver.ResolvedDependencies, cache *cache.Cache, projectProtoDir string) (string, error) {
	var includePaths []string

	// Add project's proto directory first
	if projectProtoDir != "" {
		absProjectProtoDir, err := filepath.Abs(projectProtoDir)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for project proto dir: %w", err)
		}
		includePaths = append(includePaths, absProjectProtoDir)
	}

	// Add extracted paths for each resolved dependency
	for moduleID, version := range resolvedDeps {
		namespace, name, err := ParseModuleID(moduleID) // Use exported function name
		if err != nil {
			return "", fmt.Errorf("invalid module ID in resolved dependencies: %w", err)
		}

		// Get the path to the extracted files in the cache
		// We need the root of the extracted directory, not the import path within it yet
		extractedPath, exists, err := cache.GetExtractedPath(namespace, name, version)
		if err != nil {
			return "", fmt.Errorf("failed to get extracted path for %s@%s: %w", moduleID, version, err)
		}
		if !exists {
			// This shouldn't happen if resolution and fetching worked correctly
			// Consider triggering extraction here if needed? Or rely on resolve/fetch to do it.
			// For now, assume it exists if resolved.
			return "", fmt.Errorf("module %s@%s not found in cache extracted directory", moduleID, version)
		}

		// Add the extracted path to the list
		includePaths = append(includePaths, extractedPath)
	}

	// Remove duplicates (though unlikely with absolute paths)
	uniquePaths := make(map[string]struct{})
	var finalPaths []string
	for _, p := range includePaths {
		if _, exists := uniquePaths[p]; !exists {
			uniquePaths[p] = struct{}{}
			finalPaths = append(finalPaths, p)
		}
	}

	// Join paths with the OS-specific list separator
	listSeparator := ":"
	if runtime.GOOS == "windows" {
		listSeparator = ";"
	}

	return strings.Join(finalPaths, listSeparator), nil
}

// ParseModuleID splits a module ID string "namespace/name" into its components.
// Exported for use in other packages like cli.
func ParseModuleID(moduleID string) (string, string, error) {
	parts := strings.SplitN(moduleID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid module ID format: %s", moduleID)
	}
	return parts[0], parts[1], nil
}

// FindProtoFiles finds all .proto files within a given directory recursively.
func FindProtoFiles(rootDir string) ([]string, error) {
	var protoFiles []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			// Store relative path
			relPath, err := filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}
			protoFiles = append(protoFiles, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", rootDir, err)
	}
	return protoFiles, nil
}
