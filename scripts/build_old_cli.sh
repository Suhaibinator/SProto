#!/bin/bash
# build_old_cli.sh - Script to build an old version of the CLI (pre-dependency features)
# This script checks out an old commit, builds the CLI, and restores the repo state

set -e  # Exit on any error

# Constants
OLD_CLI_DIR="./test/compat/old_cli"
OLD_CLI_BIN="$OLD_CLI_DIR/protoreg-cli"
CURRENT_BRANCH=$(git branch --show-current)
TEMP_BRANCH="temp-old-cli-build"

# Last commit before dependency management features were added
# Replace this with the actual commit hash from your repository history
# This should be the last stable version before dependency features were added
OLD_VERSION_COMMIT="abc123"  # REPLACE THIS with your actual commit hash!

# Function to print usage information
print_usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --commit HASH   - Specify a different commit hash to build from (default: $OLD_VERSION_COMMIT)"
  echo "  --help          - Show this help message"
  echo ""
  echo "This script builds an old version of the CLI (before dependency features)"
  echo "from a specified commit and places it in $OLD_CLI_DIR for compatibility testing."
}

# Parse command line arguments
COMMIT=$OLD_VERSION_COMMIT
while [[ $# -gt 0 ]]; do
  case "$1" in
    --commit)
      COMMIT="$2"
      shift 2
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

# Create directories
mkdir -p "$OLD_CLI_DIR"

# Function to clean up on exit
cleanup() {
  echo "Cleaning up..."
  # Get back to original branch
  git checkout "$CURRENT_BRANCH" &>/dev/null
  # Delete temporary branch if it exists
  if git show-ref --verify --quiet "refs/heads/$TEMP_BRANCH"; then
    git branch -D "$TEMP_BRANCH" &>/dev/null
  fi
  echo "Cleanup completed, repository state restored."
}

# Set trap to ensure cleanup on exit
trap cleanup EXIT

echo "Building old CLI version from commit $COMMIT..."

# Create a temporary branch at the old commit
git checkout -b "$TEMP_BRANCH" "$COMMIT"

# Build the old CLI version
echo "Compiling old CLI version..."
go build -o "$OLD_CLI_BIN" ./cmd/cli

# Verify the build was successful
if [ -f "$OLD_CLI_BIN" ]; then
  echo "Successfully built old CLI at $OLD_CLI_BIN"
  echo "Version information:"
  "$OLD_CLI_BIN" --version || echo "Version command not available in this CLI version"
else
  echo "Failed to build old CLI"
  exit 1
fi

# Cleanup happens via the trap
