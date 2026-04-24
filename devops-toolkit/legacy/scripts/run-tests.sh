#!/bin/bash
# Comprehensive test runner for DevOps Toolkit

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Options
VERBOSE=""
COVERAGE=""
WATCH=""
TEST_PATH=""
ONLY_UNIT=""
ONLY_INTEGRATION=""

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -v|--verbose)
      VERBOSE="-v"
      shift
      ;;
    --coverage)
      COVERAGE="--coverage"
      shift
      ;;
    --watch)
      WATCH="--watch"
      shift
      ;;
    --test=*)
      TEST_PATH="${1#*=}"
      shift
      ;;
    --unit)
      ONLY_UNIT="--testPathPattern=unit|tests/"
      shift
      ;;
    --integration)
      ONLY_INTEGRATION="--testPathPattern=integration/"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  -v, --verbose         Verbose output"
      echo "  --coverage            Generate coverage report"
      echo "  --watch               Watch mode"
      echo "  --test=PATH           Run specific test file"
      echo "  --unit                Run only unit tests"
      echo "  --integration         Run only integration tests"
      echo "  -h, --help            Show this help"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  DevOps Toolkit - Test Runner${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""

# Check if npm is available
if ! command -v npm &> /dev/null; then
    echo -e "${RED}Error: npm not found${NC}"
    exit 1
fi

# Install dependencies if needed
if [ ! -d "devops-toolkit/node_modules" ]; then
    echo -e "${YELLOW}Installing dependencies...${NC}"
    cd devops-toolkit
    npm install
    cd ..
fi

# Determine which tests to run
TEST_CMD="npm test --prefix devops-toolkit"

if [ -n "$TEST_PATH" ]; then
    TEST_CMD="$TEST_CMD --testPathPattern=$TEST_PATH"
fi

if [ -n "$ONLY_UNIT" ]; then
    TEST_CMD="$TEST_CMD $ONLY_UNIT"
fi

if [ -n "$ONLY_INTEGRATION" ]; then
    TEST_CMD="$TEST_CMD $ONLY_INTEGRATION"
fi

if [ -n "$VERBOSE" ]; then
    TEST_CMD="$TEST_CMD $VERBOSE"
fi

if [ -n "$COVERAGE" ]; then
    TEST_CMD="$TEST_CMD --coverage --coverageDirectory=coverage"
fi

if [ -n "$WATCH" ]; then
    TEST_CMD="$TEST_CMD --watch"
fi

# Run tests
echo -e "${YELLOW}Running tests...${NC}"
echo ""

$TEST_CMD

EXIT_CODE=$?

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
else
    echo -e "${RED}✗ Tests failed!${NC}"
fi

exit $EXIT_CODE
