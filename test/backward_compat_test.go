package test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	moduleNamespace = "test"
	moduleName      = "compat-test"
	moduleVersion   = "v1.0.0"
)

// TestBackwardCompatibility tests that old CLI works with new server
// and that new CLI works with servers lacking dependency features
func TestBackwardCompatibility(t *testing.T) {
	// Skip if SKIP_COMPAT_TESTS is set
	if os.Getenv("SKIP_COMPAT_TESTS") != "" {
		t.Skip("Skipping backward compatibility tests due to SKIP_COMPAT_TESTS env var")
	}

	// Make sure the test environment is running
	ensureTestEnvRunning(t) // Reuse from end_to_end_test.go

	// The test may need to be skipped if the ports don't match what we expect
	t.Logf("Checking registry availability at %s", testRegistry)

	// Check if test registry is available (quick connection test)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	_, err := client.Get(testRegistry + "/health")
	if err != nil {
		t.Skip("Test registry not available at " + testRegistry + ": " + err.Error() +
			" - this likely means the test registry is running on a different port than expected")
	}

	// Build the current CLI if needed for the second test
	currentCliPath := filepath.Join(os.TempDir(), "current-protoreg-cli")
	cmdBuild := exec.Command("go", "build", "-o", currentCliPath, "../cmd/cli")
	output, err := cmdBuild.CombinedOutput()
	require.NoError(t, err, "Failed to build current CLI: %s", string(output))

	// Get the repo root for reliable path references
	cmdRepo := exec.Command("git", "rev-parse", "--show-toplevel")
	repoRootBytes, err := cmdRepo.Output()
	require.NoError(t, err, "Failed to get repository root")
	repoRoot := strings.TrimSpace(string(repoRootBytes))

	// Define the old CLI path relative to repo root
	oldCliPath := filepath.Join(repoRoot, "test", "compat", "old_cli", "protoreg-cli")

	// First verify the old CLI exists - if not, try to build it
	if _, err := os.Stat(oldCliPath); os.IsNotExist(err) {
		// Old CLI doesn't exist, try to build it
		buildScriptPath := filepath.Join(repoRoot, "scripts", "build_old_cli.sh")
		buildCmd := exec.Command(buildScriptPath)
		output, err := buildCmd.CombinedOutput()
		require.NoError(t, err, "Failed to build old CLI: %s", string(output))

		// Verify it was built
		_, err = os.Stat(oldCliPath)
		require.NoError(t, err, "Old CLI wasn't created at expected location: %s", oldCliPath)
	}

	t.Run("Old CLI with New Server", func(t *testing.T) {
		// Create a test directory with proto files
		testDir, err := os.MkdirTemp("", "sproto-compat-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(testDir)

		// Create a simple proto file
		protoFile := filepath.Join(testDir, "test.proto")
		protoContent := `
syntax = "proto3";
package test;
message TestMessage {
  string value = 1;
}
`
		err = os.WriteFile(protoFile, []byte(protoContent), 0644)
		require.NoError(t, err)

		// Set up CLI with test registry and token
		env := []string{
			"PROTOREG_REGISTRY_URL=" + testRegistry,
			"PROTOREG_API_TOKEN=" + testToken,
		}

		// Logging registry information for debugging
		t.Logf("Using test registry: %s", testRegistry)

		// 1. Test publishing with old CLI
		publishCmd := exec.Command(oldCliPath, "publish", testDir,
			"--module", moduleNamespace+"/"+moduleName,
			"--version", moduleVersion,
			"--registry-url", testRegistry,
			"--api-token", testToken)
		publishCmd.Env = append(os.Environ(), env...)
		output, err := publishCmd.CombinedOutput()
		require.NoError(t, err, "Failed to publish with old CLI: %s", string(output))

		// 2. Test listing modules with old CLI
		listCmd := exec.Command(oldCliPath, "list",
			"--registry-url", testRegistry,
			"--api-token", testToken)
		listCmd.Env = append(os.Environ(), env...)
		output, err = listCmd.CombinedOutput()
		require.NoError(t, err, "Failed to list modules with old CLI: %s", string(output))
		// Verify the module we just published is listed
		assert.Contains(t, string(output), moduleNamespace+"/"+moduleName,
			"Published module not found in list output")

		// 3. Test fetching with old CLI
		fetchDir := filepath.Join(testDir, "fetched")
		err = os.MkdirAll(fetchDir, 0755)
		require.NoError(t, err)

		fetchCmd := exec.Command(oldCliPath, "fetch",
			moduleNamespace+"/"+moduleName, moduleVersion,
			"--output", fetchDir,
			"--registry-url", testRegistry,
			"--api-token", testToken)
		fetchCmd.Env = append(os.Environ(), env...)
		output, err = fetchCmd.CombinedOutput()
		require.NoError(t, err, "Failed to fetch with old CLI: %s", string(output))

		// Verify the file was downloaded
		_, err = os.Stat(filepath.Join(fetchDir, moduleNamespace, moduleName, moduleVersion, "test.proto"))
		assert.NoError(t, err, "Proto file not found in fetched module")

		// 4. Test new CLI with module published by old CLI
		// This verifies backward compatibility for module formats
		newFetchDir := filepath.Join(testDir, "new-fetched")
		err = os.MkdirAll(newFetchDir, 0755)
		require.NoError(t, err)

		newFetchCmd := exec.Command(currentCliPath, "fetch",
			moduleNamespace+"/"+moduleName, moduleVersion,
			"--output", newFetchDir,
			"--registry-url", testRegistry,
			"--api-token", testToken)
		newFetchCmd.Env = append(os.Environ(), env...)
		output, err = newFetchCmd.CombinedOutput()
		require.NoError(t, err, "Failed to fetch with new CLI: %s", string(output))

		// Verify the file was downloaded
		_, err = os.Stat(filepath.Join(newFetchDir, moduleNamespace, moduleName, moduleVersion, "test.proto"))
		assert.NoError(t, err, "Proto file not found in module fetched by new CLI")
	})

	t.Run("New CLI with No Dependency Features Mode", func(t *testing.T) {
		// For this test, we need to create a temporary environment variable
		// that puts the CLI into a backward compatibility mode where it doesn't
		// try to use dependency features

		// Create a test project with dependencies
		testDir, err := os.MkdirTemp("", "sproto-compat-back-*")
		require.NoError(t, err)
		defer os.RemoveAll(testDir)

		// Create a standard proto file
		protoFile := filepath.Join(testDir, "service.proto")
		protoContent := `
syntax = "proto3";
package test.service;
message Service {
  string name = 1;
}
`
		err = os.WriteFile(protoFile, []byte(protoContent), 0644)
		require.NoError(t, err)

		// Create a sproto.yaml with dependencies
		configFile := filepath.Join(testDir, "sproto.yaml")
		configContent := `
version: v1
name: test/service-compat
import_path: github.com/test/service-compat/proto

dependencies:
  - namespace: test
    name: common
    version: "v1.0.0"
    import_path: github.com/test/common/proto
`
		err = os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		// Set up CLI with test registry, token, and disable dependency features
		env := []string{
			"PROTOREG_REGISTRY_URL=" + testRegistry,
			"PROTOREG_API_TOKEN=" + testToken,
			"PROTOREG_DISABLE_DEPENDENCIES=true", // This should put the CLI in backward compatibility mode
		}

		// 1. Test publishing with new CLI in compat mode
		// It should ignore the dependencies section and just publish the module
		publishCmd := exec.Command(currentCliPath, "publish", testDir,
			"--module", "test/service-compat",
			"--version", "v1.0.0",
			"--registry-url", testRegistry,
			"--api-token", testToken,
			"--skip-deps-validation") // Add flag to bypass dependency validation
		publishCmd.Env = append(os.Environ(), env...)
		output, err := publishCmd.CombinedOutput()
		require.NoError(t, err, "Failed to publish with new CLI in compat mode: %s", string(output))

		// 2. Test fetching with new CLI in compat mode
		// It should just fetch the module without trying to resolve dependencies
		fetchDir := filepath.Join(testDir, "fetched")
		err = os.MkdirAll(fetchDir, 0755)
		require.NoError(t, err)

		fetchCmd := exec.Command(currentCliPath, "fetch",
			"test/service-compat", "v1.0.0",
			"--output", fetchDir,
			"--registry-url", testRegistry,
			"--api-token", testToken)
		fetchCmd.Env = append(os.Environ(), env...)
		output, err = fetchCmd.CombinedOutput()
		require.NoError(t, err, "Failed to fetch with new CLI in compat mode: %s", string(output))

		// Verify the file was downloaded
		_, err = os.Stat(filepath.Join(fetchDir, "test/service-compat/v1.0.0/service.proto"))
		assert.NoError(t, err, "Proto file not found in fetched module")

		// 3. Try to use dependency-specific commands, which should gracefully fail
		// or operate in a limited mode
		resolveCmd := exec.Command(currentCliPath, "resolve",
			"--registry-url", testRegistry,
			"--api-token", testToken)
		resolveCmd.Dir = testDir
		resolveCmd.Env = append(os.Environ(), env...)
		output, _ = resolveCmd.CombinedOutput()
		// We don't necessarily expect this to succeed, but it should give a clear message
		assert.Contains(t, string(output), "dependencies", "Resolve command should mention dependencies")
	})
}
