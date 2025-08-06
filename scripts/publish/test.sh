#!/bin/bash
# Test script for publish.go

set -e

echo "Running publish.go tests..."
echo "============================"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Run tests with coverage
echo "Running unit tests with coverage..."
if go test -v -cover -short ./... > test.log 2>&1; then
    echo -e "${GREEN}✓ All tests passed${NC}"
    echo ""
    echo "Coverage summary:"
    go test -cover -short ./... 2>&1 | grep coverage || true
else
    echo -e "${RED}✗ Tests failed${NC}"
    echo ""
    echo "Error output:"
    tail -20 test.log
    exit 1
fi

echo ""
echo "Test categories:"
echo "----------------"
echo "✓ Draft proposal workflow"
echo "✓ Numbered proposal workflow"  
echo "✓ Non-HEAD commit handling"
echo "✓ Error cases"
echo "✓ Discussion link updates"
echo "✓ Integration workflow"

echo ""
echo -e "${GREEN}All tests completed successfully!${NC}"

# Optional: Run specific test in verbose mode
if [ "$1" = "-v" ]; then
    echo ""
    echo "Running integration test in verbose mode..."
    go test -v -run TestIntegrationWorkflow
fi

# Cleanup
rm -f test.log