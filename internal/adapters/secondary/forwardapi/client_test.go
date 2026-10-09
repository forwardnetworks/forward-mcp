package forwardapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forward-mcp/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nopLogger discards diagnostics.
type nopLogger struct{}

func (nopLogger) Debug(string, ...interface{}) {}
func (nopLogger) Info(string, ...interface{})  {}
func (nopLogger) Warn(string, ...interface{})  {}
func (nopLogger) Error(string, ...interface{}) {}

func testClient(url string) *Client {
	return NewClient(&ports.ForwardConfig{
		APIKey:     "test-api-key",
		APISecret:  "test-api-secret",
		APIBaseURL: url,
		Timeout:    5,
	}, nopLogger{})
}

func TestClient_GetNetworks(t *testing.T) {
	tests := []struct {
		name         string
		networks     []Network
		serverStatus int
		expectError  bool
	}{
		{
			name:         "successful request",
			networks:     []Network{{ID: "1", Name: "lab"}, {ID: "2", Name: "prod"}},
			serverStatus: http.StatusOK,
		},
		{
			name:         "server error",
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				auth := base64.StdEncoding.EncodeToString([]byte("test-api-key:test-api-secret"))
				assert.Equal(t, "Basic "+auth, r.Header.Get("Authorization"))
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/networks", r.URL.Path)

				w.WriteHeader(tt.serverStatus)
				if tt.networks != nil {
					assert.NoError(t, json.NewEncoder(w).Encode(tt.networks))
				}
			}))
			defer server.Close()

			networks, err := testClient(server.URL).GetNetworks(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, networks)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.networks, networks)
			}
		})
	}
}

// A cancelled MCP request must stop the HTTP request it started, not wait for
// the API to answer.
func TestClient_CancelStopsRequest(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := testClient(server.URL).GetNetworks(ctx)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled), "want context.Canceled, got %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("GetNetworks kept running after its context was cancelled")
	}
}
