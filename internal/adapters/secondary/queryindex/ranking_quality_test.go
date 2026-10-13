package queryindex

import (
	"testing"

	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
)

// Questions paired with the library query that answers them. Some share words
// with the query name and some are paraphrases.
var rankingCases = []struct{ question, want string }{
	{"bgp neighbors not established", "/L3/BGP/BGP Peerings Not Established"},
	{"which BGP sessions are down", "/L3/BGP/BGP Peerings Not Established"},
	{"interface status", "/Interfaces/Interface Status Query"},
	{"show port up and down state", "/Interfaces/Interface Status Query"},
	{"what operating systems are out of support", "/Devices/OS Support"},
	{"end of life hardware", "/Devices/Hardware Support"},
	{"bgp peers advertising a default route", "/L3/BGP/BGP Peerings Advertising Default Route"},
	{"acl entries", "/Security/ACL Entries"},
	{"device serial numbers and models", "/Devices/Device Hardware"},
	{"trunk vlans missing from spanning tree", "/L2/Trunk VLANs Not In STP"},
	{"ip addresses on interfaces", "/Interfaces/Interface IPs"},
	{"are any ip addresses duplicated", "/Interfaces/IP Address Uniqueness"},
	{"mtu along the path", "/Interfaces/Path Interface MTU"},
	{"neighbors that were not collected over cdp or lldp", "/Discovery/Uncollected CDP-LLDP neighbors"},
	{"ntp servers on f5", "/vendor-specific/F5/F5 NTP Servers"},
}

// With the offline keyword embeddings, search must rank by BM25 alone. Fusing
// in the keyword vectors cut the top-3 hits from 11 of 15 to 7 of 15.
func TestSearchWithKeywordEmbeddingsRanksByBM25Only(t *testing.T) {
	idx := NewNQEQueryIndex(embeddings.NewKeywordEmbeddingService(), logger.New())
	if err := idx.LoadFromSpec(); err != nil {
		t.Skipf("query spec not available: %v", err)
	}

	top1, top3 := 0, 0
	for _, c := range rankingCases {
		res, err := idx.SearchQueries(c.question, 10)
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range res {
			if r.MatchType != matchBM25 {
				t.Fatalf("%q: match type %q, want %q: keyword vectors must not be fused in", c.question, r.MatchType, matchBM25)
			}
			if r.Path == c.want && i == 0 {
				top1++
			}
			if r.Path == c.want && i < 3 {
				top3++
			}
		}
	}
	if top1 < 9 || top3 < 11 {
		t.Errorf("ranking quality fell: top-1 %d/15 (want 9+), top-3 %d/15 (want 11+)", top1, top3)
	}
}

func TestKeywordEmbeddingsAreMarkedNonSemantic(t *testing.T) {
	idx := NewNQEQueryIndex(embeddings.NewKeywordEmbeddingService(), logger.New())
	if got := idx.semanticRanking("anything"); got != nil {
		t.Errorf("semantic ranking ran with keyword vectors: %d hits", len(got))
	}
}
