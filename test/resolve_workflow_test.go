package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Constants for test environment
const (
	testProjectPath = "./projects/resolve_test"
	userCacheDir    = ".cache/sproto" // Will be relative to home directory
)

// TestResolveWorkflow tests the dependency resolution workflow
func TestResolveWorkflow(t *testing.T) {
	// Skip this test if ENV var SKIP_DOCKER_TESTS is set
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Skipping docker-dependent test due to SKIP_DOCKER_TESTS env var")
	}

	// Make sure the test environment is running
	ensureTestEnvRunning(t) // Reuse function from end_to_end_test.go

	// Set up test registry with test modules
	setupRegistry(t)

	// Set up CLI with test registry and token
	os.Setenv("PROTOREG_REGISTRY_URL", testRegistry)
	os.Setenv("PROTOREG_API_TOKEN", testToken)

	// Get user home directory for cache paths
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err, "Failed to get user home directory")
	cacheRoot := filepath.Join(homeDir, userCacheDir)

	t.Run("Basic Dependency Resolution", func(t *testing.T) {
		// Clear cache before testing (to ensure we're fetching fresh)
		cmd := exec.Command("protoreg-cli", "cache", "clean")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to clean cache: %s", string(output))

		// 1. Run resolve command in test project directory
		cmd = exec.Command("protoreg-cli", "resolve")
		cmd.Dir = testProjectPath // Set working directory to our test project
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to resolve dependencies: %s", string(output))

		// 2. Verify correct modules are downloaded to cache
		// Should have common@v1.0.0, auth@v1.0.0, and service@v1.1.0 (latest matching version)
		expectedModules := []struct {
			namespace string
			name      string
			version   string
		}{
			{"test", "common", "v1.0.0"},
			{"test", "auth", "v1.0.0"},
			{"test", "service", "v1.1.0"}, // Should select latest version matching constraint
		}

		for _, mod := range expectedModules {
			// Check for artifact.zip
			artifactPath := filepath.Join(
				cacheRoot, "modules", mod.namespace, mod.name, mod.version, "artifact.zip",
			)
			_, err := os.Stat(artifactPath)
			assert.NoError(t, err, "Module artifact not found in cache: %s", artifactPath)

			// Check for extracted directory
			extractedPath := filepath.Join(
				cacheRoot, "modules", mod.namespace, mod.name, mod.version, "extracted",
			)
			_, err = os.Stat(extractedPath)
			assert.NoError(t, err, "Module extracted directory not found in cache: %s", extractedPath)
		}

		// 3. Verify import paths are correctly mapped in extraction directory
		// Each module should have files extracted according to its import_path
		importPathChecks := []string{
			filepath.Join(cacheRoot, "modules/test/common/v1.0.0/extracted/github.com/test/common/proto/types/primitive.proto"),
			filepath.Join(cacheRoot, "modules/test/auth/v1.0.0/extracted/github.com/test/auth/proto/user/user.proto"),
			filepath.Join(cacheRoot, "modules/test/service/v1.1.0/extracted/github.com/test/service/proto/user_service/service.proto"),
		}

		for _, path := range importPathChecks {
			_, err := os.Stat(path)
			assert.NoError(t, err, "Expected file at import path not found: %s", path)
		}
	})

	t.Run("Dependency Resolution with Update Flag", func(t *testing.T) {
		// Run resolve with update flag
		cmd := exec.Command("protoreg-cli", "resolve", "--update")
		cmd.Dir = testProjectPath
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to resolve with update flag: %s", string(output))

		// Verify it refetched modules (output should show downloading)
		assert.Contains(t, string(output), "Downloading", "Update should have refetched modules")
	})

	t.Run("Fetch with Dependencies", func(t *testing.T) {
		// Create temporary output directory
		outputDir, err := os.MkdirTemp("", "sproto-test-fetch-*")
		require.NoError(t, err)
		defer os.RemoveAll(outputDir)

		// Fetch service module with all dependencies
		cmd := exec.Command("protoreg-cli", "fetch", "test/service", "v1.1.0",
			"--output", outputDir, "--with-deps")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to fetch with dependencies: %s", string(output))

		// Verify both direct and transitive dependencies were fetched
		expectedFiles := []string{
			filepath.Join(outputDir, "test/service/v1.1.0/user_service/service.proto"),
			filepath.Join(outputDir, "test/common/v1.0.0/types/primitive.proto"),
			filepath.Join(outputDir, "test/common/v1.0.0/types/status.proto"),
			filepath.Join(outputDir, "test/auth/v1.0.0/user/user.proto"),
		}

		for _, file := range expectedFiles {
			_, err := os.Stat(file)
			assert.NoError(t, err, "Expected file not found: %s", file)
		}
	})

	t.Run("Compile with Dependencies", func(t *testing.T) {
		// Skip compilation test if protoc is not installed
		_, err := exec.LookPath("protoc")
		if err != nil {
			t.Skip("Skipping compilation test because protoc is not installed")
		}

		// Create temporary output directory for generated files
		genDir, err := os.MkdirTemp("", "sproto-test-gen-*")
		require.NoError(t, err)
		defer os.RemoveAll(genDir)

		// Run compile command in test project directory
		cmd := exec.Command("protoreg-cli", "compile", "--descriptor_set_out", filepath.Join(genDir, "api.pb"))
		cmd.Dir = testProjectPath
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to compile: %s", string(output))

		// Verify the descriptor file was generated
		_, err = os.Stat(filepath.Join(genDir, "api.pb"))
		assert.NoError(t, err, "Expected descriptor set file not found")

		// Output should contain paths to all relevant proto files
		assert.Contains(t, string(output), "api.proto", "Compile output should mention our proto file")
	})

	t.Run("Cache Operations", func(t *testing.T) {
		// 1. List cache contents
		cmd := exec.Command("protoreg-cli", "cache", "list")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to list cache: %s", string(output))

		// Verify the expected modules are listed
		assert.Contains(t, string(output), "test/common", "Cache should contain common module")
		assert.Contains(t, string(output), "test/auth", "Cache should contain auth module")
		assert.Contains(t, string(output), "test/service", "Cache should contain service module")

		// 2. Invalidate specific module
		cmd = exec.Command("protoreg-cli", "cache", "invalidate", "test/auth")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to invalidate cache entry: %s", string(output))

		// Verify the module was removed from cache
		cmd = exec.Command("protoreg-cli", "cache", "list")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)
		assert.NotContains(t, string(output), "test/auth", "Invalidated module should not be in cache list")

		// 3. Run resolve again to re-fetch the invalidated module
		cmd = exec.Command("protoreg-cli", "resolve")
		cmd.Dir = testProjectPath
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to resolve after invalidating cache: %s", string(output))

		// Verify it fetched the invalidated module
		assert.Contains(t, string(output), "auth", "Should have re-fetched invalidated module")
	})
}

// Helper function to set up test registry with test modules
func setupRegistry(t *testing.T) {
	// Check if the setup script exists
	scriptPath := "../scripts/setup_test_registry.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Fatalf("Test registry setup script not found at: %s", scriptPath)
	}

	// Run setup script
	cmd := exec.Command(scriptPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to set up test registry: %s", string(output))
	t.Log("Test registry populated with modules")
}
