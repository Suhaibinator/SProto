#!/bin/bash
# setup_old_db.sh - Script to set up a database with the old schema and populate it with sample data
# This script helps test the database migration process from pre-dependency to post-dependency schema

set -e  # Exit on any error

# Constants
TEST_DB_CONTAINER="sproto_postgres_test"
DB_NAME="sproto_test"
DB_USER="postgres"
DB_PASSWORD="postgres_test"
OLD_SCHEMA_FILE="./sql/schema.sql"  # Original schema without dependency management
MIGRATION_FILE="./sql/002_add_dependency_management.sql"  # Migration to add dependency schema

# Function to print usage information
print_usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --with-sample-data  - Add sample data to the old schema database (default behavior)"
  echo "  --schema-only       - Create the old schema without adding sample data"
  echo "  --run-migration     - Apply the migration to add dependency management schema"
  echo "  --help              - Show this help message"
  echo ""
  echo "This script creates a database with the old schema (before dependency management)"
  echo "for testing migration and backward compatibility."
}

# Parse command line arguments
WITH_SAMPLE_DATA=true
RUN_MIGRATION=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-sample-data)
      WITH_SAMPLE_DATA=true
      shift
      ;;
    --schema-only)
      WITH_SAMPLE_DATA=false
      shift
      ;;
    --run-migration)
      RUN_MIGRATION=true
      shift
      ;;
    --help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      print_usage
      exit 1
      ;;
  esac
done

# Check if test environment is running
docker exec $TEST_DB_CONTAINER psql -U $DB_USER -c "SELECT 1" >/dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "Error: Test database container ($TEST_DB_CONTAINER) is not running."
  echo "Please start the test environment first with: ./scripts/test-env.sh start"
  exit 1
fi

echo "Setting up database with old schema..."

# Drop existing database if it exists
docker exec $TEST_DB_CONTAINER psql -U $DB_USER -c "DROP DATABASE IF EXISTS $DB_NAME WITH (FORCE);" postgres

# Create fresh database
docker exec $TEST_DB_CONTAINER psql -U $DB_USER -c "CREATE DATABASE $DB_NAME;" postgres

# Apply old schema (before dependency management)
if [ -f "$OLD_SCHEMA_FILE" ]; then
  echo "Applying old schema from $OLD_SCHEMA_FILE"
  cat "$OLD_SCHEMA_FILE" | docker exec -i $TEST_DB_CONTAINER psql -U $DB_USER -d $DB_NAME
else
  echo "Error: Old schema file not found at $OLD_SCHEMA_FILE"
  exit 1
fi

# Add sample data if requested
if [ "$WITH_SAMPLE_DATA" = true ]; then
  echo "Adding sample data to the database..."
  # Insert sample modules
  docker exec $TEST_DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "
    INSERT INTO modules (namespace, name, created_at) VALUES 
      ('oldco', 'common', CURRENT_TIMESTAMP),
      ('oldco', 'auth', CURRENT_TIMESTAMP),
      ('oldco', 'api', CURRENT_TIMESTAMP);
  "
  
  # Insert sample versions
  docker exec $TEST_DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "
    INSERT INTO module_versions (module_id, version, digest, storage_key, created_at) VALUES 
      ((SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'common'), 'v0.1.0', 'sha256:aaa111', 'oldco/common/v0.1.0.zip', CURRENT_TIMESTAMP),
      ((SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'common'), 'v0.2.0', 'sha256:aaa222', 'oldco/common/v0.2.0.zip', CURRENT_TIMESTAMP),
      ((SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'auth'), 'v0.1.0', 'sha256:bbb111', 'oldco/auth/v0.1.0.zip', CURRENT_TIMESTAMP),
      ((SELECT id FROM modules WHERE namespace = 'oldco' AND name = 'api'), 'v0.1.0', 'sha256:ccc111', 'oldco/api/v0.1.0.zip', CURRENT_TIMESTAMP);
  "
  
  echo "Sample data added successfully."
fi

# Run migration if requested
if [ "$RUN_MIGRATION" = true ]; then
  echo "Running migration to add dependency management schema..."
  if [ -f "$MIGRATION_FILE" ]; then
    cat "$MIGRATION_FILE" | docker exec -i $TEST_DB_CONTAINER psql -U $DB_USER -d $DB_NAME
    echo "Migration completed."
  else
    echo "Error: Migration file not found at $MIGRATION_FILE"
    exit 1
  fi
fi

echo "Database setup completed successfully."
echo "To connect to this database for manual inspection:"
echo "docker exec -it $TEST_DB_CONTAINER psql -U $DB_USER -d $DB_NAME"
