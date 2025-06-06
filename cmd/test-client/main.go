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
	defer func() {
		err := cmd.Process.Kill()
		if err != nil {
			log.Printf("Failed to kill MCP server process: %v", err)
		}
	}()

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
			description: "List devices in network 162112",
			tool:        "list_devices",
			args: map[string]interface{}{
				"network_id": "162112",
				"limit":      5,
			},
		},
		{
			name:        "list_snapshots",
			description: "List snapshots for network 162112",
			tool:        "list_snapshots",
			args: map[string]interface{}{
				"network_id": "162112",
			},
		},
		{
			name:        "search_paths",
			description: "Search paths to 8.8.8.8 in network 162112",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"dst_ip":      "8.8.8.8",
				"max_results": 1,
			},
		},
		{
			name:        "customer_path_search_basic",
			description: "Customer path search: 10.0.0.1 → 10.1.0.1 (network 162112) - Cloud infrastructure",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"src_ip":      "10.0.0.1",
				"dst_ip":      "10.1.0.1",
				"max_results": 5,
			},
		},
		{
			name:        "customer_path_search_with_intent",
			description: "Customer path search with PREFER_DELIVERED intent - AWS/GCP cloud",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"src_ip":      "10.0.0.1",
				"dst_ip":      "10.1.0.1",
				"intent":      "PREFER_DELIVERED",
				"max_results": 10,
			},
		},
		{
			name:        "customer_path_search_tcp_443",
			description: "Customer path search for HTTPS (TCP 443) - Cloud infrastructure",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"src_ip":      "10.0.0.1",
				"dst_ip":      "10.1.0.1",
				"ip_proto":    6,
				"dst_port":    "443",
				"max_results": 5,
			},
		},
		{
			name:        "customer_path_search_with_functions",
			description: "Customer path search with network functions - AWS/GCP infrastructure",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":                "162112",
				"src_ip":                    "10.0.0.1",
				"dst_ip":                    "10.1.0.1",
				"include_network_functions": true,
				"max_results":               3,
			},
		},
		{
			name:        "customer_reverse_path_search",
			description: "Customer reverse path: 10.1.0.1 → 10.0.0.1 - Cloud reverse path",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"src_ip":      "10.1.0.1",
				"dst_ip":      "10.0.0.1",
				"max_results": 5,
			},
		},
		{
			name:        "diagnostic_path_search_dst_only",
			description: "Diagnostic: Any path to external IP (8.8.8.8) from cloud",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"src_ip":      "10.0.0.1",
				"dst_ip":      "8.8.8.8",
				"max_results": 3,
			},
		},
		{
			name:        "diagnostic_path_search_src_only",
			description: "Diagnostic: Path from cloud to internet destination",
			tool:        "search_paths",
			args: map[string]interface{}{
				"network_id":  "162112",
				"src_ip":      "10.0.0.1",
				"dst_ip":      "1.1.1.1",
				"max_results": 3,
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
		responseScanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			// Look for complete JSON objects
			start := 0
			for i := 0; i < len(data); i++ {
				if data[i] == '{' {
					start = i
					// Find matching closing brace
					braceCount := 1
					for j := i + 1; j < len(data); j++ {
						if data[j] == '{' {
							braceCount++
						} else if data[j] == '}' {
							braceCount--
							if braceCount == 0 {
								// Found complete JSON object
								return j + 1, data[start : j+1], nil
							}
						}
					}
				}
			}
			// Need more data
			return 0, nil, nil
		})

		if responseScanner.Scan() {
			responseText := responseScanner.Text()

			// Try to parse as JSON-RPC response
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
