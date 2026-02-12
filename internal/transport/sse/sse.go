package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/metoro-io/mcp-golang/transport"
)

const (
	maxMessageSize = 4 * 1024 * 1024 // 4MB
)

// SSETransport implements transport.Transport for a single SSE session.
// It writes SSE events to the client via the ResponseWriter and receives
// JSON-RPC messages via HandlePostMessage.
type SSETransport struct {
	sessionID  string
	writer     http.ResponseWriter
	flusher    http.Flusher
	mu         sync.Mutex
	connected  bool
	responseCh map[int64]chan *transport.BaseJsonRpcMessage

	onClose   func()
	onError   func(error)
	onMessage func(ctx context.Context, message *transport.BaseJsonRpcMessage)

	messageEndpoint string // The POST URL clients should use
}

// NewSSETransport creates a new SSE transport for a single client connection.
// messageEndpoint is the full URL path for POST messages (e.g., "/message?sessionId=xxx").
func NewSSETransport(sessionID string, w http.ResponseWriter, messageEndpoint string) (*SSETransport, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	return &SSETransport{
		sessionID:       sessionID,
		writer:          w,
		flusher:         flusher,
		connected:       true,
		responseCh:      make(map[int64]chan *transport.BaseJsonRpcMessage),
		messageEndpoint: messageEndpoint,
	}, nil
}

// Start sets SSE headers and sends the initial endpoint event.
func (t *SSETransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Set SSE headers
	t.writer.Header().Set("Content-Type", "text/event-stream")
	t.writer.Header().Set("Cache-Control", "no-cache")
	t.writer.Header().Set("Connection", "keep-alive")
	t.writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Send the endpoint event telling the client where to POST messages
	if err := t.writeSSEEvent("endpoint", t.messageEndpoint); err != nil {
		return fmt.Errorf("failed to send endpoint event: %w", err)
	}

	return nil
}

// Send sends a JSON-RPC message to the SSE client as a "message" event,
// or routes it to a waiting HandlePostMessage call via the response channel.
func (t *SSETransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("transport is closed")
	}

	// If this is a response or error, try to route it to the waiting POST handler
	if message.Type == transport.BaseMessageTypeJSONRPCResponseType && message.JsonRpcResponse != nil {
		key := int64(message.JsonRpcResponse.Id)
		if ch, ok := t.responseCh[key]; ok {
			ch <- message
			return nil
		}
	}
	if message.Type == transport.BaseMessageTypeJSONRPCErrorType && message.JsonRpcError != nil {
		key := int64(message.JsonRpcError.Id)
		if ch, ok := t.responseCh[key]; ok {
			ch <- message
			return nil
		}
	}

	// Otherwise send as SSE event
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return t.writeSSEEvent("message", string(data))
}

// Close marks the transport as disconnected and calls the close handler.
func (t *SSETransport) Close() error {
	t.mu.Lock()
	handler := t.onClose
	t.connected = false
	// Close all waiting response channels
	for key, ch := range t.responseCh {
		close(ch)
		delete(t.responseCh, key)
	}
	t.mu.Unlock()

	if handler != nil {
		handler()
	}
	return nil
}

// SetCloseHandler sets the callback for when the connection is closed.
func (t *SSETransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onClose = handler
}

// SetErrorHandler sets the callback for error reporting.
func (t *SSETransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onError = handler
}

// SetMessageHandler sets the callback for incoming JSON-RPC messages.
func (t *SSETransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onMessage = handler
}

// HandlePostMessage processes a POST request containing a JSON-RPC message.
// It deserializes the message, dispatches it to the MCP server via the message handler,
// and waits for the response to come back via Send().
func (t *SSETransport) HandlePostMessage(ctx context.Context, body []byte) (*transport.BaseJsonRpcMessage, error) {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil, fmt.Errorf("transport is closed")
	}

	// Allocate internal key for response routing
	var key int64
	for key = 0; key < 1000000; key++ {
		if _, ok := t.responseCh[key]; !ok {
			break
		}
	}
	responseCh := make(chan *transport.BaseJsonRpcMessage, 1)
	t.responseCh[key] = responseCh
	t.mu.Unlock()

	// Try to deserialize the message in priority order
	var prevId *transport.RequestId
	deserialized := false

	// Try request
	var request transport.BaseJSONRPCRequest
	if err := json.Unmarshal(body, &request); err == nil {
		deserialized = true
		id := request.Id
		prevId = &id
		request.Id = transport.RequestId(key)

		t.mu.Lock()
		handler := t.onMessage
		t.mu.Unlock()

		if handler != nil {
			handler(ctx, transport.NewBaseMessageRequest(&request))
		}
	}

	// Try notification
	if !deserialized {
		var notification transport.BaseJSONRPCNotification
		if err := json.Unmarshal(body, &notification); err == nil {
			deserialized = true

			t.mu.Lock()
			handler := t.onMessage
			t.mu.Unlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageNotification(&notification))
			}

			// Notifications don't expect a response - clean up and return nil
			t.mu.Lock()
			delete(t.responseCh, key)
			t.mu.Unlock()
			return nil, nil
		}
	}

	if !deserialized {
		t.mu.Lock()
		delete(t.responseCh, key)
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to deserialize JSON-RPC message")
	}

	// Wait for the response from the MCP server (delivered via Send())
	select {
	case resp, ok := <-responseCh:
		t.mu.Lock()
		delete(t.responseCh, key)
		t.mu.Unlock()

		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}
		// Restore original request ID
		if prevId != nil && resp.JsonRpcResponse != nil {
			resp.JsonRpcResponse.Id = *prevId
		}
		if prevId != nil && resp.JsonRpcError != nil {
			resp.JsonRpcError.Id = *prevId
		}
		return resp, nil
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.responseCh, key)
		t.mu.Unlock()
		return nil, ctx.Err()
	}
}

// writeSSEEvent writes a single SSE event. Must be called with t.mu held.
func (t *SSETransport) writeSSEEvent(event, data string) error {
	_, err := fmt.Fprintf(t.writer, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}
	t.flusher.Flush()
	return nil
}

// session holds state for a single SSE client connection.
type session struct {
	transport *SSETransport
	cancel    context.CancelFunc
}

// SSEServer manages SSE connections and routes POST messages to the correct session.
type SSEServer struct {
	addr     string
	basePath string
	mu       sync.RWMutex
	sessions map[string]*session
	server   *http.Server

	// serviceFactory is called for each new SSE session. It receives the mcp.Server
	// and should register tools/prompts/resources and call Serve().
	serviceFactory func(transport transport.Transport) error
}

// NewSSEServer creates a new SSE server.
// addr is the listen address (e.g., ":8080").
// basePath is the URL prefix (e.g., "" or "/mcp").
// serviceFactory is called for each new client session to set up the MCP server.
func NewSSEServer(addr, basePath string, serviceFactory func(transport transport.Transport) error) *SSEServer {
	return &SSEServer{
		addr:           addr,
		basePath:       basePath,
		sessions:       make(map[string]*session),
		serviceFactory: serviceFactory,
	}
}

// Start starts the HTTP server. It blocks until the server is stopped.
func (s *SSEServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.basePath+"/sse", s.handleSSE)
	mux.HandleFunc(s.basePath+"/message", s.handleMessage)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server and closes all sessions.
func (s *SSEServer) Shutdown(ctx context.Context) error {
	// Close all sessions
	s.mu.Lock()
	for id, sess := range s.sessions {
		sess.cancel()
		sess.transport.Close()
		delete(s.sessions, id)
	}
	s.mu.Unlock()

	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleSSE handles GET /sse requests. Each connection creates a new MCP session.
func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := uuid.New().String()
	messageEndpoint := fmt.Sprintf("%s/message?sessionId=%s", s.basePath, sessionID)

	sseTransport, err := NewSSETransport(sessionID, w, messageEndpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())

	sess := &session{
		transport: sseTransport,
		cancel:    cancel,
	}

	s.mu.Lock()
	s.sessions[sessionID] = sess
	s.mu.Unlock()

	// Set up cleanup on disconnect
	defer func() {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		sseTransport.Close()
		cancel()
	}()

	// Set up the MCP server for this session (registers tools, starts serving)
	if err := s.serviceFactory(sseTransport); err != nil {
		http.Error(w, fmt.Sprintf("failed to initialize MCP session: %v", err), http.StatusInternalServerError)
		return
	}

	// Block until the client disconnects or context is cancelled
	<-ctx.Done()
}

// handleMessage handles POST /message?sessionId=xxx requests.
func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "sessionId query parameter required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Read body with size limit
	body, err := io.ReadAll(io.LimitReader(r.Body, maxMessageSize))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Process the message through the transport
	response, err := sess.transport.HandlePostMessage(r.Context(), body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Notifications return nil response
	if response == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Also send the response as an SSE event on the event stream
	sess.transport.mu.Lock()
	if sess.transport.connected {
		data, marshalErr := json.Marshal(response)
		if marshalErr == nil {
			sess.transport.writeSSEEvent("message", string(data))
		}
	}
	sess.transport.mu.Unlock()

	// Return the response in the POST reply as well
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	jsonData, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
}
