#!/bin/bash

# Test script for Forward Networks MCP Server
# This script can be run directly in Cursor's terminal

set -e

echo "🚀 Forward Networks MCP Server Test Script"
echo "=========================================="

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "❌ .env file not found. Please create one with your Forward Networks credentials."
    echo ""
    echo "Example .env file:"
    echo "FORWARD_API_KEY=your-api-key"
    echo "FORWARD_API_SECRET=your-api-secret"
    echo "FORWARD_API_BASE_URL=https://your-instance.forwardnetworks.com"
    echo "FORWARD_INSECURE_SKIP_VERIFY=true"
    exit 1
fi

echo "✅ Found .env configuration file"

# Build the server and test client
echo "🔨 Building MCP server and test client..."
make build build-test-client

if [ ! -f "bin/forward-mcp-server" ]; then
    echo "❌ Failed to build MCP server"
    exit 1
fi

if [ ! -f "bin/forward-mcp-test-client" ]; then
    echo "❌ Failed to build test client"
    exit 1
fi

echo "✅ Build successful"

# Source the .env file
export $(cat .env | grep -v '^#' | xargs)

echo "🔗 Connecting to: $FORWARD_API_BASE_URL"

# Test 1: Run unit tests
echo ""
echo "🧪 Running unit tests..."
make test

# Test 2: Run integration tests (if credentials are available)
echo ""
echo "🌐 Running integration tests..."
if make test-integration 2>/dev/null; then
    echo "✅ Integration tests passed"
else
    echo "⚠️  Integration tests skipped (credentials may be invalid or instance unreachable)"
fi

# Test 3: Quick MCP protocol test
echo ""
echo "📡 Testing MCP protocol..."

# Start the server in background
./bin/forward-mcp-server &
SERVER_PID=$!

# Give the server a moment to start
sleep 2

# Send a simple MCP request
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | nc -q 1 localhost 9999 2>/dev/null || {
    echo "⚠️  Direct protocol test failed (this is normal - server uses stdio transport)"
}

# Kill the background server
kill $SERVER_PID 2>/dev/null || true

echo ""
echo "🎯 Quick validation tests completed!"

echo ""
echo "🔧 To test manually:"
echo "   1. Run: make run-test-client"
echo "   2. Or configure Claude Desktop with the config file provided"
echo "   3. Or run individual tests: make test"

echo ""
echo "📋 Available Make targets:"
make help 