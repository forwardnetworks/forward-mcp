package semcache

import (
	"testing"

	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/ports"
)

// sameVector rates every text as identical: the worst case for a semantic match.
type sameVector struct{}

func (sameVector) GenerateEmbedding(string) ([]float64, error) { return []float64{1, 0, 0}, nil }

func TestGetExactNeverReturnsAnotherKeysResult(t *testing.T) {
	cfg := &ports.SemanticCacheConfig{Enabled: true, MaxEntries: 10, TTLHours: 1, SimilarityThreshold: 0.85, MaxMemoryMB: 16, EvictionPolicy: "lru"}
	sc := NewSemanticCache(sameVector{}, stderrlog.New(), "test", cfg)
	defer sc.Close()

	core := "query_id:FQ_1|params:map[device:core-1]"
	edge := "query_id:FQ_1|params:map[device:edge-9]"
	if err := sc.Put(core, "net", "snap", &ports.NQERunResult{SnapshotID: "core-1-result"}); err != nil {
		t.Fatal(err)
	}

	if _, hit := sc.GetExact(edge, "net", "snap"); hit {
		t.Fatal("GetExact returned core-1's result for an edge-9 lookup")
	}
	if r, hit := sc.GetExact(core, "net", "snap"); !hit || r.SnapshotID != "core-1-result" {
		t.Fatalf("GetExact missed the exact key: hit=%v", hit)
	}
	// Get still does semantic matching; this is why NQE results must not use it.
	if _, hit := sc.Get(edge, "net", "snap"); !hit {
		t.Fatal("expected Get to match semantically with identical vectors")
	}
}
