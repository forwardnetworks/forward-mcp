package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/forward-mcp/internal/domain"
)

func mockOf(t *testing.T, s *Service) *MockForwardClient {
	t.Helper()
	m, ok := s.forwardClient.(*MockForwardClient)
	if !ok {
		t.Fatalf("test service does not use MockForwardClient")
	}
	return m
}

func TestSearchConfigsRejectsEmptySearchTerm(t *testing.T) {
	s := createTestService()
	for _, term := range []string{"", "   "} {
		_, err := s.SearchConfigs(context.Background(), SearchConfigsArgs{NetworkID: "1", SearchTerm: term})
		if err == nil || !strings.Contains(err.Error(), "search_term is required") {
			t.Errorf("term %q: got %v, want a search_term error", term, err)
		}
	}
}

func TestSearchConfigsSendsAllThreeQueryParameters(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = &domain.NQERunResult{SnapshotID: "snap-1"}

	_, err := s.SearchConfigs(context.Background(), SearchConfigsArgs{
		NetworkID: "1", SearchTerm: "ntp server", DeviceFilter: "atl-*, sjc-*",
		Options: &NQEQueryOptions{Limit: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := m.lastByIDParams
	if p == nil {
		t.Fatal("no query was run")
	}
	if got := p.Parameters["Config_Pattern"]; got != "ntp server" {
		t.Errorf("Config_Pattern = %v", got)
	}
	if _, ok := p.Parameters["searchPattern"]; ok {
		t.Error("old parameter name searchPattern is still sent")
	}
	if os, ok := p.Parameters["Operating_Systems"].([]string); !ok || len(os) != 0 {
		t.Errorf("Operating_Systems = %#v, want an empty list", p.Parameters["Operating_Systems"])
	}
	names, _ := p.Parameters["Device_Name_Patterns"].([]string)
	if len(names) != 2 || names[0] != "atl-*" || names[1] != "sjc-*" {
		t.Errorf("Device_Name_Patterns = %#v", p.Parameters["Device_Name_Patterns"])
	}
}

func TestSearchConfigsWithoutDeviceFilterSendsEmptyList(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = &domain.NQERunResult{SnapshotID: "snap-1"}
	if _, err := s.SearchConfigs(context.Background(), SearchConfigsArgs{NetworkID: "1", SearchTerm: "x", Options: &NQEQueryOptions{Limit: 5}}); err != nil {
		t.Fatal(err)
	}
	names, ok := m.lastByIDParams.Parameters["Device_Name_Patterns"].([]string)
	if !ok || names == nil || len(names) != 0 {
		t.Errorf("Device_Name_Patterns = %#v, want a non-nil empty list", m.lastByIDParams.Parameters["Device_Name_Patterns"])
	}
}

func TestRunNQEQueryTextRunsTheQueryAndReportsPartialResults(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = &domain.NQERunResult{
		SnapshotID:    "snap-1",
		Items:         []map[string]interface{}{{"name": "a"}, {"name": "b"}},
		TotalNumItems: 10,
	}
	res, err := s.RunNQEQueryText(context.Background(), RunNQEQueryTextArgs{
		NetworkID: "1", Query: "foreach d in network.devices select {name: d.name}",
		Options: &NQEQueryOptions{Limit: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.lastQueryText != "foreach d in network.devices select {name: d.name}" {
		t.Errorf("query text sent = %q", m.lastQueryText)
	}
	for _, want := range []string{`"totalNumItems":10`, `"rowsReturned":2`, "Partial result: 2 of 10 rows", "offset to 2"} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("result missing %q: %s", want, res.Text)
		}
	}
}

func TestRunNQEQueryTextRejectsEmptyQuery(t *testing.T) {
	s := createTestService()
	if _, err := s.RunNQEQueryText(context.Background(), RunNQEQueryTextArgs{NetworkID: "1", Query: "  "}); err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Errorf("empty query: got %v", err)
	}
}
