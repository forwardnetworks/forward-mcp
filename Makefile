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

.PHONY: all build build-test-client test test-integration test-coverage clean run run-test-client dev deps embedding-status embedding-generate-keyword embedding-generate-openai embedding-cache-info embedding-benchmark embedding-clean demo-smart-search test-path-search-integration test-path-search-mcp lint

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

# ==========================================
# EMBEDDING MANAGEMENT TARGETS
# ==========================================

# Check embedding status and coverage
embedding-status:
	@echo "🔍 Checking embedding status..."
	@$(GOCMD) run scripts/embedding-status/main.go

# Generate embeddings using keyword-based service (fast, free, offline)
embedding-generate-keyword:
	@echo "🚀 Generating keyword-based embeddings (fast, free, offline)..."
	@echo "⚡ This uses your optimized keyword embedding system from ACHIEVEMENTS.md"
	@FORWARD_EMBEDDING_PROVIDER=keyword $(GOCMD) run scripts/generate-embeddings/main.go

# Generate embeddings using OpenAI API (slow, costs money, better semantic quality)
embedding-generate-openai:
	@echo "🧠 Generating OpenAI-based embeddings (requires OPENAI_API_KEY)..."
	@if [ -z "$$OPENAI_API_KEY" ]; then \
		echo "❌ Error: OPENAI_API_KEY environment variable not set"; \
		echo "💡 Set it with: export OPENAI_API_KEY=your-key-here"; \
		echo "💡 Or use 'make embedding-generate-keyword' for free alternative"; \
		exit 1; \
	fi
	@echo "💰 Warning: This will make API calls to OpenAI and cost money"
	@echo "📊 Estimated cost: ~$$1-5 for 6000+ queries"
	@read -p "Continue? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@FORWARD_EMBEDDING_PROVIDER=openai $(GOCMD) run scripts/generate-embeddings/main.go

# Show embedding cache information
embedding-cache-info:
	@echo "📊 Embedding cache information:"
	@if [ -f "spec/nqe-embeddings.json" ]; then \
		echo "✅ Cache file exists: spec/nqe-embeddings.json"; \
		echo "📁 Cache size: $$(du -h spec/nqe-embeddings.json | cut -f1)"; \
		echo "🔢 Cache entries: $$(grep -o '\"/' spec/nqe-embeddings.json | wc -l | tr -d ' ')"; \
		echo "📅 Last modified: $$(stat -f "%Sm" spec/nqe-embeddings.json)"; \
	else \
		echo "❌ No embedding cache found"; \
		echo "💡 Run 'make embedding-generate-keyword' to create one"; \
	fi

# Benchmark embedding search performance
embedding-benchmark:
	@echo "⚡ Running embedding search benchmark..."
	@$(GOCMD) run scripts/benchmark-search/main.go

# Clear embedding cache (use with caution)
embedding-clean:
	@echo "🗑️  Clearing embedding cache..."
	@if [ -f "spec/nqe-embeddings.json" ]; then \
		read -p "⚠️  This will delete all cached embeddings. Continue? (y/N): " confirm && [ "$$confirm" = "y" ] && \
		rm -f spec/nqe-embeddings.json && \
		echo "✅ Embedding cache cleared" || \
		echo "❌ Operation cancelled"; \
	else \
		echo "ℹ️  No embedding cache to clear"; \
	fi

# Embedding Demo Targets
demo-smart-search: ## 🚀 Run smart query discovery demo (shows semantic search → executable mapping)
	@echo "🚀 Running smart query discovery demo..."
	@go run ./scripts/demo-smart-search

# Test path search integration with customer-specific IPs
test-path-search-integration: ## 🔍 Run path search integration tests with customer IPs (10.6.142.197 → 10.5.0.130)
	@echo "🔍 Running path search integration tests with customer IPs..."
	@echo "   Source IP: 10.6.142.197 → Destination IP: 10.5.0.130"
	@if [ ! -f .env ]; then \
		echo "❌ .env file not found. Please create it with FORWARD_API_KEY, FORWARD_API_SECRET, and FORWARD_API_BASE_URL"; \
		exit 1; \
	fi
	@go test -v ./internal/service -run "TestIntegrationPathSearch" -tags=integration

# Test path search using MCP client (interactive)
test-path-search-mcp: build build-test-client ## 🚀 Test path search using MCP test client (interactive mode)
	@echo "🚀 Starting MCP test client for path search testing..."
	@echo "💡 Available path search tests:"
	@echo "   5. Customer path search: 100.100.1.1 → 190.37.14.114 (basic)"
	@echo "   6. Customer path search with PREFER_DELIVERED intent"
	@echo "   7. Customer path search for HTTPS (TCP 443)"
	@echo "   8. Customer path search with network functions"
	@echo "   9. Customer reverse path: 190.37.14.114 → 100.100.1.1"
	@echo ""
	@echo "📝 Note: Using test network_id '162112'"
	@echo "💡 Troubleshooting: If 0 paths found, try options 1-3 first to verify connectivity"
	@echo ""
	@./bin/forward-mcp-test-client

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "🏗️  BUILD & RUN:"
	@echo "  build              - Build the MCP server"
	@echo "  build-test-client  - Build the test client"
	@echo "  run                - Build and run the server"
	@echo "  run-test-client    - Build and run the test client"
	@echo "  dev                - Run in development mode"
	@echo ""
	@echo "🧪 TESTING:"
	@echo "  test               - Run unit tests"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  test-path-search-mcp - Test path search using MCP client (interactive)"
	@echo ""
	@echo "🤖 EMBEDDINGS:"
	@echo "  embedding-status         - Check embedding coverage and stats"
	@echo "  embedding-generate-keyword - Generate fast, free keyword embeddings"
	@echo "  embedding-generate-openai  - Generate OpenAI embeddings (costs money)"
	@echo "  embedding-cache-info     - Show embedding cache information"
	@echo "  embedding-benchmark      - Test search performance"
	@echo "  embedding-clean          - Clear embedding cache"
	@echo "  demo-smart-search        - Run smart query discovery demo"
	@echo ""
	@echo "🛠️  UTILITIES:"
	@echo "  clean              - Clean build artifacts"
	@echo "  deps               - Install dependencies"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run in Docker"
	@echo "  help               - Show this help"

.PHONY: lint
lint:
	golangci-lint run 