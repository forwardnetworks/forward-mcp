#!/bin/bash

# Forward Networks MCP Test Runner
# Usage: ./scripts/test.sh [unit|integration|all|bench] [options]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
TEST_TYPE="unit"
VERBOSE=""
COVERAGE=""

# Function to print usage
usage() {
    echo "Usage: $0 [unit|integration|all|bench] [options]"
    echo ""
    echo "Test types:"
    echo "  unit         Run unit tests with mock client (default)"
    echo "  integration  Run integration tests with real API (requires .env)"
    echo "  all          Run both unit and integration tests"
    echo "  bench        Run benchmark tests"
    echo ""
    echo "Options:"
    echo "  -v, --verbose    Verbose output"
    echo "  -c, --coverage   Generate coverage report"
    echo "  -h, --help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 unit -v                     # Run unit tests with verbose output"
    echo "  $0 integration                 # Run integration tests"
    echo "  $0 all -c                      # Run all tests with coverage"
    echo "  $0 bench                       # Run benchmarks"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        unit|integration|all|bench)
            TEST_TYPE="$1"
            shift
            ;;
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -c|--coverage)
            COVERAGE="-coverprofile=coverage.out -covermode=atomic"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

# Function to run tests
run_tests() {
    local test_pattern="$1"
    local description="$2"
    
    echo -e "${BLUE}Running $description...${NC}"
    
    if [[ "$TEST_TYPE" == "bench" ]]; then
        go test ./internal/service/ -bench=. -run=NONE $VERBOSE
    else
        if go test ./internal/service/ -run="$test_pattern" $VERBOSE $COVERAGE; then
            echo -e "${GREEN}✓ $description passed${NC}"
        else
            echo -e "${RED}✗ $description failed${NC}"
            exit 1
        fi
    fi
    echo ""
}

# Main execution
echo -e "${YELLOW}Forward Networks MCP Test Suite${NC}"
echo "========================================"
echo ""

case $TEST_TYPE in
    "unit")
        run_tests "^Test[^I]" "Unit Tests (Mock Client)"
        ;;
    "integration")
        echo -e "${YELLOW}Note: Integration tests require .env file with valid API credentials${NC}"
        echo ""
        run_tests "^TestIntegration" "Integration Tests (Real API)"
        ;;
    "all")
        run_tests "^Test[^I]" "Unit Tests (Mock Client)"
        echo -e "${YELLOW}Note: Integration tests require .env file with valid API credentials${NC}"
        echo ""
        run_tests "^TestIntegration" "Integration Tests (Real API)"
        ;;
    "bench")
        run_tests "" "Benchmark Tests"
        ;;
esac

# Generate coverage report if requested
if [[ -n "$COVERAGE" && "$TEST_TYPE" != "bench" ]]; then
    if [[ -f "coverage.out" ]]; then
        echo -e "${BLUE}Generating coverage report...${NC}"
        go tool cover -html=coverage.out -o coverage.html
        echo -e "${GREEN}✓ Coverage report generated: coverage.html${NC}"
        
        # Show coverage summary
        echo ""
        echo -e "${BLUE}Coverage Summary:${NC}"
        go tool cover -func=coverage.out | tail -1
    fi
fi

echo -e "${GREEN}All tests completed successfully!${NC}" 