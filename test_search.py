#!/usr/bin/env python3

import json
import subprocess
import sys
import time

def test_mcp_tool(tool_name, arguments):
    """Test a specific MCP tool"""
    print(f"\n🔍 Testing {tool_name}...")
    
    # Create the JSON-RPC request
    request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": arguments
        }
    }
    
    # Start the MCP server
    try:
        process = subprocess.Popen(
            ['./bin/forward-mcp-server'],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        
        # Send the request
        request_json = json.dumps(request) + '\n'
        stdout, stderr = process.communicate(input=request_json, timeout=30)
        
        print(f"✅ Response from {tool_name}:")
        
        # Try to parse JSON response
        try:
            response = json.loads(stdout.strip().split('\n')[-1])  # Get last line
            if 'result' in response:
                print(json.dumps(response['result'], indent=2))
            else:
                print(response)
        except json.JSONDecodeError:
            print("Raw output:")
            print(stdout)
            
    except subprocess.TimeoutExpired:
        print(f"❌ Timeout waiting for {tool_name}")
        process.kill()
    except Exception as e:
        print(f"❌ Error testing {tool_name}: {e}")

def main():
    print("🚀 Testing Forward MCP Server Semantic Search Functionality")
    print("=" * 60)
    
    # Test 1: Query Index Stats
    test_mcp_tool("get_query_index_stats", {"detailed": True})
    
    # Test 2: Search for security queries
    test_mcp_tool("search_nqe_queries", {
        "query": "security vulnerabilities", 
        "limit": 3
    })
    
    # Test 3: Search for BGP routing
    test_mcp_tool("search_nqe_queries", {
        "query": "BGP routing problems", 
        "limit": 3
    })
    
    # Test 4: Search for interface utilization
    test_mcp_tool("search_nqe_queries", {
        "query": "interface utilization", 
        "limit": 3
    })
    
    # Test 5: Find executable query
    test_mcp_tool("find_executable_query", {
        "query": "show me all network devices",
        "limit": 2
    })
    
    print("\n✅ All tests completed!")

if __name__ == "__main__":
    main() 