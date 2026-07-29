package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerNegotiatesMCP20260728(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	implementation := &mcp.Implementation{
		Name:    "forward-mcp-test",
		Title:   "Forward MCP Test",
		Version: "3.1.0-test",
	}
	server := New(implementation, "test instructions")

	var (
		sawDiscover             bool
		sawInitialize           bool
		discoverProtocolVersion string
		discoverClientName      string
		discoverHasCapabilities bool
	)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			switch method {
			case "server/discover":
				sawDiscover = true
				discoverRequest, ok := request.(*mcp.ServerRequest[*mcp.DiscoverParams])
				if !ok {
					t.Fatalf("discover request type = %T", request)
				}
				discoverProtocolVersion = discoverRequest.ProtocolVersion()
				if info := discoverRequest.ClientInfo(); info != nil {
					discoverClientName = info.Name
				}
				discoverHasCapabilities = discoverRequest.ClientCapabilities() != nil
			case "initialize":
				sawInitialize = true
			}
			return next(ctx, method, request)
		}
	})

	mcp.AddTool(server, &mcp.Tool{Name: "test_tool", Description: "A test tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
		})
	server.AddResource(&mcp.Resource{
		URI:      "forward://test/context",
		Name:     "test_context",
		MIMEType: "application/json",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      "forward://test/context",
			MIMEType: "application/json",
			Text:     `{\"tenant\":\"private\"}`,
		}}}, nil
	})

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	if !sawDiscover {
		t.Fatal("client did not negotiate through server/discover")
	}
	if sawInitialize {
		t.Fatal("2026-07-28 connection unexpectedly used the legacy initialize handshake")
	}
	if discoverProtocolVersion != ProtocolVersion {
		t.Fatalf("discover request protocol version = %q, want %q", discoverProtocolVersion, ProtocolVersion)
	}
	if discoverClientName != "test-client" || !discoverHasCapabilities {
		t.Fatalf("discover request metadata = (client %q, capabilities %v)", discoverClientName, discoverHasCapabilities)
	}

	initialization := clientSession.InitializeResult()
	if initialization == nil {
		t.Fatal("client has no negotiated server metadata")
	}
	if got := initialization.ProtocolVersion; got != ProtocolVersion {
		t.Fatalf("protocol version = %q, want %q", got, ProtocolVersion)
	}
	if initialization.ServerInfo == nil || initialization.ServerInfo.Name != implementation.Name {
		t.Fatalf("server info = %#v, want name %q", initialization.ServerInfo, implementation.Name)
	}
	if initialization.Capabilities.Logging != nil {
		t.Fatal("server advertises the deprecated logging capability")
	}
	if initialization.Capabilities.Tools == nil || initialization.Capabilities.Resources == nil {
		t.Fatalf("registered capabilities were not inferred: %#v", initialization.Capabilities)
	}

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	wantTTL := int(catalogCacheTTL / time.Millisecond)
	if tools.TTLMs != wantTTL || tools.CacheScope != "public" {
		t.Fatalf("tools cache hint = (%d, %q), want (%d, public)", tools.TTLMs, tools.CacheScope, wantTTL)
	}
	if _, ok := tools.GetMeta()[mcp.MetaKeyServerInfo]; !ok {
		t.Fatal("tools/list result is missing per-response server identity")
	}

	resource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "forward://test/context"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if resource.TTLMs != 0 || resource.CacheScope != "private" {
		t.Fatalf("resource cache hint = (%d, %q), want (0, private)", resource.TTLMs, resource.CacheScope)
	}
}
