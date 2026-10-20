package usecases

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/forward-mcp/internal/domain"
)

func TestParsePrefixLevels(t *testing.T) {
	levels, err := parsePrefixLevels([]string{"/24", "8", " /16 ", "/24"})
	if err != nil || !reflect.DeepEqual(levels, []int{8, 16, 24}) {
		t.Errorf("got %v, %v", levels, err)
	}
	if levels, _ := parsePrefixLevels(nil); !reflect.DeepEqual(levels, []int{8, 16, 24}) {
		t.Errorf("default levels = %v", levels)
	}
	for _, bad := range []string{"/0", "/33", "abc"} {
		if _, err := parsePrefixLevels([]string{bad}); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestGroupByPrefixMasksAddresses(t *testing.T) {
	members := prefixMembers([]map[string]interface{}{
		{"device": "r1", "ip": "10.1.2.3"},
		{"device": "r2", "ip": "10.1.9.9"},
		{"device": "r3", "ip": "10.2.0.1"},
		{"device": "r3", "ip": "10.2.0.1"},  // repeat, dropped
		{"device": "r4", "ip": "not-an-ip"}, // dropped
	})
	groups := groupByPrefix(members, 16)
	if len(groups) != 2 || groups[0].Prefix != "10.1.0.0/16" || groups[1].Prefix != "10.2.0.0/16" {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Devices != 2 || groups[1].Addresses != 1 {
		t.Errorf("counts: %+v %+v", groups[0], groups[1])
	}
}

func TestClassifyPrefixTest(t *testing.T) {
	cases := []struct {
		resp domain.PathSearchBulkResponse
		want string
	}{
		{domain.PathSearchBulkResponse{}, "NO_PATH"},
		{domain.PathSearchBulkResponse{TimedOut: true}, "TIMED_OUT"},
		{domain.PathSearchBulkResponse{Info: domain.PathSearchInfo{Paths: []domain.BulkPath{{ForwardingOutcome: "BLACKHOLE"}}}}, "NOT_DELIVERED"},
		{domain.PathSearchBulkResponse{Info: domain.PathSearchInfo{Paths: []domain.BulkPath{{ForwardingOutcome: "DELIVERED", SecurityOutcome: "DENIED"}}}}, "DENIED"},
		{domain.PathSearchBulkResponse{Info: domain.PathSearchInfo{Paths: []domain.BulkPath{
			{ForwardingOutcome: "DROPPED"}, {ForwardingOutcome: "DELIVERED", SecurityOutcome: "PERMITTED"},
		}}}, "DELIVERED"},
	}
	for _, c := range cases {
		var tst prefixTest
		classifyPrefixTest(&tst, c.resp)
		if tst.Status != c.want {
			t.Errorf("status = %s, want %s", tst.Status, c.want)
		}
	}
}

func TestAnalyzeNetworkPrefixesTracesRealOutcomes(t *testing.T) {
	service := createTestService()
	mock := service.forwardClient.(*MockForwardClient)
	mock.nqeResult = &domain.NQERunResult{SnapshotID: "s1", Items: []map[string]interface{}{
		{"device": "r1", "ip": "10.1.0.1", "prefixLength": 30},
		{"device": "r2", "ip": "10.2.0.1", "prefixLength": 30},
		{"device": "r3", "ip": "10.3.0.1", "prefixLength": 30},
	}}
	mock.pathResponse = &domain.PathSearchResponse{Paths: []domain.Path{{Outcome: "BLACKHOLE"}}}

	res, err := service.AnalyzeNetworkPrefixes(context.Background(), NetworkPrefixAnalysisArgs{
		NetworkID: "162112", PrefixLevels: []string{"/16"}, MaxResults: 4,
	})
	if err != nil {
		t.Fatalf("AnalyzeNetworkPrefixes: %v", err)
	}
	var out struct {
		Tests          []prefixTest   `json:"tests"`
		Summary        map[string]int `json:"summary"`
		CandidatePairs int            `json:"candidatePairs"`
	}
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, res.Text)
	}
	// Three prefixes give six ordered pairs; max_results keeps four.
	if out.CandidatePairs != 6 || len(out.Tests) != 4 {
		t.Errorf("candidates %d, tests %d; want 6 and 4", out.CandidatePairs, len(out.Tests))
	}
	// The outcome comes from the path search, not a placeholder.
	if out.Summary["not_delivered"] != 4 || out.Tests[0].Status != "NOT_DELIVERED" || out.Tests[0].FromDevice != "r1" || out.Tests[0].DstIP != "10.2.0.1" {
		t.Errorf("summary %v, first test %+v", out.Summary, out.Tests[0])
	}

	if _, err := service.AnalyzeNetworkPrefixes(context.Background(), NetworkPrefixAnalysisArgs{
		NetworkID: "162112", FromDevices: []string{"nope"},
	}); err == nil {
		t.Error("expected an error for a from_devices name with no address")
	}
}
