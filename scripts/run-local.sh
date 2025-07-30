#!/bin/bash
# Local runner script for SAM.gov Monitor
# This script sets up the environment and runs the monitor with enhanced logging

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}SAM.gov Monitor - Local Runner${NC}"
echo "================================="

# Get the script directory and project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Check if .env file exists in project root
if [ -f "$PROJECT_ROOT/.env" ]; then
    echo -e "${YELLOW}Loading environment from $PROJECT_ROOT/.env file...${NC}"
    export $(cat "$PROJECT_ROOT/.env" | grep -v '^#' | xargs)
else
    echo -e "${YELLOW}No .env file found in project root. Using existing environment variables.${NC}"
fi

# Verify required environment variables
if [ -z "$SAM_API_KEY" ]; then
    echo -e "${RED}ERROR: SAM_API_KEY is not set${NC}"
    echo "Please set SAM_API_KEY environment variable or create a .env file"
    exit 1
fi

# Change to project root for all operations
cd "$PROJECT_ROOT"

# Build the binary if it doesn't exist or if source is newer
if [ ! -f ./bin/monitor ] || [ cmd/monitor/main.go -nt ./bin/monitor ]; then
    echo -e "${YELLOW}Building monitor binary...${NC}"
    make build
fi

# Set additional environment variables for local execution
export SAM_USER_AGENT="SAM.gov-Monitor-Local/1.0 (github.com/trustdan/SAM.gov-API-notifier)"
export SAM_RATE_LIMIT_DELAY="0s"  # No delay - we have limited requests
export SAM_MAX_RETRIES="0"        # NO RETRIES - preserve our 10 daily requests!

# Run with verbose logging
echo -e "${GREEN}Starting monitor with enhanced logging...${NC}"
echo "User-Agent: $SAM_USER_AGENT"
echo "Rate limit delay: $SAM_RATE_LIMIT_DELAY"
echo "Max retries: $SAM_MAX_RETRIES"
echo "================================="

# Run the monitor with debug logging
DEBUG=true ./bin/monitor -config config/queries.yaml "$@"