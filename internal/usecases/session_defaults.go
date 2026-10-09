package usecases

import (
	"context"
	"sync"
	"time"

	"github.com/forward-mcp/internal/ports"
)

const (
	maxSessionDefaults = 1000
	sessionDefaultTTL  = 24 * time.Hour
)

// sessionNetworks keeps the default network per MCP session, so one remote
// user's set_default_network does not move another user's queries. Calls
// without a session (stdio) use the shared default.
type sessionNetworks struct {
	mu      sync.Mutex
	shared  string
	bySess  map[string]sessionNetwork
	nowFunc func() time.Time
}

type sessionNetwork struct {
	networkID string
	lastUsed  time.Time
}

func newSessionNetworks(shared string) *sessionNetworks {
	return &sessionNetworks{shared: shared, bySess: make(map[string]sessionNetwork), nowFunc: time.Now}
}

// get returns the session's default network, or the shared one.
func (n *sessionNetworks) get(ctx context.Context) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if id := ports.SessionID(ctx); id != "" {
		if e, ok := n.bySess[id]; ok {
			e.lastUsed = n.nowFunc()
			n.bySess[id] = e
			return e.networkID
		}
	}
	return n.shared
}

// set changes the default network for the caller's session, or the shared
// default when the call has no session.
func (n *sessionNetworks) set(ctx context.Context, networkID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	id := ports.SessionID(ctx)
	if id == "" {
		n.shared = networkID
		return
	}
	now := n.nowFunc()
	n.bySess[id] = sessionNetwork{networkID: networkID, lastUsed: now}
	n.prune(now)
}

// prune drops idle sessions, then the least recently used beyond the cap.
func (n *sessionNetworks) prune(now time.Time) {
	for id, e := range n.bySess {
		if now.Sub(e.lastUsed) > sessionDefaultTTL {
			delete(n.bySess, id)
		}
	}
	for len(n.bySess) > maxSessionDefaults {
		oldestID, oldest := "", now
		for id, e := range n.bySess {
			if e.lastUsed.Before(oldest) || oldestID == "" {
				oldestID, oldest = id, e.lastUsed
			}
		}
		delete(n.bySess, oldestID)
	}
}
