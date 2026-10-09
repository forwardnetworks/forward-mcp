package main

// Test MCP tool listing to verify ADR-2610091555 implementation
// This script starts the MCP server and requests the tools list

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type MCPRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolsList struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func main() {
	fmt.Println("🧪 MCP Protocol Tool Quality Test")
	fmt.Println("==================================\n")

	// Start MCP server
	fmt.Println("Starting MCP server...")
	cmd := exec.Command("./bin/forward-mcp-server")
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("❌ Failed to create stdin pipe: %v\n", err)
		os.Exit(1)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("❌ Failed to create stdout pipe: %v\n", err)
		os.Exit(1)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("❌ Failed to create stderr pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ Failed to start server: %v\n", err)
		os.Exit(1)
	}
	defer cmd.Process.Kill()

	// Read stderr in background (server logs)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Suppress server logs for cleaner output
		}
	}()

	// Give server time to start
	time.Sleep(500 * time.Millisecond)
	fmt.Println("✅ Server started\n")

	// Send initialize request
	fmt.Println("Sending initialize request...")
	initReq := MCPRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	if err := json.NewEncoder(stdin).Encode(initReq); err != nil {
		fmt.Printf("❌ Failed to send initialize: %v\n", err)
		os.Exit(1)
	}

	// Read initialize response
	var initResp MCPResponse
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &initResp); err != nil {
			fmt.Printf("❌ Failed to parse initialize response: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println("✅ Initialize successful\n")

	// Request tools list
	fmt.Println("Requesting tools list...")
	toolsReq := MCPRequest{
		Jsonrpc: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	if err := json.NewEncoder(stdin).Encode(toolsReq); err != nil {
		fmt.Printf("❌ Failed to send tools/list: %v\n", err)
		os.Exit(1)
	}

	// Read tools response
	var toolsResp MCPResponse
	if scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &toolsResp); err != nil {
			fmt.Printf("❌ Failed to parse tools response: %v\n", err)
			fmt.Printf("Raw response: %s\n", scanner.Text())
			os.Exit(1)
		}
	}

	if toolsResp.Error != nil {
		fmt.Printf("❌ Server returned error: %s\n", toolsResp.Error.Message)
		os.Exit(1)
	}

	// Parse tools list
	var toolsList ToolsList
	if err := json.Unmarshal(toolsResp.Result, &toolsList); err != nil {
		fmt.Printf("❌ Failed to parse tools list: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Received %d tools\n\n", len(toolsList.Tools))

	// Verify tool quality standards
	fmt.Println("Verifying ADR-2610091555 compliance...")
	fmt.Println("=====================================\n")

	templateCount := 0
	tooLongCount := 0
	hasEmojiCount := 0

	for _, tool := range toolsList.Tools {
		// Check template compliance (starts with "Tool to")
		if strings.HasPrefix(tool.Description, "Tool to") {
			templateCount++
		}

		// Check length (max 1024 chars per Composio standard)
		if len(tool.Description) > 1024 {
			tooLongCount++
			fmt.Printf("⚠️  %s: description too long (%d chars)\n", tool.Name, len(tool.Description))
		}

		// Check for emojis (should be removed)
		if strings.ContainsAny(tool.Description, "🔍📊🔧⚠️💾🗑️") {
			hasEmojiCount++
			fmt.Printf("⚠️  %s: contains emojis\n", tool.Name)
		}
	}

	// Report results
	fmt.Printf("Template compliance: %d/%d tools (%.1f%%)\n", templateCount, len(toolsList.Tools), float64(templateCount)/float64(len(toolsList.Tools))*100)
	fmt.Printf("Length violations: %d tools\n", tooLongCount)
	fmt.Printf("Emoji violations: %d tools\n\n", hasEmojiCount)

	// Show sample tool descriptions
	fmt.Println("Sample Tool Descriptions (first 5 tools):")
	fmt.Println("==========================================")
	for i := 0; i < 5 && i < len(toolsList.Tools); i++ {
		tool := toolsList.Tools[i]
		fmt.Printf("\n%d. %s\n", i+1, tool.Name)
		fmt.Printf("   %s\n", truncate(tool.Description, 150))
	}

	// Final verdict
	fmt.Println("\n\nFinal Verdict:")
	fmt.Println("==============")
	if templateCount == len(toolsList.Tools) && tooLongCount == 0 && hasEmojiCount == 0 {
		fmt.Println("✅ ALL TESTS PASSED")
		fmt.Printf("   - %d tools registered\n", len(toolsList.Tools))
		fmt.Println("   - 100% template compliance")
		fmt.Println("   - No length violations")
		fmt.Println("   - No emoji violations")
		fmt.Println("\n🎉 ADR-2610091555 implementation verified!")
	} else {
		fmt.Println("❌ SOME TESTS FAILED")
		fmt.Printf("   - %d/%d tools follow template\n", templateCount, len(toolsList.Tools))
		if tooLongCount > 0 {
			fmt.Printf("   - %d tools exceed 1024 chars\n", tooLongCount)
		}
		if hasEmojiCount > 0 {
			fmt.Printf("   - %d tools contain emojis\n", hasEmojiCount)
		}
		os.Exit(1)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
