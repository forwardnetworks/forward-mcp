package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/forward-mcp/internal/ports"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type captureLog struct {
	mu    sync.Mutex
	lines []string
}

func (c *captureLog) add(f string, a ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fmt.Sprintf(f, a...))
}
func (c *captureLog) Debug(f string, a ...interface{})                      { c.add(f, a...) }
func (c *captureLog) Info(f string, a ...interface{})                       { c.add(f, a...) }
func (c *captureLog) Warn(f string, a ...interface{})                       { c.add(f, a...) }
func (c *captureLog) Error(f string, a ...interface{})                      { c.add(f, a...) }
func (c *captureLog) LogToolCall(string, interface{}, time.Duration, error) {}
func (c *captureLog) contains(s string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.lines {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

type pingArgs struct{}

func newMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "ping", Description: "Tool to ping."},
		func(context.Context, *mcp.CallToolRequest, pingArgs) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "pong"}}}, nil, nil
		})
	return s
}

func newTestServer(t *testing.T, cfg *ports.HTTPConfig, log ports.Logger) *httptest.Server {
	t.Helper()
	h, err := New(cfg, log).Handler(newMCPServer())
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func apiKeyConfig() *ports.HTTPConfig {
	return &ports.HTTPConfig{
		Enabled:  true,
		AuthMode: "api-key",
		APIKeys:  map[string]string{"good-key": "alice"},
	}
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func callPing(t *testing.T, transport mcp.Transport) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ping"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if got := res.Content[0].(*mcp.TextContent).Text; got != "pong" {
		t.Fatalf("got %q, want pong", got)
	}
}

func TestHealthNeedsNoAuth(t *testing.T) {
	ts := newTestServer(t, apiKeyConfig(), &captureLog{})
	for _, path := range []string{"/health", "/ready"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestMCPEndpointsRejectMissingOrBadKey(t *testing.T) {
	ts := newTestServer(t, apiKeyConfig(), &captureLog{})
	for _, path := range []string{MCPPath, LegacySSEPath} {
		for _, auth := range []string{"", "Bearer wrong-key"} {
			req, _ := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader("{}"))
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s auth=%q: status %d, want 401", path, auth, resp.StatusCode)
			}
		}
	}
}

func TestStreamableHTTPToolCall(t *testing.T) {
	log := &captureLog{}
	ts := newTestServer(t, apiKeyConfig(), log)
	callPing(t, &mcp.StreamableClientTransport{
		Endpoint:   ts.URL + MCPPath,
		HTTPClient: &http.Client{Transport: bearer{"good-key"}},
	})
	if !log.contains("user=alice") {
		t.Error("request log does not name the authenticated user")
	}
}

func TestLegacySSEToolCall(t *testing.T) {
	ts := newTestServer(t, apiKeyConfig(), &captureLog{})
	callPing(t, &mcp.SSEClientTransport{
		Endpoint:   ts.URL + LegacySSEPath,
		HTTPClient: &http.Client{Transport: bearer{"good-key"}},
	})
}

func TestRateLimitReturns429WithConfiguredLimit(t *testing.T) {
	cfg := apiKeyConfig()
	cfg.RateLimit = 1
	ts := newTestServer(t, cfg, &captureLog{})

	var last *http.Response
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+MCPPath, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer good-key")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		last = resp
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second request: status %d, want 429", last.StatusCode)
	}
	if got := last.Header.Get("X-RateLimit-Limit"); got != "1" {
		t.Errorf("X-RateLimit-Limit = %q, want 1", got)
	}
}

func TestStartHasNoServerWideWriteTimeout(t *testing.T) {
	cfg := apiKeyConfig()
	cfg.Host, cfg.Port, cfg.AllowInsecure, cfg.WriteTimeout = "127.0.0.1", 0, true, 1
	s := New(cfg, &captureLog{})
	if err := s.Start(context.Background(), newMCPServer()); err != nil {
		t.Fatal(err)
	}
	defer s.Stop(context.Background())
	if s.httpServer.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v; a server-wide write timeout cuts long-lived MCP streams", s.httpServer.WriteTimeout)
	}
}

func TestStartRefusesPlainHTTPWithoutOptIn(t *testing.T) {
	cfg := apiKeyConfig()
	if err := New(cfg, &captureLog{}).Start(context.Background(), newMCPServer()); err == nil {
		t.Fatal("Start succeeded without TLS or FORWARD_HTTP_ALLOW_INSECURE")
	}
}
