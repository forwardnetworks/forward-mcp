package ports

import "context"

type sessionKey struct{}

// WithSessionID records which MCP session a tool call belongs to. Primary
// adapters set it; use cases read it to keep per-session settings apart.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionKey{}, id)
}

// SessionID returns the MCP session of a tool call, or "" when there is none
// (stdio has a single session and uses the shared settings).
func SessionID(ctx context.Context) string {
	id, _ := ctx.Value(sessionKey{}).(string)
	return id
}
