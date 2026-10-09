package queryindex

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/forward-mcp/internal/ports"
)

// Okapi BM25 parameters (standard values).
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// Field weights: a term in the intent counts three times, and so on. This is
// a simple BM25F: weighted term frequencies feed one BM25 score.
const (
	weightIntent      = 3
	weightDescription = 2
	weightPath        = 2
	weightQueryID     = 1
	weightCode        = 1
)

var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true,
	"by": true, "do": true, "for": true, "from": true, "how": true, "i": true, "in": true,
	"is": true, "it": true, "many": true, "me": true, "my": true, "of": true, "on": true,
	"or": true, "show": true, "that": true, "the": true, "this": true, "to": true,
	"what": true, "which": true, "with": true, "all": true, "get": true, "list": true,
	"find": true, "give": true, "there": true, "we": true, "our": true,
}

// bm25Index is an inverted index over the query library.
type bm25Index struct {
	postings map[string]map[int]float64 // term -> doc -> weighted term frequency
	docLen   []float64
	avgLen   float64
	n        int
	// src identifies the query slice this index was built from.
	src *ports.NQEQueryIndexEntry
}

// tokenize lowercases, splits on anything that is not a letter or digit,
// drops stopwords and applies light plural stemming.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if stopwords[f] {
			continue
		}
		out = append(out, stem(f))
	}
	return out
}

func stem(t string) string {
	switch {
	case len(t) > 4 && strings.HasSuffix(t, "ies"):
		return t[:len(t)-3] + "y"
	case len(t) > 3 && strings.HasSuffix(t, "s") && !strings.HasSuffix(t, "ss"):
		return t[:len(t)-1]
	}
	return t
}

func newBM25Index(queries []*ports.NQEQueryIndexEntry) *bm25Index {
	idx := &bm25Index{
		postings: make(map[string]map[int]float64),
		docLen:   make([]float64, len(queries)),
		n:        len(queries),
	}
	if len(queries) > 0 {
		idx.src = queries[0]
	}

	var total float64
	for doc, q := range queries {
		add := func(text string, weight float64) {
			for _, term := range tokenize(text) {
				p := idx.postings[term]
				if p == nil {
					p = make(map[int]float64)
					idx.postings[term] = p
				}
				p[doc] += weight
				idx.docLen[doc] += weight
			}
		}
		add(q.Intent, weightIntent)
		add(q.Description, weightDescription)
		add(q.Path, weightPath)
		add(q.QueryID, weightQueryID)
		add(q.Code, weightCode)
		total += idx.docLen[doc]
	}
	if idx.n > 0 {
		idx.avgLen = total / float64(idx.n)
	}
	return idx
}

// idf is the non-negative BM25 inverse document frequency.
func (b *bm25Index) idf(df int) float64 {
	return math.Log(1 + (float64(b.n)-float64(df)+0.5)/(float64(df)+0.5))
}

type bm25Hit struct {
	doc      int
	score    float64
	coverage float64 // fraction of distinct query terms the doc contains
}

// search returns hits ranked by BM25 score, best first.
func (b *bm25Index) search(text string) []bm25Hit {
	terms := uniq(tokenize(text))
	if len(terms) == 0 || b.n == 0 {
		return nil
	}

	scores := make(map[int]float64)
	matched := make(map[int]int)
	for _, term := range terms {
		p := b.postings[term]
		if len(p) == 0 {
			continue
		}
		idf := b.idf(len(p))
		for doc, tf := range p {
			norm := tf + bm25K1*(1-bm25B+bm25B*b.docLen[doc]/b.avgLen)
			scores[doc] += idf * tf * (bm25K1 + 1) / norm
			matched[doc]++
		}
	}

	hits := make([]bm25Hit, 0, len(scores))
	for doc, s := range scores {
		hits = append(hits, bm25Hit{doc: doc, score: s, coverage: float64(matched[doc]) / float64(len(terms))})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].doc < hits[j].doc
	})
	return hits
}

// normalized maps a hit into 0..1: term coverage times score relative to the
// best hit. The best hit scores 1.0 only if it matches every query term.
func normalized(h bm25Hit, best float64) float64 {
	if best <= 0 {
		return 0
	}
	return h.coverage * h.score / best
}

func uniq(terms []string) []string {
	seen := make(map[string]bool, len(terms))
	out := terms[:0]
	for _, t := range terms {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
