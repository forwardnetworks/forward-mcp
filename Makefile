# Project configuration
PROJECT_NAME=forward-mcp
BINARY_NAME=forward-mcp-server
TEST_CLIENT=forward-mcp-test-client
BUILD_DIR=bin
MAIN_FILE=cmd/server/main.go
TEST_CLIENT_FILE=cmd/test-client/main.go

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

# Go build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all build build-test-client test test-integration test-coverage clean run run-test-client dev deps

all: test build

# Build the main server
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)

# Build the test client
build-test-client:
	@echo "Building $(TEST_CLIENT)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(TEST_CLIENT) $(TEST_CLIENT_FILE)

# Run unit tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v ./... -tags=integration

# Run test coverage
test-coverage:
	@echo "Running test coverage..."
	$(GOTEST) -v ./... -coverprofile=coverage.out
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GOCLEAN)

# Run the server
run: build
	@echo "Starting MCP server..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run the test client
run-test-client: build build-test-client
	@echo "Starting MCP test client..."
	./$(BUILD_DIR)/$(TEST_CLIENT)

# Development server
dev:
	@echo "Starting development server..."
	$(GOCMD) run $(MAIN_FILE)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_linux $(MAIN_FILE)

# Build for Windows
build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME).exe $(MAIN_FILE)

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(PROJECT_NAME) .

# Docker run
docker-run: docker-build
	@echo "Running Docker container..."
	docker run --env-file .env -p 8080:8080 $(PROJECT_NAME)

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build the MCP server"
	@echo "  build-test-client  - Build the test client"
	@echo "  test               - Run unit tests"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  run                - Build and run the server"
	@echo "  run-test-client    - Build and run the test client"
	@echo "  dev                - Run in development mode"
	@echo "  clean              - Clean build artifacts"
	@echo "  deps               - Install dependencies"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run in Docker"
	@echo "  help               - Show this help" 