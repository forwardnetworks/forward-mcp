package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/forward-mcp/internal/config"
)

// MCPRequest represents a request to the MCP server
type MCPRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a response from the MCP server
type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// ToolCallParams represents parameters for calling a tool
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

func main() {
	fmt.Println("🚀 Forward Networks MCP Test Client")
	fmt.Println("===================================")

	// Load config to verify setup
	cfg := config.LoadConfig()
	if cfg.Forward.APIKey == "" {
		fmt.Println("❌ No API key found. Make sure your .env file is configured.")
		return
	}

	fmt.Printf("✅ Connected to: %s\n", cfg.Forward.APIBaseURL)
	fmt.Printf("🔒 TLS Skip Verify: %v\n\n", cfg.Forward.InsecureSkipVerify)

	// Start the MCP server process
	cmd := exec.Command("./bin/forward-mcp-server")
	cmd.Env = os.Environ() // Pass through environment variables

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
	defer cmd.Process.Kill()

	fmt.Println("📡 MCP Server started. Available commands:")
	fmt.Println()

	// Test available tools
	testCommands := []struct {
		name        string
		description string
		tool        string
		args        map[string]interface{}
	}{
		{
			name:        "list_networks",
			description: "List all networks",
			tool:        "list_networks",
			args:        map[string]interface{}{},
		},
		{
			name:        "list_devices",
			description: "List devices in network 101",
			tool:        "list_devices",
			args: map[string]interface{}{
				"network_id": "101",
				"limit":      5,
			},
		},
		{
			name:        "list_snapshots",
			description: "List snapshots for network 101",
			tool:        "list_snapshots",
			args: map[string]interface{}{
				"network_id": "101",
			},
		},
		{
			name:        "search_paths",
			description: "Search paths to 8.8.8.8 in network 101",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "101",
				"dst_ip":      "8.8.8.8",
				"max_results": 1,
			},
		},
	}

	// Print available commands
	for i, cmd := range testCommands {
		fmt.Printf("%d. %s - %s\n", i+1, cmd.name, cmd.description)
	}
	fmt.Println("0. Exit")
	fmt.Println()

	// Interactive mode
	scanner := bufio.NewScanner(os.Stdin)
	requestID := 1

	for {
		fmt.Print("Enter command number (or 'help' for list): ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if input == "0" || input == "exit" || input == "quit" {
			fmt.Println("👋 Goodbye!")
			break
		}

		if input == "help" {
			for i, cmd := range testCommands {
				fmt.Printf("%d. %s - %s\n", i+1, cmd.name, cmd.description)
			}
			fmt.Println("0. Exit")
			continue
		}

		// Parse command number
		var cmdIndex int
		if _, err := fmt.Sscanf(input, "%d", &cmdIndex); err != nil {
			fmt.Println("❌ Invalid input. Enter a number or 'help'.")
			continue
		}

		if cmdIndex < 1 || cmdIndex > len(testCommands) {
			fmt.Println("❌ Invalid command number.")
			continue
		}

		selectedCmd := testCommands[cmdIndex-1]

		// Send MCP request
		fmt.Printf("🔄 Executing: %s...\n", selectedCmd.description)

		request := MCPRequest{
			Jsonrpc: "2.0",
			ID:      requestID,
			Method:  "tools/call",
			Params: ToolCallParams{
				Name:      selectedCmd.tool,
				Arguments: selectedCmd.args,
			},
		}
		requestID++

		// Send request
		requestBytes, _ := json.Marshal(request)
		if _, err := stdin.Write(append(requestBytes, '\n')); err != nil {
			fmt.Printf("❌ Failed to send request: %v\n", err)
			continue
		}

		// Read response
		responseScanner := bufio.NewScanner(stdout)
		if responseScanner.Scan() {
			responseText := responseScanner.Text()

			var response MCPResponse
			if err := json.Unmarshal([]byte(responseText), &response); err != nil {
				fmt.Printf("❌ Failed to parse response: %v\n", err)
				fmt.Printf("Raw response: %s\n", responseText)
				continue
			}

			if response.Error != nil {
				fmt.Printf("❌ Error: %v\n", response.Error)
			} else {
				fmt.Printf("✅ Success!\n")
				// Pretty print the result
				resultBytes, _ := json.MarshalIndent(response.Result, "", "  ")
				fmt.Printf("📊 Result:\n%s\n", string(resultBytes))
			}
		} else {
			fmt.Println("❌ No response received")
		}

		fmt.Println()
	}
}
