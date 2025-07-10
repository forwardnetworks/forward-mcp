#!/bin/bash

echo "Testing Forward MCP Server Semantic Search..."

# Start the server in background
./bin/forward-mcp-server &
SERVER_PID=$!

# Wait for server to start
sleep 3

echo "1. Testing query index stats..."
echo '{"method": "tools/call", "params": {"name": "get_query_index_stats", "arguments": {"detailed": true}}}' | nc localhost 8080

echo -e "\n2. Testing semantic search for security queries..."
echo '{"method": "tools/call", "params": {"name": "search_nqe_queries", "arguments": {"query": "security vulnerabilities", "limit": 5}}}' | nc localhost 8080

echo -e "\n3. Testing semantic search for BGP routing..."
echo '{"method": "tools/call", "params": {"name": "search_nqe_queries", "arguments": {"query": "BGP routing problems", "limit": 3}}}' | nc localhost 8080

# Clean up
kill $SERVER_PID
echo -e "\nTest completed!" 