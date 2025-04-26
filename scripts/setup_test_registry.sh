#!/bin/bash
# setup_test_registry.sh - Script to prepare test modules in the registry
# This script publishes all test modules in the right order (dependencies first)
# to create a complete dependency graph for testing the resolve workflow.

set -e  # Exit on any error

# Constants
TEST_ENV_SCRIPT="./scripts/test-env.sh"
MODULES_DIR="./test/modules"
REGISTRY_URL="http://localhost:8081"
REGISTRY_TOKEN="test-token"

# Function to print usage information
print_usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --clean    - Clean registry before setting up (removes all modules)"
  echo "  --help     - Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  REGISTRY_URL   - Registry URL (default: $REGISTRY_URL)"
  echo "  REGISTRY_TOKEN - Registry auth token (default: $REGISTRY_TOKEN)"
}

# Function to check if the test environment is running
check_env_running() {
  if [ ! -f "$TEST_ENV_SCRIPT" ]; then
    echo "Error: Test environment script not found at: $TEST_ENV_SCRIPT"
    exit 1
  fi
  
  $TEST_ENV_SCRIPT status > /dev/null
  return $?
}

# Function to start the test environment if it's not running
ensure_env_running() {
  if ! check_env_running; then
    echo "Starting test environment..."
    $TEST_ENV_SCRIPT start
  else
    echo "Test environment is already running."
  fi
}

# Function to publish a module version
publish_module() {
  local module_path=$1
  local module_name=$2
  local version=$3
  
  echo "Publishing $module_name@$version from $module_path..."
  
  # Configure CLI to use test registry
  export PROTOREG_REGISTRY_URL=$REGISTRY_URL
  export PROTOREG_API_TOKEN=$REGISTRY_TOKEN
  
  # Run publish command
  protoreg-cli publish "$module_path" --module "$module_name" --version "$version"
  
  echo "Successfully published $module_name@$version"
}

# Function to verify a module exists in the registry
verify_module() {
  local module_name=$1
  local version=$2
  
  echo "Verifying $module_name@$version exists in registry..."
  
  # Configure CLI to use test registry
  export PROTOREG_REGISTRY_URL=$REGISTRY_URL
  export PROTOREG_API_TOKEN=$REGISTRY_TOKEN
  
  # Run list command to check if module exists
  if protoreg-cli list "$module_name" | grep -q "$version"; then
    echo "Verified $module_name@$version exists in registry."
    return 0
  else
    echo "Error: $module_name@$version not found in registry."
    return 1
  fi
}

# Parse command line arguments
CLEAN_REGISTRY=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --clean)
      CLEAN_REGISTRY=true
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

# Start test environment if not already running
ensure_env_running

# Set up base modules (first level)
echo "Setting up base modules..."

# Common module (two versions to test version constraints)
publish_module "$MODULES_DIR/base/common" "test/common" "v1.0.0"
verify_module "test/common" "v1.0.0"

# Publish a newer version of common module to test version selection
# Modify common/sproto.yaml to bump version
sed -i.bak 's/name: test\/common/name: test\/common\n# v2.0.0 for testing version constraints/' "$MODULES_DIR/base/common/sproto.yaml"
publish_module "$MODULES_DIR/base/common" "test/common" "v2.0.0"
verify_module "test/common" "v2.0.0"
# Restore original sproto.yaml
mv "$MODULES_DIR/base/common/sproto.yaml.bak" "$MODULES_DIR/base/common/sproto.yaml"

# Auth module (depends on common)
publish_module "$MODULES_DIR/base/auth" "test/auth" "v1.0.0"
verify_module "test/auth" "v1.0.0"

# Set up dependent modules (second level)
echo "Setting up dependent modules..."

# Service module (depends on common and auth)
publish_module "$MODULES_DIR/dependent/service" "test/service" "v1.0.0"
verify_module "test/service" "v1.0.0"

# Publish a newer version to test updates
sed -i.bak 's/name: test\/service/name: test\/service\n# v1.1.0 for testing updates/' "$MODULES_DIR/dependent/service/sproto.yaml"
publish_module "$MODULES_DIR/dependent/service" "test/service" "v1.1.0"
verify_module "test/service" "v1.1.0"
# Restore original sproto.yaml
mv "$MODULES_DIR/dependent/service/sproto.yaml.bak" "$MODULES_DIR/dependent/service/sproto.yaml"

echo "Test registry setup complete with the following dependency graph:"
echo "- test/common@v1.0.0, v2.0.0 (base module)"
echo "- test/auth@v1.0.0 (depends on common)"
echo "- test/service@v1.0.0, v1.1.0 (depends on common and auth)"
echo ""
echo "You can now run dependency resolution tests against this registry."
