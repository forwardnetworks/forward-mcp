package queryindex

import (
	"fmt"
	"math"
	"sort"

	"github.com/forward-mcp/internal/ports"
)

// rrfK is the Reciprocal Rank Fusion constant from Cormack et al. (2009).
const rrfK = 60

// Match types reported in QuerySearchResult.MatchType.
const (
	matchBM25     = "bm25"
	matchSemantic = "semantic"
	matchHybrid   = "hybrid"
)

// getBM25 returns the BM25 index for the current query slice, rebuilding it
// when the slice changes. Callers hold idx.mutex for reading.
func (idx *NQEQueryIndex) getBM25() *bm25Index {
	idx.bm25Mu.Lock()
	defer idx.bm25Mu.Unlock()

	var first *ports.NQEQueryIndexEntry
	if len(idx.queries) > 0 {
		first = idx.queries[0]
	}
	if idx.bm25 == nil || idx.bm25.n != len(idx.queries) || idx.bm25.src != first {
		idx.bm25 = newBM25Index(idx.queries)
	}
	return idx.bm25
}

// SearchQueries ranks queries with BM25. When queries carry real embeddings,
// it fuses the BM25 ranking with the embedding ranking (Reciprocal Rank
// Fusion), so exact terms like "bgp" and meaning-level matches both count.
func (idx *NQEQueryIndex) SearchQueries(searchText string, limit int) ([]*ports.QuerySearchResult, error) {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	if len(idx.queries) == 0 {
		return nil, fmt.Errorf("query index is empty - run LoadFromSpec() first")
	}

	bmHits := idx.getBM25().search(searchText)
	semantic := idx.semanticRanking(searchText)

	type fused struct {
		entry      *ports.NQEQueryIndexEntry
		rrf        float64
		score      float64
		inBM25     bool
		inSemantic bool
	}
	byDoc := make(map[*ports.NQEQueryIndexEntry]*fused)
	get := func(e *ports.NQEQueryIndexEntry) *fused {
		f := byDoc[e]
		if f == nil {
			f = &fused{entry: e}
			byDoc[e] = f
		}
		return f
	}

	if len(bmHits) > 0 {
		best := bmHits[0].score
		for rank, h := range bmHits {
			f := get(idx.queries[h.doc])
			f.rrf += 1.0 / float64(rrfK+rank+1)
			f.score = math.Max(f.score, normalized(h, best))
			f.inBM25 = true
		}
	}
	for rank, s := range semantic {
		f := get(s.entry)
		f.rrf += 1.0 / float64(rrfK+rank+1)
		f.score = math.Max(f.score, math.Min(s.similarity, 1))
		f.inSemantic = true
	}

	all := make([]*fused, 0, len(byDoc))
	for _, f := range byDoc {
		all = append(all, f)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].rrf != all[j].rrf {
			return all[i].rrf > all[j].rrf
		}
		return all[i].entry.QueryID < all[j].entry.QueryID
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	results := make([]*ports.QuerySearchResult, len(all))
	for i, f := range all {
		matchType := matchBM25
		switch {
		case f.inBM25 && f.inSemantic:
			matchType = matchHybrid
		case f.inSemantic:
			matchType = matchSemantic
		}
		results[i] = &ports.QuerySearchResult{
			NQEQueryIndexEntry: f.entry,
			SimilarityScore:    f.score,
			MatchType:          matchType,
		}
	}
	idx.logger.Debug("Search '%s': %d BM25 hits, %d semantic hits, returning %d", searchText, len(bmHits), len(semantic), len(results))
	return results, nil
}

type semanticHit struct {
	entry      *ports.NQEQueryIndexEntry
	similarity float64
}

// semanticRanking ranks embedded queries by cosine similarity. It returns nil
// when no query has an embedding or the search text cannot be embedded.
func (idx *NQEQueryIndex) semanticRanking(searchText string) []semanticHit {
	embedded := false
	for _, q := range idx.queries {
		if len(q.Embedding) > 0 {
			embedded = true
			break
		}
	}
	if !embedded {
		return nil
	}

	vec64, err := idx.embeddingService.GenerateEmbedding(searchText)
	if err != nil {
		idx.logger.Debug("Failed to embed search text, using BM25 only: %v", err)
		return nil
	}
	vec := make([]float32, len(vec64))
	for i, v := range vec64 {
		vec[i] = float32(v)
	}

	var hits []semanticHit
	for _, q := range idx.queries {
		if len(q.Embedding) == 0 {
			continue
		}
		sim := calculateCosineSimilarity(vec, q.Embedding)
		if q.IsStrongMeta {
			sim *= 1.2
		}
		if sim > 0.01 {
			hits = append(hits, semanticHit{entry: q, similarity: sim})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].similarity > hits[j].similarity })
	return hits
}
