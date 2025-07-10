package main

import (
"encoding/json"
"fmt"
"log"

"github.com/forward-mcp/internal/config"
"github.com/forward-mcp/internal/logger"
"github.com/forward-mcp/internal/service"
)

func main() {
// Load config
cfg, err := config.LoadConfig()
if err != nil {
log.Fatalf("Failed to load config: %v", err)
}

// Create logger
logger := logger.New()

// Create MCP service
mcpService, err := service.NewForwardMCPService(cfg, logger)
if err != nil {
log.Fatalf("Failed to create MCP service: %v", err)
}

// Test list queries to see repository information
fmt.Println("🔍 Testing repository information in query listing...")

// Get a few org queries
fmt.Println("\n📊 Org repository queries:")
response, err := mcpService.CallTool("search_nqe_queries", map[string]interface{}{
"query": "Users",
"limit": 3,
})
if err != nil {
log.Fatalf("Failed to search org queries: %v", err)
}

// Parse and display results
var results []map[string]interface{}
if err := json.Unmarshal([]byte(response.Content[0].Text), &results); err == nil {
for i, result := range results {
fmt.Printf("  %d. Path: %s\n", i+1, result["path"])
fmt.Printf("     Repository: %s\n", result["repository"])
fmt.Printf("     Intent: %s\n\n", result["intent"])
}
}

// Get a few fwd queries
fmt.Println("📊 Fwd repository queries:")
response, err = mcpService.CallTool("search_nqe_queries", map[string]interface{}{
"query": "Cloud",
"limit": 3,
})
if err != nil {
log.Fatalf("Failed to search fwd queries: %v", err)
}

// Parse and display results
if err := json.Unmarshal([]byte(response.Content[0].Text), &results); err == nil {
for i, result := range results {
fmt.Printf("  %d. Path: %s\n", i+1, result["path"])
fmt.Printf("     Repository: %s\n", result["repository"])
fmt.Printf("     Intent: %s\n\n", result["intent"])
}
}

fmt.Println("✅ Repository information test completed!")
}
