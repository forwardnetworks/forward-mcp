package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/adapters/secondary/queryindex"
	"github.com/forward-mcp/internal/adapters/secondary/semcache"
	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/domain"
	"github.com/forward-mcp/internal/ports"
	"github.com/forward-mcp/internal/usecases"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeAPI answers GetNetworks; any other Forward API call panics, which
// would show up as a failed test.
type fakeAPI struct{ ports.ForwardAPI }

func (fakeAPI) GetNetworks(context.Context) ([]ports.Network, error) {
	return []ports.Network{{ID: "101", Name: "lab"}}, nil
}

// connect serves the use cases on an MCP server and returns a client session
// connected to it over in-memory transports.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	cfg := &domain.Config{Forward: domain.ForwardConfig{APIBaseURL: "https://test.example.com"}}
	log := logger.New()
	embedder := embeddings.NewMockEmbeddingService()
	svc := usecases.New(cfg, log, usecases.Deps{
		API:        fakeAPI{},
		Cache:      semcache.NewSemanticCache(embedder, log, "test", &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
	})
	t.Cleanup(func() { _ = svc.Shutdown(5 * time.Second) })

	server := mcp.NewServer(&mcp.Implementation{Name: "forward-mcp-test", Version: "0.0.1"}, nil)
	if err := Register(server, svc, log); err != nil {
		t.Fatalf("Register: %v", err)
	}
	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestRegisterServesEverything(t *testing.T) {
	cs := connect(t)
	ctx := context.Background()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(tools.Tools) != 55 {
		t.Errorf("tools: got %d, want 55", len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		if toolAnnotations[tool.Name] == nil {
			t.Errorf("tool %s has no annotations", tool.Name)
		}
	}
	prompts, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("prompts/list: %v", err)
	}
	if len(prompts.Prompts) != 6 {
		t.Errorf("prompts: got %d, want 6", len(prompts.Prompts))
	}
	resources, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("resources/list: %v", err)
	}
	if len(resources.Resources) != 1 {
		t.Errorf("resources: got %d, want 1", len(resources.Resources))
	}
}

// A tool call crosses the adapter: the use case's Result arrives as exactly
// one text block.
func TestToolCallReturnsOneTextBlock(t *testing.T) {
	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_networks", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if res.IsError {
		t.Fatalf("list_networks returned a tool error: %+v", res.Content)
	}
	if len(res.Content) != 1 {
		t.Fatalf("content blocks: got %d, want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type: got %T, want *mcp.TextContent", res.Content[0])
	}
	if !strings.Contains(text.Text, "lab") {
		t.Errorf("list_networks text does not name the network: %q", text.Text)
	}
}

func TestToCallToolResult(t *testing.T) {
	if toCallToolResult(nil) != nil {
		t.Error("nil Result must render as nil")
	}
	r := toCallToolResult(&usecases.Result{Text: "hello"})
	if len(r.Content) != 1 || r.Content[0].(*mcp.TextContent).Text != "hello" {
		t.Errorf("unexpected rendering: %+v", r.Content)
	}
}
