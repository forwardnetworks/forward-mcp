package queryindex

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/ports"
)

// The server keeps generated embeddings in its data directory. The spec cache
// seeds the index but is never written, so a run leaves the checkout unchanged.
func TestUseEmbeddingsCacheSeedsFromSpecAndNeverWritesIt(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "spec-nqe-embeddings.json")
	seedBytes := []byte(`{"/L3/BGP": [0.5, 0.25]}`)
	if err := os.WriteFile(seed, seedBytes, 0644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, "data", "nqe-embeddings.json")
	if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
		t.Fatal(err)
	}

	idx := NewNQEQueryIndex(embeddings.NewMockEmbeddingService(), logger.New())
	idx.embeddingsCachePath = seed // as if the spec file were found here
	idx.UseEmbeddingsCache(cache)
	idx.queries = []*ports.NQEQueryIndexEntry{{QueryID: "q1", Path: "/L3/BGP"}, {QueryID: "q2", Path: "/L2/VLANs"}}

	// No cache yet: the seed is read.
	if err := idx.loadEmbeddingsFromCache(); err != nil {
		t.Fatalf("load from seed: %v", err)
	}
	if got := idx.queries[0].Embedding; len(got) != 2 || got[0] != 0.5 {
		t.Fatalf("seed embedding not loaded: %v", got)
	}

	// Saving writes the cache, not the seed.
	idx.queries[1].Embedding = []float32{1, 0}
	if err := idx.saveEmbeddingsToCache(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if after, _ := os.ReadFile(seed); !bytes.Equal(after, seedBytes) {
		t.Errorf("seed file was rewritten:\n%s", after)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	// Once the cache exists it wins over the seed.
	fresh := NewNQEQueryIndex(embeddings.NewMockEmbeddingService(), logger.New())
	fresh.embeddingsCachePath = seed
	fresh.UseEmbeddingsCache(cache)
	fresh.queries = []*ports.NQEQueryIndexEntry{{QueryID: "q2", Path: "/L2/VLANs"}}
	if err := fresh.loadEmbeddingsFromCache(); err != nil {
		t.Fatalf("load from cache: %v", err)
	}
	if got := fresh.queries[0].Embedding; len(got) != 2 || got[0] != 1 {
		t.Errorf("cache embedding not loaded: %v", got)
	}
}
