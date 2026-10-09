package queryindex

import (
	"strings"
	"testing"

	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/ports"
)

func indexOf(t *testing.T, qs ...*ports.NQEQueryIndexEntry) *NQEQueryIndex {
	t.Helper()
	idx := NewNQEQueryIndex(embeddings.NewMockEmbeddingService(), logger.New())
	idx.queries = qs
	return idx
}

func ids(rs []*ports.QuerySearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.QueryID
	}
	return out
}

func TestBM25RareTermOutranksCommonTerm(t *testing.T) {
	idx := indexOf(t,
		&ports.NQEQueryIndexEntry{QueryID: "a", Intent: "Device inventory", Description: "Device names and platforms"},
		&ports.NQEQueryIndexEntry{QueryID: "b", Intent: "Device interfaces", Description: "Interfaces per device"},
		&ports.NQEQueryIndexEntry{QueryID: "c", Intent: "Device uptime", Description: "Uptime per device"},
		&ports.NQEQueryIndexEntry{QueryID: "bgp", Intent: "BGP sessions", Description: "BGP neighbor state per device"},
	)
	rs, err := idx.SearchQueries("bgp device", 10)
	if err != nil {
		t.Fatal(err)
	}
	if rs[0].QueryID != "bgp" {
		t.Fatalf("ranking = %v; the rare term 'bgp' should put query bgp first", ids(rs))
	}
}

func TestBM25IntentOutweighsCode(t *testing.T) {
	idx := indexOf(t,
		&ports.NQEQueryIndexEntry{QueryID: "code", Intent: "Interface counters", Code: "foreach vlan in device.vlans"},
		&ports.NQEQueryIndexEntry{QueryID: "intent", Intent: "VLAN membership", Code: "foreach x in device.interfaces"},
	)
	rs, _ := idx.SearchQueries("vlan", 10)
	if len(rs) != 2 || rs[0].QueryID != "intent" {
		t.Fatalf("ranking = %v, want intent first", ids(rs))
	}
}

func TestBM25StemsPluralsAndDropsStopwords(t *testing.T) {
	idx := indexOf(t, &ports.NQEQueryIndexEntry{QueryID: "r", Intent: "Route table", Description: "Each route with next hop"})
	if rs, _ := idx.SearchQueries("show me all the routes", 10); len(rs) != 1 {
		t.Fatalf("'routes' should match 'route'; got %v", ids(rs))
	}
	if rs, _ := idx.SearchQueries("show me all the", 10); len(rs) != 0 {
		t.Fatalf("a search of only stopwords should match nothing; got %v", ids(rs))
	}
}

func TestBM25ScoresStayBetweenZeroAndOne(t *testing.T) {
	idx := indexOf(t,
		&ports.NQEQueryIndexEntry{QueryID: "both", Intent: "OSPF neighbors", Description: "OSPF adjacency state"},
		&ports.NQEQueryIndexEntry{QueryID: "one", Intent: "OSPF areas", Description: "Area configuration"},
	)
	rs, _ := idx.SearchQueries("ospf neighbor", 10)
	if len(rs) != 2 {
		t.Fatalf("got %v", ids(rs))
	}
	if rs[0].SimilarityScore != 1 {
		t.Errorf("best full match score = %v, want 1", rs[0].SimilarityScore)
	}
	if s := rs[1].SimilarityScore; s <= 0 || s >= 1 {
		t.Errorf("partial match score = %v, want between 0 and 1", s)
	}
}

func TestBM25RebuildsWhenQueriesChange(t *testing.T) {
	idx := indexOf(t, &ports.NQEQueryIndexEntry{QueryID: "old", Intent: "MTU mismatch"})
	if rs, _ := idx.SearchQueries("mtu", 10); len(rs) != 1 {
		t.Fatal("expected old query")
	}
	idx.queries = []*ports.NQEQueryIndexEntry{{QueryID: "new", Intent: "Duplex mismatch"}}
	rs, _ := idx.SearchQueries("mtu", 10)
	if len(rs) != 0 {
		t.Fatalf("stale index: still found %v", ids(rs))
	}
}

// Ranking against the real bundled query library.
func TestBM25OnRealLibrary(t *testing.T) {
	idx := NewNQEQueryIndex(embeddings.NewMockEmbeddingService(), logger.New())
	if err := idx.LoadFromSpec(); err != nil {
		t.Skipf("query library not available: %v", err)
	}
	cases := map[string]string{
		"bgp neighbors":            "/L3/BGP/BGP Neighbor Adjacency Check",
		"established bgp peerings": "/L3/BGP/Established BGP Peerings",
		"layer 2 interfaces":       "/L2/Layer 2 Interfaces",
		"vlan consistency":         "/Interfaces/VLAN Consistency",
	}
	for search, want := range cases {
		rs, err := idx.SearchQueries(search, 3)
		if err != nil || len(rs) == 0 {
			t.Errorf("%q: no results (%v)", search, err)
			continue
		}
		if !strings.HasPrefix(rs[0].Path, want) {
			t.Errorf("%q: first result %q, want %q", search, rs[0].Path, want)
		}
	}
}
