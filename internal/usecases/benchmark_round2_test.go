package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/forward-mcp/internal/domain"
)

func deviceRows() *domain.NQERunResult {
	return &domain.NQERunResult{
		SnapshotID: "snap-1",
		Items: []map[string]interface{}{
			{"Device": "a", "Type": "ROUTER", "Vendor": "CISCO", "OS": "IOS", "OS Version": "15.2", "Tags": []interface{}{}, "Location": nil, "Collection IP": "10.0.0.1"},
			{"Device": "b", "Type": "SWITCH", "Vendor": "ARISTA", "OS": "EOS", "OS Version": "4.25", "Tags": []interface{}{"x"}, "Location": "SJC", "Collection IP": "10.0.0.2"},
		},
		TotalNumItems: 2,
	}
}

// The cache key once held only the query ID and parameters, so a second call
// with a different filter or limit returned the first call's rows.
func TestRunNQEQueryByIDCacheKeepsDifferentRequestsApart(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = deviceRows()
	ctx := context.Background()

	run := func(opts *NQEQueryOptions) {
		t.Helper()
		if _, err := s.RunNQEQueryByID(ctx, RunNQEQueryByIDArgs{NetworkID: "1", QueryID: "Q_x", Options: opts}); err != nil {
			t.Fatal(err)
		}
	}
	run(&NQEQueryOptions{Limit: 2, Filters: []NQEColumnFilter{{ColumnName: "VLAN ID", Value: "10"}}})
	first := m.lastByIDParams
	run(&NQEQueryOptions{Limit: 2, Filters: []NQEColumnFilter{{ColumnName: "VLAN ID", Value: "104"}}})
	if m.lastByIDParams == first {
		t.Fatal("a request with a different filter was answered from the cache")
	}
	second := m.lastByIDParams
	run(&NQEQueryOptions{Limit: 1})
	if m.lastByIDParams == second {
		t.Fatal("a request with a different limit was answered from the cache")
	}
	third := m.lastByIDParams
	run(&NQEQueryOptions{Limit: 1}) // identical to the last call: the cache may answer
	if m.lastByIDParams != third {
		t.Error("an identical request was not answered from the cache")
	}
}

func TestRunNQEQueryByIDReturnsFirstPageWithoutALimit(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = deviceRows()
	res, err := s.RunNQEQueryByID(context.Background(), RunNQEQueryByIDArgs{NetworkID: "1", QueryID: "Q_a"})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.lastByIDParams.Options.Limit; got != defaultFirstPageLimit {
		t.Errorf("limit = %d, want %d", got, defaultFirstPageLimit)
	}
	if !strings.Contains(res.Text, "Found 2 items") || strings.Contains(res.Text, "proceed") {
		t.Errorf("want rows, not a warning: %s", res.Text)
	}
	if strings.Contains(res.Text, "Would you like to") {
		t.Error("the trailing menu is back")
	}
}

func TestRunNQEQueryByIDClampsALimitAboveTheMaximum(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = deviceRows()
	res, err := s.RunNQEQueryByID(context.Background(), RunNQEQueryByIDArgs{NetworkID: "1", QueryID: "Q_b", Options: &NQEQueryOptions{Limit: 5000}})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.lastByIDParams.Options.Limit; got != maxPageLimit {
		t.Errorf("limit = %d, want %d", got, maxPageLimit)
	}
	if !strings.Contains(res.Text, "above the page maximum") {
		t.Errorf("no clamp note: %s", res.Text)
	}
}

func TestGetDeviceBasicInfoCompactDropsColumnsAndEmptyValues(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = deviceRows()
	full, err := s.GetDeviceBasicInfo(context.Background(), GetDeviceBasicInfoArgs{NetworkID: "1", Options: &NQEQueryOptions{Limit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	compact, err := s.GetDeviceBasicInfo(context.Background(), GetDeviceBasicInfoArgs{NetworkID: "1", Compact: true, Options: &NQEQueryOptions{Limit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full.Text, "Collection IP") {
		t.Fatal("the full result lost a column: compact mode must not change it")
	}
	for _, gone := range []string{"Collection IP", "Tags", "Location"} {
		if strings.Contains(compact.Text, gone) {
			t.Errorf("compact output still has %q", gone)
		}
	}
	for _, kept := range []string{`"Device":"a"`, `"OS Version":"15.2"`, `"Vendor":"ARISTA"`} {
		if !strings.Contains(compact.Text, kept) {
			t.Errorf("compact output lost %s: %s", kept, compact.Text)
		}
	}
	if len(compact.Text) >= len(full.Text) {
		t.Errorf("compact (%d) is not smaller than full (%d)", len(compact.Text), len(full.Text))
	}
}

func TestProjectItemsDoesNotChangeTheInput(t *testing.T) {
	in := deviceRows().Items
	out := projectItems(in, []string{"Device"}, false)
	if len(out[0]) != 1 || out[0]["Device"] != "a" {
		t.Errorf("out = %v", out[0])
	}
	if len(in[0]) != 8 {
		t.Errorf("input row was changed: %v", in[0])
	}
	all := projectItems(in, nil, true) // no column list: keep all columns, drop empties
	if _, ok := all[0]["Tags"]; ok {
		t.Error("empty list was kept")
	}
	if _, ok := all[0]["Location"]; ok {
		t.Error("null was kept")
	}
	if all[0]["Collection IP"] != "10.0.0.1" {
		t.Error("a real value was lost")
	}
}

func TestSearchConfigsExplainsAnEmptyResultWhenAFilterIsSet(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = &domain.NQERunResult{SnapshotID: "snap-1", Items: []map[string]interface{}{}}
	opts := &NQEQueryOptions{Limit: 5}

	withFilter, err := s.SearchConfigs(context.Background(), SearchConfigsArgs{NetworkID: "1", SearchTerm: "ntp", DeviceFilter: "atl-.*", Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withFilter.Text, "case-sensitive glob") {
		t.Errorf("no filter hint: %s", withFilter.Text)
	}
	noFilter, err := s.SearchConfigs(context.Background(), SearchConfigsArgs{NetworkID: "1", SearchTerm: "ntp2", Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(noFilter.Text, "glob") {
		t.Errorf("filter hint shown with no filter: %s", noFilter.Text)
	}
}

func TestGetNetworkSummaryReportsCountsAndVendorBreakdown(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.nqeResult = &domain.NQERunResult{
		SnapshotID:    "snap-9",
		TotalNumItems: 7,
		Items:         []map[string]interface{}{{"Vendor": "CISCO", "OS": "IOS", "Count": 7}},
	}
	res, err := s.GetNetworkSummary(context.Background(), GetNetworkSummaryArgs{NetworkID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"devices":7`, `"interfaces":7`, `"deviceToDeviceLinks":7`, `"vrfInstances":7`, `"acls":7`, `"snapshotId":"snap-9"`, `"Vendor":"CISCO"`, "can differ from the totals"} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("summary missing %q: %s", want, res.Text)
		}
	}
}

func TestGetNetworkSummaryFailsLoudlyWhenAQueryFails(t *testing.T) {
	s := createTestService()
	m := mockOf(t, s)
	m.shouldError = true
	m.errorMessage = "boom"
	if _, err := s.GetNetworkSummary(context.Background(), GetNetworkSummaryArgs{NetworkID: "1"}); err == nil || !strings.Contains(err.Error(), "failed to count devices") {
		t.Errorf("got %v, want a count error that names the count", err)
	}
}
