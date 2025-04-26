package proto

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path" // Use path package for manipulation
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse"
)

// ImportScanner uses protoparse to find import statements in .proto files.
type ImportScanner struct {
	parser *protoparse.Parser
}

// NewImportScanner creates a new scanner instance.
// It takes optional importPaths for the parser to resolve standard imports like google/protobuf/*.
func NewImportScanner(importPaths ...string) *ImportScanner {
	// Configure the parser. We might need to include paths for standard protos
	// if they aren't automatically found or if we are scanning isolated files.
	// For scanning local project files, Accessor is often sufficient.
	parser := &protoparse.Parser{
		// ImportPaths: importPaths, // Add paths if needed for resolving external imports during parse
		// Accessor: protoparse.FileContentsFromMap(map[string]string{}), // Can be used to provide file contents directly
		// Rely on default source info handling for now.
	}
	return &ImportScanner{parser: parser}
}

// ScanImports parses a single .proto file and returns a list of its import paths.
// It returns the raw import strings as found in the file.
func (s *ImportScanner) ScanImports(protoFilePath string) ([]string, error) {
	// protoparse.Parser.ParseFiles expects relative paths from one of its ImportPaths
	// or absolute paths. For simplicity here, let's assume protoFilePath is the
	// path we want to parse directly. We might need a more robust way to handle this
	// depending on how the scanner is used (e.g., relative to a project root).

	// Using ParseFilesButDoNotLink is slightly more efficient if we only need imports
	// as it avoids the linking step.
	fileDescriptors, err := s.parser.ParseFilesButDoNotLink(protoFilePath)
	if err != nil {
		// Check for specific protoparse errors if needed
		return nil, fmt.Errorf("failed to parse proto file '%s': %w", protoFilePath, err)
	}

	if len(fileDescriptors) == 0 {
		// Should not happen if ParseFilesButDoNotLink succeeds for a single file path
		return nil, fmt.Errorf("no file descriptor returned after parsing '%s'", protoFilePath)
	}

	// The result slice contains one descriptor when parsing a single file path
	fileDesc := fileDescriptors[0]
	// Access the Dependency field which is a slice of strings
	imports := fileDesc.Dependency

	// Note: This gets the list of imports as strings directly from the file descriptor.
	// If the parser couldn't find
	// an import, it might error out earlier or potentially omit it here.
	// The task asks for *extracted* imports. Let's refine this.

	// Re-parsing with a custom accessor to just read the file might be better
	// if we don't want the parser to *resolve* imports.
	// Alternative: Read the file manually and regex? Less robust.

	// Let's stick with protoparse for now, assuming it gives us the import strings
	// via GetDependencies even if resolution fails later during linking.
	// If ParseFilesButDoNotLink fails due to *unresolvable* imports, we might need
	// a different approach or configure the parser differently (e.g., ignore unknown imports).

	// For now, let's assume GetDependencies gives us the paths as written.
	log.Printf("Scanned imports for %s: %v", protoFilePath, imports)
	return imports, nil
}

// ScanDirectory recursively scans a directory for .proto files and returns a map
// where keys are file paths (relative to dirPath) and values are slices of import paths found in that file.
func (s *ImportScanner) ScanDirectory(dirPath string) (map[string][]string, error) {
	results := make(map[string][]string)
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return err // Propagate the error
		}
		if !d.IsDir() && strings.HasSuffix(path, ".proto") {
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				log.Printf("Could not get relative path for %s (base %s): %v", path, dirPath, err)
				// Use absolute path as fallback? Or skip? Let's skip for now.
				return nil
			}
			relPath = filepath.ToSlash(relPath) // Use forward slashes for consistency

			// We need to parse each file individually.
			// Configure the parser to use the directory being scanned as an import path
			// so it can potentially resolve relative imports within the directory.
			parser := &protoparse.Parser{
				ImportPaths: []string{dirPath}, // Add the root directory
				// Rely on default source info handling.
			}
			// Parse the single file. Pass the absolute path.
			fileDescriptors, parseErr := parser.ParseFilesButDoNotLink(path)
			if parseErr != nil {
				// Log error but continue scanning other files
				log.Printf("Failed to parse proto file '%s': %v", path, parseErr)
				// Store the error associated with this file? For now, just skip.
				return nil // Continue walking
			}

			if len(fileDescriptors) > 0 {
				fileDesc := fileDescriptors[0]
				// Access the Dependency field directly
				imports := fileDesc.Dependency
				// Only add if there are imports
				if len(imports) > 0 {
					results[relPath] = imports
					log.Printf("Scanned imports for %s: %v", relPath, imports)
				} else {
					results[relPath] = []string{} // Store empty slice if no imports
					log.Printf("Scanned %s: No imports found", relPath)
				}
			}
		}
		return nil // Continue walking
	})

	if err != nil {
		// This error is from filepath.WalkDir itself, not necessarily file parsing.
		return nil, fmt.Errorf("error walking directory '%s': %w", dirPath, err)
	}

	return results, nil
}

// Helper function to read file content for the parser's accessor if needed later.
func readFileContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// NormalizeImportPath takes a raw import path found in a .proto file
// and converts it to a canonical, cleaned format using forward slashes.
// Example: "google/protobuf/../protobuf/timestamp.proto" -> "google/protobuf/timestamp.proto"
// Example: ".\foo\bar.proto" -> "foo/bar.proto"
func NormalizeImportPath(importPath string) string {
	// 1. Clean the path using path.Clean (removes ., .., collapses //)
	// Note: path.Clean might add a leading '.' if the result is empty or just '.',
	// and might remove a trailing slash. It uses OS-specific separators internally.
	cleaned := path.Clean(importPath)

	// 2. Replace backslashes with forward slashes explicitly AFTER cleaning
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")

	// 3. Remove leading "./" if present
	cleaned = strings.TrimPrefix(cleaned, "./")

	// 4. Handle edge case where cleaning results in just "." or empty string
	if cleaned == "." || cleaned == "" {
		// Decide on behavior: return "" or "."? Let's return "" for now based on previous logic.
		return ""
	}

	// path.Clean removes trailing slashes.

	return cleaned
}
