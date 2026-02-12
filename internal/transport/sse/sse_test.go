package sse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/metoro-io/mcp-golang/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFlusher wraps httptest.ResponseRecorder with flushing support.
type mockFlusher struct {
	*httptest.ResponseRecorder
}

func (m *mockFlusher) Flush() {}

func newMockWriter() *mockFlusher {
	return &mockFlusher{httptest.NewRecorder()}
}

func TestNewSSETransport(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)
	assert.NotNil(t, tr)
	assert.Equal(t, "test-session", tr.sessionID)
	assert.True(t, tr.connected)
}

func TestNewSSETransport_NoFlusher(t *testing.T) {
	// httptest.ResponseRecorder without the Flush wrapper
	w := httptest.NewRecorder()

	// ResponseRecorder does implement Flush, so this test verifies the success path
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)
	assert.NotNil(t, tr)
}

func TestSSETransport_Start(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	err = tr.Start(context.Background())
	require.NoError(t, err)

	// Check SSE headers
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	// Check endpoint event was sent
	body := w.Body.String()
	assert.Contains(t, body, "event: endpoint")
	assert.Contains(t, body, "data: /message?sessionId=test-session")
}

func TestSSETransport_SendResponse(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)
	require.NoError(t, tr.Start(context.Background()))

	// Create a response channel that Send should route to
	ch := make(chan *transport.BaseJsonRpcMessage, 1)
	tr.mu.Lock()
	tr.responseCh[42] = ch
	tr.mu.Unlock()

	resp := &transport.BaseJsonRpcMessage{
		Type: transport.BaseMessageTypeJSONRPCResponseType,
		JsonRpcResponse: &transport.BaseJSONRPCResponse{
			Id:      42,
			Jsonrpc: "2.0",
			Result:  json.RawMessage(`{"ok":true}`),
		},
	}

	err = tr.Send(context.Background(), resp)
	require.NoError(t, err)

	// Should be routed to channel, not SSE stream
	received := <-ch
	assert.Equal(t, transport.RequestId(42), received.JsonRpcResponse.Id)
}

func TestSSETransport_SendSSEEvent(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)
	require.NoError(t, tr.Start(context.Background()))

	// Send a notification (no matching response channel) - should go to SSE stream
	msg := &transport.BaseJsonRpcMessage{
		Type: transport.BaseMessageTypeJSONRPCNotificationType,
		JsonRpcNotification: &transport.BaseJSONRPCNotification{
			Jsonrpc: "2.0",
			Method:  "test/notification",
		},
	}

	err = tr.Send(context.Background(), msg)
	require.NoError(t, err)

	body := w.Body.String()
	assert.Contains(t, body, "event: message")
}

func TestSSETransport_Close(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	closeCalled := false
	tr.SetCloseHandler(func() {
		closeCalled = true
	})

	err = tr.Close()
	require.NoError(t, err)
	assert.True(t, closeCalled)
	assert.False(t, tr.connected)
}

func TestSSETransport_SendAfterClose(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	tr.Close()

	err = tr.Send(context.Background(), &transport.BaseJsonRpcMessage{
		Type: transport.BaseMessageTypeJSONRPCNotificationType,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transport is closed")
}

func TestSSETransport_Handlers(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	errorCalled := false
	tr.SetErrorHandler(func(err error) {
		errorCalled = true
	})
	assert.NotNil(t, tr.onError)

	messageCalled := false
	tr.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) {
		messageCalled = true
	})
	assert.NotNil(t, tr.onMessage)

	// Verify error handler was stored (we don't call it directly in this test)
	_ = errorCalled
	_ = messageCalled
}

func TestSSETransport_HandlePostMessage(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)
	require.NoError(t, tr.Start(context.Background()))

	// Set up a message handler that echoes back a response via Send
	tr.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) {
		if msg.Type == transport.BaseMessageTypeJSONRPCRequestType {
			resp := &transport.BaseJsonRpcMessage{
				Type: transport.BaseMessageTypeJSONRPCResponseType,
				JsonRpcResponse: &transport.BaseJSONRPCResponse{
					Id:      msg.JsonRpcRequest.Id,
					Jsonrpc: "2.0",
					Result:  json.RawMessage(`{"tools":[]}`),
				},
			}
			tr.Send(ctx, resp)
		}
	})

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	resp, err := tr.HandlePostMessage(context.Background(), []byte(reqBody))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, transport.BaseMessageTypeJSONRPCResponseType, resp.Type)
	// Original ID should be restored
	assert.Equal(t, transport.RequestId(1), resp.JsonRpcResponse.Id)
}

func TestSSETransport_HandlePostMessage_Notification(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	notificationReceived := false
	tr.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) {
		if msg.Type == transport.BaseMessageTypeJSONRPCNotificationType {
			notificationReceived = true
		}
	})

	reqBody := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	resp, err := tr.HandlePostMessage(context.Background(), []byte(reqBody))
	require.NoError(t, err)
	assert.Nil(t, resp) // Notifications return nil
	assert.True(t, notificationReceived)
}

func TestSSETransport_HandlePostMessage_InvalidJSON(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	_, err = tr.HandlePostMessage(context.Background(), []byte("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize")
}

func TestSSETransport_HandlePostMessage_ContextCancel(t *testing.T) {
	w := newMockWriter()
	tr, err := NewSSETransport("test-session", w, "/message?sessionId=test-session")
	require.NoError(t, err)

	// Set up handler that does NOT respond (simulates slow server)
	tr.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) {
		// Do nothing - no response will be sent
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	_, err = tr.HandlePostMessage(ctx, []byte(reqBody))
	assert.Error(t, err)
}

func TestSSEServer_SessionLifecycle(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)
	assert.NotNil(t, sseServer)
	assert.Empty(t, sseServer.sessions)
}

func TestSSEServer_HandleMessage_NoSession(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	req := httptest.NewRequest(http.MethodPost, "/message?sessionId=nonexistent", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	sseServer.handleMessage(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSSEServer_HandleMessage_MissingSessionID(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	sseServer.handleMessage(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSSEServer_HandleMessage_WrongMethod(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	req := httptest.NewRequest(http.MethodGet, "/message?sessionId=test", nil)
	w := httptest.NewRecorder()

	sseServer.handleMessage(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestSSEServer_HandleSSE_WrongMethod(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	req := httptest.NewRequest(http.MethodPost, "/sse", nil)
	w := httptest.NewRecorder()

	sseServer.handleSSE(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestSSEServer_HandleMessage_CORS(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	req := httptest.NewRequest(http.MethodOptions, "/message?sessionId=test", nil)
	w := httptest.NewRecorder()

	sseServer.handleMessage(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestSSEServer_ConcurrentSessions(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	var wg sync.WaitGroup
	sessionCount := 10

	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Create a mock SSE transport and register it
			w := newMockWriter()
			sessionID := "test-session"
			tr, err := NewSSETransport(sessionID, w, "/message?sessionId="+sessionID)
			if err != nil {
				t.Errorf("failed to create transport: %v", err)
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sseServer.mu.Lock()
			sseServer.sessions[sessionID+"-"+time.Now().String()] = &session{
				transport: tr,
				cancel:    cancel,
			}
			sseServer.mu.Unlock()

			_ = tr.Start(ctx)
		}()
	}

	wg.Wait()

	sseServer.mu.RLock()
	assert.Equal(t, sessionCount, len(sseServer.sessions))
	sseServer.mu.RUnlock()
}

func TestSSEServer_Shutdown(t *testing.T) {
	factory := func(tr transport.Transport) error {
		return tr.Start(context.Background())
	}

	sseServer := NewSSEServer(":0", "", factory)

	// Add a mock session
	w := newMockWriter()
	tr, _ := NewSSETransport("test", w, "/message?sessionId=test")
	_, cancel := context.WithCancel(context.Background())
	sseServer.sessions["test"] = &session{transport: tr, cancel: cancel}

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	err := sseServer.Shutdown(ctx)
	require.NoError(t, err)
	assert.Empty(t, sseServer.sessions)
}

func TestSSEServer_Integration(t *testing.T) {
	// Full integration test using httptest server
	factory := func(tr transport.Transport) error {
		// Simulate MCP server: start transport and set up a simple handler
		if err := tr.Start(context.Background()); err != nil {
			return err
		}
		tr.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) {
			if msg.Type == transport.BaseMessageTypeJSONRPCRequestType {
				resp := &transport.BaseJsonRpcMessage{
					Type: transport.BaseMessageTypeJSONRPCResponseType,
					JsonRpcResponse: &transport.BaseJSONRPCResponse{
						Id:      msg.JsonRpcRequest.Id,
						Jsonrpc: "2.0",
						Result:  json.RawMessage(`{"capabilities":{}}`),
					},
				}
				tr.Send(ctx, resp)
			}
		})
		return nil
	}

	sseServer := NewSSEServer(":0", "", factory)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", sseServer.handleSSE)
	mux.HandleFunc("/message", sseServer.handleMessage)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Step 1: Connect to SSE endpoint
	resp, err := http.Get(ts.URL + "/sse")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// Read the endpoint event
	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	require.NoError(t, err)
	eventData := string(buf[:n])
	assert.Contains(t, eventData, "event: endpoint")
	assert.Contains(t, eventData, "data: /message?sessionId=")

	// Extract session ID from the endpoint event
	lines := strings.Split(eventData, "\n")
	var messageURL string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			messageURL = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	require.NotEmpty(t, messageURL)

	// Step 2: Send a JSON-RPC request via POST
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	postResp, err := http.Post(ts.URL+messageURL, "application/json", strings.NewReader(initReq))
	require.NoError(t, err)
	defer postResp.Body.Close()

	assert.Equal(t, http.StatusOK, postResp.StatusCode)

	postBody, err := io.ReadAll(postResp.Body)
	require.NoError(t, err)

	var jsonRPCResp map[string]interface{}
	err = json.Unmarshal(postBody, &jsonRPCResp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", jsonRPCResp["jsonrpc"])
	assert.Equal(t, float64(1), jsonRPCResp["id"])
}
