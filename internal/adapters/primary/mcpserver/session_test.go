package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
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

type twoNetworksAPI struct{ ports.ForwardAPI }

func (twoNetworksAPI) GetNetworks(context.Context) ([]ports.Network, error) {
	return []ports.Network{{ID: "101", Name: "lab"}, {ID: "202", Name: "prod"}}, nil
}

// One remote user's set_default_network must not change another user's default.
func TestDefaultNetworkIsPerSession(t *testing.T) {
	cfg := &domain.Config{Forward: domain.ForwardConfig{APIBaseURL: "https://test.example.com"}}
	log := logger.New()
	embedder := embeddings.NewMockEmbeddingService()
	svc := usecases.New(cfg, log, usecases.Deps{
		API:        twoNetworksAPI{},
		Cache:      semcache.NewSemanticCache(embedder, log, "test", &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
	})
	t.Cleanup(func() { _ = svc.Shutdown(5 * time.Second) })

	server := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	if err := Register(server, svc, log); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := func() *mcp.ClientSession {
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).
			Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cs.Close() })
		return cs
	}
	call := func(cs *mcp.ClientSession, tool string, args map[string]any) string {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return b.String()
	}

	alice, bob, carol := session(), session(), session()
	call(alice, "set_default_network", map[string]any{"network_identifier": "101"})
	call(bob, "set_default_network", map[string]any{"network_identifier": "prod"})

	for name, tc := range map[string]struct {
		cs   *mcp.ClientSession
		want string
	}{
		"alice": {alice, `"default_network_id":"101"`},
		"bob":   {bob, `"default_network_id":"202"`},
		"carol": {carol, `"default_network_id":""`},
	} {
		got := call(tc.cs, "get_default_settings", nil)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: settings do not contain %s:\n%s", name, tc.want, got)
		}
	}
}
