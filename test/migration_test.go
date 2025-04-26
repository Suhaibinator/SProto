package test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseMigration tests the migration from older schema to the one with dependency management.
func TestDatabaseMigration(t *testing.T) {
	// Skip this test if ENV var SKIP_MIGRATION_TESTS is set
	if os.Getenv("SKIP_MIGRATION_TESTS") != "" {
		t.Skip("Skipping migration tests due to SKIP_MIGRATION_TESTS env var")
	}

	// Make sure the test environment is running
	ensureTestEnvRunning(t) // Reuse from end_to_end_test.go

	// Setup the old database schema with sample data
	t.Log("Setting up database with old schema and sample data...")
	setupCmd := exec.Command("./scripts/setup_old_db.sh", "--with-sample-data")
	output, err := setupCmd.CombinedOutput()
	require.NoError(t, err, "Failed to set up old database: %s", string(output))

	// Verify the old database has the expected data
	// We'll use the Docker exec command to run SQL queries in the database
	checkOldDataCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM modules WHERE namespace = 'oldco'")
	output, err = checkOldDataCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check old data: %s", string(output))
	assert.Contains(t, string(output), "3", "Expected 3 modules in old data")

	checkOldVersionsCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM module_versions")
	output, err = checkOldVersionsCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check old versions: %s", string(output))
	assert.Contains(t, string(output), "4", "Expected 4 module versions in old data")

	// Run the migration
	t.Log("Running migration to add dependency management schema...")
	migrationCmd := exec.Command("./scripts/setup_old_db.sh", "--run-migration")
	output, err = migrationCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run migration: %s", string(output))

	// Verify the old data is still intact after migration
	checkPostMigrationDataCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM modules WHERE namespace = 'oldco'")
	output, err = checkPostMigrationDataCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check post-migration data: %s", string(output))
	assert.Contains(t, string(output), "3", "Expected 3 modules in post-migration data")

	// Verify that new tables exist and are empty (no dependencies stored yet)
	checkDependencyTableCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM module_dependencies")
	output, err = checkDependencyTableCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check dependency table: %s", string(output))
	// The table should exist but be empty
	assert.Contains(t, string(output), "0", "Expected 0 dependencies initially")

	checkImportPathTableCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM module_import_paths")
	output, err = checkImportPathTableCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check import path table: %s", string(output))
	// The table should exist but be empty
	assert.Contains(t, string(output), "0", "Expected 0 import paths initially")

	// Test that we can add data to the new tables
	t.Log("Testing adding dependency data to migrated schema...")
	addDependencyCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-c",
		"INSERT INTO module_dependencies (module_version_id, dependency_module_id, version_constraint) VALUES "+
			"((SELECT id FROM module_versions WHERE version = 'v0.1.0' AND module_id = "+
			"(SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'api')), "+
			"(SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'common'), 'v0.1.0')")
	output, err = addDependencyCmd.CombinedOutput()
	require.NoError(t, err, "Failed to add dependency: %s", string(output))

	// Verify the dependency was added
	checkAddedDependencyCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM module_dependencies")
	output, err = checkAddedDependencyCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check added dependency: %s", string(output))
	assert.Contains(t, string(output), "1", "Expected 1 dependency after adding")

	// Test that we can also add import path data
	addImportPathCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-c",
		"INSERT INTO module_import_paths (module_id, import_path) VALUES "+
			"((SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'common'), "+
			"'github.com/oldco/common/proto')")
	output, err = addImportPathCmd.CombinedOutput()
	require.NoError(t, err, "Failed to add import path: %s", string(output))

	// Verify the import path was added
	checkAddedImportPathCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT COUNT(*) FROM module_import_paths")
	output, err = checkAddedImportPathCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check added import path: %s", string(output))
	assert.Contains(t, string(output), "1", "Expected 1 import path after adding")

	// Verify that we can query dependency information along with module information
	checkJoinQueryCmd := exec.Command("docker", "exec", "sproto_postgres_test", "psql",
		"-U", "postgres", "-d", "sproto_test", "-t", "-c",
		"SELECT m.namespace, m.name, mv.version, md.version_constraint FROM modules m "+
			"JOIN module_versions mv ON m.id = mv.module_id "+
			"JOIN module_dependencies md ON mv.id = md.module_version_id "+
			"JOIN modules dm ON md.dependency_module_id = dm.id "+
			"WHERE dm.namespace = 'oldco' AND dm.name = 'common'")
	output, err = checkJoinQueryCmd.CombinedOutput()
	require.NoError(t, err, "Failed to check join query: %s", string(output))
	assert.Contains(t, string(output), "oldco", "Join query should return results")
	assert.Contains(t, string(output), "api", "Join query should include api module")
	assert.Contains(t, string(output), "v0.1.0", "Join query should include version")

	t.Log("Database migration test completed successfully.")
}
