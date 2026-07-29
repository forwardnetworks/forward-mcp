// Package mcpserver configures the protocol-facing MCP server.
package mcpserver

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolVersion is the newest MCP revision supported by the pinned Go SDK.
const ProtocolVersion = "2026-07-28"

// Catalogs are immutable after server startup, so clients can safely cache
// discovery and list responses for a short period. A reconnect after a server
// upgrade receives the new catalog immediately.
const catalogCacheTTL = 5 * time.Minute

// New returns an MCP server configured for the 2026-07-28 stateless protocol
// core while retaining the SDK's negotiation support for older clients.
func New(implementation *mcp.Implementation, instructions string) *mcp.Server {
	server := mcp.NewServer(implementation, &mcp.ServerOptions{
		Instructions: instructions,
		// The SDK historically advertises logging when Capabilities is nil.
		// Logging is deprecated in 2026-07-28, so start with an empty set and
		// let registered tools, prompts, and resources be inferred normally.
		Capabilities: &mcp.ServerCapabilities{},
	})

	server.AddReceivingMiddleware(cacheHints)
	return server
}

// cacheHints supplies the cache metadata required by MCP 2026-07-28. Static
// catalogs are public; resource payloads are tenant-specific and must never be
// stored in a shared cache.
func cacheHints(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil || result == nil {
			return result, err
		}

		ttlMs := int(catalogCacheTTL / time.Millisecond)
		switch typed := result.(type) {
		case *mcp.DiscoverResult:
			typed.TTLMs = ttlMs
			typed.CacheScope = "public"
		case *mcp.ListToolsResult:
			typed.TTLMs = ttlMs
			typed.CacheScope = "public"
		case *mcp.ListPromptsResult:
			typed.TTLMs = ttlMs
			typed.CacheScope = "public"
		case *mcp.ListResourcesResult:
			typed.TTLMs = ttlMs
			typed.CacheScope = "public"
		case *mcp.ListResourceTemplatesResult:
			typed.TTLMs = ttlMs
			typed.CacheScope = "public"
		case *mcp.ReadResourceResult:
			typed.TTLMs = 0
			typed.CacheScope = "private"
		}

		return result, nil
	}
}
