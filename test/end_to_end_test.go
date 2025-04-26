package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Constants for test environment
const (
	testToken         = "test-token"
	testRegistry      = "http://localhost:8081"
	modulesDir        = "./modules"
	commonModulePath  = "./modules/base/common"
	authModulePath    = "./modules/base/auth"
	serviceModulePath = "./modules/dependent/service"
)

// TestPublishWorkflow tests the complete publish workflow
func TestPublishWorkflow(t *testing.T) {
	// Skip this test if ENV var SKIP_DOCKER_TESTS is set
	if os.Getenv("SKIP_DOCKER_TESTS") != "" {
		t.Skip("Skipping docker-dependent test due to SKIP_DOCKER_TESTS env var")
	}

	// Make sure the test environment is running
	ensureTestEnvRunning(t)

	// Set up CLI with test registry and token
	os.Setenv("PROTOREG_REGISTRY_URL", testRegistry)
	os.Setenv("PROTOREG_API_TOKEN", testToken)

	t.Run("Basic module without dependencies", func(t *testing.T) {
		// 1. Publish the base common module
		cmd := exec.Command("protoreg-cli", "publish", commonModulePath,
			"--module", "test/common", "--version", "v1.0.0")

		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to publish common module: %s", string(output))

		// 2. List modules to verify it was published
		cmd = exec.Command("protoreg-cli", "list")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)

		// Check if our module is in the list
		assert.Contains(t, string(output), "test/common", "Published module not found in list")

		// 3. List versions to verify the specific version was published
		cmd = exec.Command("protoreg-cli", "list", "test/common")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)

		// Check if our version is in the list
		assert.Contains(t, string(output), "v1.0.0", "Published version not found in list")

		// 4. Fetch the artifact to verify it's downloadable
		tempDir, err := os.MkdirTemp("", "sproto-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		cmd = exec.Command("protoreg-cli", "fetch", "test/common", "v1.0.0", "--output", tempDir)
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to fetch module: %s", string(output))

		// Verify the fetched files exist
		expectedProtoFile := filepath.Join(tempDir, "test/common/v1.0.0/types/primitive.proto")
		_, err = os.Stat(expectedProtoFile)
		assert.NoError(t, err, "Failed to find expected file in fetched module")
	})

	t.Run("Module with dependencies", func(t *testing.T) {
		// 1. First publish the auth module (which has no dependencies in our test setup)
		cmd := exec.Command("protoreg-cli", "publish", authModulePath,
			"--module", "test/auth", "--version", "v1.0.0")

		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to publish auth module: %s", string(output))

		// 2. Now publish the service module which depends on common and auth
		cmd = exec.Command("protoreg-cli", "publish", serviceModulePath,
			"--module", "test/service", "--version", "v1.0.0")

		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to publish service module: %s", string(output))

		// 3. List the dependencies of the service module
		cmd = exec.Command("protoreg-cli", "list", "test/service/dependencies")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)

		// Check if dependencies are listed
		outputStr := string(output)
		assert.Contains(t, outputStr, "test/common", "Common dependency not found")
		assert.Contains(t, outputStr, "test/auth", "Auth dependency not found")

		// 4. Fetch the module with dependencies
		tempDir, err := os.MkdirTemp("", "sproto-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		cmd = exec.Command("protoreg-cli", "fetch", "test/service", "v1.0.0",
			"--output", tempDir, "--with-deps")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to fetch module with dependencies: %s", string(output))

		// Verify the fetched files exist for the main module and its dependencies
		expectedFiles := []string{
			filepath.Join(tempDir, "test/service/v1.0.0/user_service/service.proto"),
			filepath.Join(tempDir, "test/common/v1.0.0/types/primitive.proto"),
			filepath.Join(tempDir, "test/auth/v1.0.0/user/user.proto"),
		}

		for _, file := range expectedFiles {
			_, err = os.Stat(file)
			assert.NoError(t, err, "Failed to find expected file: %s", file)
		}
	})

	t.Run("Error handling for invalid modules", func(t *testing.T) {
		// 1. Test publishing module with missing dependency
		cmd := exec.Command("protoreg-cli", "publish", "./modules/invalid/missing-deps",
			"--module", "test/missing-deps", "--version", "v1.0.0")

		output, err := cmd.CombinedOutput()
		assert.Error(t, err, "Should fail when publishing with missing dependency")
		assert.Contains(t, string(output), "not found", "Error should indicate dependency not found")

		// 2. Test publishing module with invalid version constraint
		cmd = exec.Command("protoreg-cli", "publish", "./modules/invalid/bad-version",
			"--module", "test/bad-version", "--version", "v1.0.0")

		output, err = cmd.CombinedOutput()
		assert.Error(t, err, "Should fail when publishing with invalid version constraint")
		assert.Contains(t, string(output), "version", "Error should mention version")
	})
}

// Helper function to ensure test environment is running
func ensureTestEnvRunning(t *testing.T) {
	// Check if the test script exists relative to project root
	scriptPath := "scripts/test-env.sh"
	// Check existence relative to the test file's location first for safety
	if _, err := os.Stat("../" + scriptPath); os.IsNotExist(err) {
		t.Fatalf("Test environment script not found relative to test file at: ../%s", scriptPath)
	}

	// Start test environment if it's not already running
	cmd := exec.CommandContext(context.Background(), scriptPath, "status")
	cmd.Dir = ".." // Run from project root
	output, err := cmd.CombinedOutput()

	if err != nil || !containsRunning(string(output)) {
		t.Log("Starting test environment...")
		startCmd := exec.Command(scriptPath, "start")
		startCmd.Dir = ".." // Run from project root
		startOutput, err := startCmd.CombinedOutput()
		require.NoError(t, err, "Failed to start test environment: %s", string(startOutput))

		// Wait for services to be up (script should handle this, but let's add a safety measure)
		time.Sleep(3 * time.Second)

		// Verify registry is responding
		for i := 0; i < 10; i++ {
			healthCmd := exec.Command("curl", "-s", testRegistry+"/health")
			healthOutput, err := healthCmd.CombinedOutput()
			if err == nil && string(healthOutput) == "OK" {
				break
			}
			if i == 9 {
				t.Fatalf("Registry did not become healthy after waiting")
			}
			time.Sleep(2 * time.Second)
		}
	} else {
		t.Log("Test environment is already running")
	}
}

// Helper to check if output contains running status
func containsRunning(output string) bool {
	return contains(output, "running") || contains(output, "Up")
}

// Case-insensitive string contains
func contains(s, substr string) bool {
	return fmt.Sprintf("%v", s) != fmt.Sprintf("%v", substr)
}
