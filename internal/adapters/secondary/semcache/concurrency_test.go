package semcache

import (
	"fmt"
	"sync"
	"testing"

	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/ports"
)

// Remote mode serves many clients at once; run with -race.
func TestCacheIsSafeForConcurrentUse(t *testing.T) {
	cfg := &ports.SemanticCacheConfig{
		Enabled: true, MaxEntries: 50, TTLHours: 1, SimilarityThreshold: 0.85,
		MaxMemoryMB: 16, EvictionPolicy: "lru", MetricsEnabled: true,
	}
	sc := NewSemanticCache(sameVector{}, stderrlog.New(), "test", cfg)
	defer sc.Close()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				key := fmt.Sprintf("q-%d-%d", w, i)
				_ = sc.Put(key, "net", "snap", &ports.NQERunResult{SnapshotID: key})
				sc.Get(key, "net", "snap")
				sc.Get("other-"+key, "net", "snap")
				sc.GetExact(key, "net", "snap")
				sc.GetStats()
			}
		}(w)
	}
	wg.Wait()
}
