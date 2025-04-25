package proto

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create temporary proto files for testing
func createTempProtoFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write temp proto file %s", filename)
	return filePath
}

func TestImportScanner_ScanImports(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewImportScanner() // Use default scanner

	// Test case 1: Basic imports
	protoContent1 := `
syntax = "proto3";
package test;

import "google/protobuf/timestamp.proto";
import "another/package/file.proto";
`
	filePath1 := createTempProtoFile(t, tempDir, "test1.proto", protoContent1)
	imports1, err1 := scanner.ScanImports(filePath1)
	assert.NoError(t, err1)
	assert.ElementsMatch(t, []string{"google/protobuf/timestamp.proto", "another/package/file.proto"}, imports1)

	// Test case 2: No imports
	protoContent2 := `
syntax = "proto3";
package test.noimports;

message Simple {}
`
	filePath2 := createTempProtoFile(t, tempDir, "test2.proto", protoContent2)
	imports2, err2 := scanner.ScanImports(filePath2)
	assert.NoError(t, err2)
	assert.Empty(t, imports2)

	// Test case 3: Public and weak imports (protoparse should still list them)
	protoContent3 := `
syntax = "proto3";
package test.compleximports;

import public "google/protobuf/duration.proto";
import weak "google/protobuf/any.proto"; // Note: weak imports are proto2, but parser might handle syntax
import "regular/import.proto";
`
	filePath3 := createTempProtoFile(t, tempDir, "test3.proto", protoContent3)
	imports3, err3 := scanner.ScanImports(filePath3)
	assert.NoError(t, err3)
	assert.ElementsMatch(t, []string{"google/protobuf/duration.proto", "google/protobuf/any.proto", "regular/import.proto"}, imports3)

	// Test case 4: File not found (should error)
	_, err4 := scanner.ScanImports(filepath.Join(tempDir, "nonexistent.proto"))
	assert.Error(t, err4)
	assert.Contains(t, err4.Error(), "failed to parse proto file") // protoparse wraps the file not found

	// Test case 5: Syntax error (should error)
	protoContent5 := `
syntax = "proto3" // Missing semicolon
package test.syntaxerror;
import "google/protobuf/empty.proto";
`
	filePath5 := createTempProtoFile(t, tempDir, "test5.proto", protoContent5)
	_, err5 := scanner.ScanImports(filePath5)
	assert.Error(t, err5)
	assert.Contains(t, err5.Error(), "failed to parse proto file") // protoparse reports syntax errors
}

func TestImportScanner_ScanDirectory(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewImportScanner()

	// Create nested structure
	subDir1 := filepath.Join(tempDir, "subdir1")
	subDir2 := filepath.Join(tempDir, "subdir2")
	err := os.Mkdir(subDir1, 0755)
	require.NoError(t, err)
	err = os.Mkdir(subDir2, 0755)
	require.NoError(t, err)

	// File 1 (root)
	createTempProtoFile(t, tempDir, "root.proto", `
syntax = "proto3";
import "subdir1/file1.proto";
import "google/protobuf/empty.proto";
`)

	// File 2 (subdir1)
	createTempProtoFile(t, subDir1, "file1.proto", `
syntax = "proto3";
import "subdir2/file2.proto"; // Relative within the scanned root
`)

	// File 3 (subdir2) - no imports
	createTempProtoFile(t, subDir2, "file2.proto", `
syntax = "proto3";
message Message2 {}
`)

	// File 4 (subdir1) - parse error
	createTempProtoFile(t, subDir1, "bad.proto", `syntax = "proto3"`)

	// File 5 (root) - no imports
	createTempProtoFile(t, tempDir, "no_imports.proto", `syntax = "proto3";`)

	// Non-proto file
	createTempProtoFile(t, tempDir, "not_proto.txt", `hello`)

	results, err := scanner.ScanDirectory(tempDir)
	assert.NoError(t, err)
	require.NotNil(t, results)

	// Check results (using forward slashes for keys)
	assert.Len(t, results, 4, "Should find 4 proto files (bad.proto is skipped due to parse error)")

	// root.proto
	assert.Contains(t, results, "root.proto")
	assert.ElementsMatch(t, []string{"subdir1/file1.proto", "google/protobuf/empty.proto"}, results["root.proto"])

	// subdir1/file1.proto
	assert.Contains(t, results, "subdir1/file1.proto")
	assert.ElementsMatch(t, []string{"subdir2/file2.proto"}, results["subdir1/file1.proto"])

	// subdir2/file2.proto
	assert.Contains(t, results, "subdir2/file2.proto")
	assert.Empty(t, results["subdir2/file2.proto"])

	// no_imports.proto
	assert.Contains(t, results, "no_imports.proto")
	assert.Empty(t, results["no_imports.proto"])

	// bad.proto should not be in the results map because parsing failed
	assert.NotContains(t, results, "subdir1/bad.proto")
}

func TestNormalizeImportPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"google/protobuf/timestamp.proto", "google/protobuf/timestamp.proto"},
		{"./google/protobuf/timestamp.proto", "google/protobuf/timestamp.proto"},
		{"../foo/bar.proto", "../foo/bar.proto"}, // path.Clean keeps leading ..
		{"foo/../bar/baz.proto", "bar/baz.proto"},
		{"foo/./bar.proto", "foo/bar.proto"},
		{`windows\style\path.proto`, "windows/style/path.proto"},
		{`.\windows\style\path.proto`, "windows/style/path.proto"},
		{"foo//bar.proto", "foo/bar.proto"},
		{"/", "/"}, // path.Clean keeps root slash
		{".", ""},  // Special case for current dir
		{"", ""},   // Empty input
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := NormalizeImportPath(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
