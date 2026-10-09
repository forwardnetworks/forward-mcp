package service

import (
	"context"
	"testing"
	"time"

	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/adapters/secondary/queryindex"
	"github.com/forward-mcp/internal/adapters/secondary/semcache"
	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/domain"
)

// When a database cannot be opened, main.go passes a nil store and the
// service runs without it. Tools that need the store must say so, not panic.
func TestServiceRunsWithoutStores(t *testing.T) {
	cfg := &domain.Config{Forward: domain.ForwardConfig{APIBaseURL: "https://test.example.com"}}
	log := logger.New()
	embedder := embeddings.NewMockEmbeddingService()
	svc := NewForwardMCPService(cfg, log, Deps{
		API:        NewMockForwardClient(),
		Cache:      semcache.NewSemanticCache(embedder, log, "test", &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
	})
	defer svc.Shutdown(5 * time.Second)

	if _, err := svc.getMemoryStats(context.Background(), GetMemoryStatsArgs{}); err == nil {
		t.Error("get_memory_stats without a memory store: want an error, got nil")
	}
	res, err := svc.getDatabaseStatus(context.Background(), GetDatabaseStatusArgs{})
	if err != nil {
		t.Fatalf("get_database_status without a query store: %v", err)
	}
	if res == nil || res.Text == "" {
		t.Fatal("get_database_status returned no content")
	}
}
