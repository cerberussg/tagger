#!/bin/bash

# Test runner script for tagger project

set -e  # Exit on any error

echo "🎵 Tagger Test Suite 🎵"
echo "======================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_status "Go version: $(go version)"

# Ensure we're in the right directory (has go.mod)
if [ ! -f "go.mod" ]; then
    print_error "go.mod not found. Please run this script from the project root."
    exit 1
fi

# Download dependencies
print_status "Downloading dependencies..."
go mod download
go mod tidy

# Run unit tests
print_status "Running unit tests..."
echo "========================"

# Run tests with coverage
if go test -v -race -coverprofile=coverage.out ./pkg/enricher/...; then
    print_success "Unit tests passed!"
    
    # Show coverage report
    print_status "Coverage report:"
    go tool cover -func=coverage.out | tail -1
    
    # Generate HTML coverage report
    go tool cover -html=coverage.out -o coverage.html
    print_status "HTML coverage report generated: coverage.html"
else
    print_error "Unit tests failed!"
    exit 1
fi

echo ""

# Ask if user wants to run integration tests
read -p "Do you want to run integration tests against real MusicBrainz API? (y/N): " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_warning "Running integration tests - this will make real API calls to MusicBrainz"
    print_warning "Please be respectful of their rate limits (1 request per second)"
    echo ""
    
    if go test -v -tags=integration ./pkg/enricher/musicbrainz/...; then
        print_success "Integration tests passed!"
    else
        print_error "Integration tests failed!"
        exit 1
    fi
else
    print_status "Skipping integration tests"
fi

echo ""

# Run benchmarks (optional)
read -p "Do you want to run benchmarks? (y/N): " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_status "Running benchmarks..."
    go test -bench=. -benchmem ./pkg/enricher/...
else
    print_status "Skipping benchmarks"
fi

echo ""

# Run linting if golangci-lint is available
if command -v golangci-lint &> /dev/null; then
    print_status "Running linter..."
    if golangci-lint run; then
        print_success "Linting passed!"
    else
        print_warning "Linting found issues (but tests still passed)"
    fi
else
    print_warning "golangci-lint not found - skipping linting"
    print_status "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
fi

echo ""
print_success "Test suite completed! 🎉"

# Clean up
rm -f coverage.out

# Summary
echo ""
echo "📊 Test Summary:"
echo "- Unit tests: ✅ Passed"
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "- Integration tests: ✅ Passed"
else
    echo "- Integration tests: ⏭️  Skipped"
fi
echo "- Coverage report: coverage.html"
echo ""
echo "Next steps:"
echo "1. Review coverage.html to see test coverage"
echo "2. If integration tests passed, your MusicBrainz provider is working!"
echo "3. Ready to integrate into your batch command"