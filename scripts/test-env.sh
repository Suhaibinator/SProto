#!/bin/bash
# test-env.sh - Helper script to manage the SProto test environment

set -e  # Exit on any error

# Constants
DOCKER_COMPOSE_FILE="docker-compose.test.yaml"
REGISTRY_URL="http://localhost:8081"  # The port we exposed in the test compose file
REGISTRY_TOKEN="test-token"           # Matches the token in the test compose file

# Default wait times
MAX_WAIT_TIME=60  # Maximum time to wait for services to be ready, in seconds
WAIT_INTERVAL=5   # Time between health checks, in seconds

# Function to print usage information
print_usage() {
  echo "Usage: $0 [COMMAND]"
  echo ""
  echo "Commands:"
  echo "  start    - Start the test environment"
  echo "  stop     - Stop the test environment"
  echo "  status   - Check the status of the test environment"
  echo "  restart  - Restart the test environment"
  echo "  clean    - Stop the test environment and remove volumes"
  echo "  help     - Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  WAIT_TIME       - Maximum time to wait for services (default: $MAX_WAIT_TIME seconds)"
  echo "  WAIT_INTERVAL   - Time between health checks (default: $WAIT_INTERVAL seconds)"
}

# Function to start the test environment
start_env() {
  echo "Starting SProto test environment..."
  
  # Check if docker-compose.test.yaml exists
  if [ ! -f "$DOCKER_COMPOSE_FILE" ]; then
    echo "Error: $DOCKER_COMPOSE_FILE not found!"
    echo "Make sure you're running this script from the project root directory."
    exit 1
  fi
  
  # Start the services in detached mode
  docker-compose -f "$DOCKER_COMPOSE_FILE" up -d
  
  # Wait for services to be ready
  wait_for_services
  
  echo ""
  echo "SProto test environment is ready!"
  echo "Registry API URL: $REGISTRY_URL"
  echo "Registry Auth Token: $REGISTRY_TOKEN"
  echo ""
  echo "To use with CLI:"
  echo "  export PROTOREG_REGISTRY_URL=$REGISTRY_URL"
  echo "  export PROTOREG_API_TOKEN=$REGISTRY_TOKEN"
  echo ""
}

# Function to wait for all services to be ready
wait_for_services() {
  echo "Waiting for services to be ready..."
  
  local wait_time=${WAIT_TIME:-$MAX_WAIT_TIME}
  local interval=${WAIT_INTERVAL:-$WAIT_INTERVAL}
  local elapsed=0
  
  while [ $elapsed -lt $wait_time ]; do
    # Check PostgreSQL
    if docker-compose -f "$DOCKER_COMPOSE_FILE" exec postgres-test pg_isready -U postgres &>/dev/null; then
      echo "✓ PostgreSQL is ready"
      pg_ready=true
    else
      echo "○ Waiting for PostgreSQL..."
      pg_ready=false
    fi
    
    # Check MinIO
    if docker-compose -f "$DOCKER_COMPOSE_FILE" exec minio-test curl -s http://localhost:9000/minio/health/live &>/dev/null; then
      echo "✓ MinIO is ready"
      minio_ready=true
    else
      echo "○ Waiting for MinIO..."
      minio_ready=false
    fi
    
    # Check Registry API
    if curl -s "$REGISTRY_URL/health" &>/dev/null; then
      echo "✓ Registry API is ready"
      api_ready=true
    else
      echo "○ Waiting for Registry API..."
      api_ready=false
    fi
    
    # If all services are ready, break the loop
    if [ "$pg_ready" = true ] && [ "$minio_ready" = true ] && [ "$api_ready" = true ]; then
      return 0
    fi
    
    # Sleep and increment elapsed time
    sleep $interval
    elapsed=$((elapsed + interval))
    echo "Still waiting... ($elapsed/$wait_time seconds elapsed)"
  done
  
  echo "Error: Timed out waiting for services to be ready"
  show_logs
  exit 1
}

# Function to show logs for debugging
show_logs() {
  echo "Showing recent logs for troubleshooting:"
  docker-compose -f "$DOCKER_COMPOSE_FILE" logs --tail=50
}

# Function to stop the test environment
stop_env() {
  echo "Stopping SProto test environment..."
  docker-compose -f "$DOCKER_COMPOSE_FILE" down
  echo "SProto test environment stopped"
}

# Function to clean up the test environment (including volumes)
clean_env() {
  echo "Stopping SProto test environment and removing volumes..."
  docker-compose -f "$DOCKER_COMPOSE_FILE" down -v
  echo "SProto test environment cleaned up"
}

# Function to check status of the test environment
check_status() {
  echo "Checking SProto test environment status..."
  docker-compose -f "$DOCKER_COMPOSE_FILE" ps
  
  # Try to ping the registry API
  if curl -s "$REGISTRY_URL/health" &>/dev/null; then
    echo "Registry API is responding at $REGISTRY_URL"
  else
    echo "Registry API is not responding at $REGISTRY_URL"
  fi
}

# Main script logic
case "${1:-help}" in
  start)
    start_env
    ;;
  stop)
    stop_env
    ;;
  restart)
    stop_env
    start_env
    ;;
  clean)
    clean_env
    ;;
  status)
    check_status
    ;;
  help|--help|-h)
    print_usage
    ;;
  *)
    echo "Unknown command: ${1}"
    print_usage
    exit 1
    ;;
esac
