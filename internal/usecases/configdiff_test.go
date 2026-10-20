package usecases

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/forward-mcp/internal/domain"
)

func configRows(device string, lines ...string) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(lines))
	for _, l := range lines {
		rows = append(rows, map[string]interface{}{"device": device, "line": l})
	}
	return rows
}

func TestDiffConfigLines(t *testing.T) {
	before := []string{"hostname a", "!", "ntp server 1.1.1.1", "!", "end"}
	after := []string{"hostname a", "!", "ntp server 2.2.2.2", "end", "logging host 3.3.3.3"}
	added, removed := diffConfigLines(before, after)
	if want := []string{"ntp server 2.2.2.2", "logging host 3.3.3.3"}; !reflect.DeepEqual(added, want) {
		t.Errorf("added = %v, want %v", added, want)
	}
	// One "!" fewer counts as one removed line.
	if want := []string{"ntp server 1.1.1.1", "!"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

func TestGetConfigDiff(t *testing.T) {
	service := createTestService()
	mock := service.forwardClient.(*MockForwardClient)
	beforeRows := append(configRows("sjc-r1", "hostname sjc-r1", "router bgp 1", "router bgp 1 > neighbor 10.0.0.1"),
		configRows("sjc-r2", "hostname sjc-r2")...)
	beforeRows = append(beforeRows, configRows("atl-r1", "hostname atl-r1")...)
	afterRows := append(configRows("sjc-r1", "hostname sjc-r1", "router bgp 1", "router bgp 1 > neighbor 10.0.0.2"),
		configRows("sjc-r2", "hostname sjc-r2")...)
	afterRows = append(afterRows, configRows("sjc-r3", "hostname sjc-r3")...)
	mock.nqeTextResults = map[string]*domain.NQERunResult{
		"100": {SnapshotID: "100", Items: beforeRows},
		"200": {SnapshotID: "200", Items: afterRows},
	}

	res, err := service.GetConfigDiff(context.Background(), GetConfigDiffArgs{NetworkID: "162112", BeforeSnapshot: "100", AfterSnapshot: "200"})
	if err != nil {
		t.Fatalf("GetConfigDiff: %v", err)
	}
	var out struct {
		DevicesCompared    int                  `json:"devicesCompared"`
		DevicesChanged     int                  `json:"devicesChanged"`
		Changes            []deviceConfigChange `json:"changes"`
		ConfigOnlyInBefore []string             `json:"configOnlyInBefore"`
		ConfigOnlyInAfter  []string             `json:"configOnlyInAfter"`
	}
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, res.Text)
	}
	if out.DevicesCompared != 2 || out.DevicesChanged != 1 {
		t.Errorf("compared %d changed %d, want 2 and 1", out.DevicesCompared, out.DevicesChanged)
	}
	if len(out.Changes) != 1 || out.Changes[0].Device != "sjc-r1" ||
		!reflect.DeepEqual(out.Changes[0].Added, []string{"router bgp 1 > neighbor 10.0.0.2"}) ||
		!reflect.DeepEqual(out.Changes[0].Removed, []string{"router bgp 1 > neighbor 10.0.0.1"}) {
		t.Errorf("changes = %+v", out.Changes)
	}
	if !reflect.DeepEqual(out.ConfigOnlyInBefore, []string{"atl-r1"}) || !reflect.DeepEqual(out.ConfigOnlyInAfter, []string{"sjc-r3"}) {
		t.Errorf("only before %v, only after %v", out.ConfigOnlyInBefore, out.ConfigOnlyInAfter)
	}

	// device_filter limits the devices compared.
	res, err = service.GetConfigDiff(context.Background(), GetConfigDiffArgs{NetworkID: "162112", BeforeSnapshot: "100", AfterSnapshot: "200", DeviceFilter: "atl-*"})
	if err != nil {
		t.Fatalf("GetConfigDiff with filter: %v", err)
	}
	if strings.Contains(res.Text, "sjc-") {
		t.Errorf("device_filter atl-* returned sjc devices: %s", res.Text)
	}

	// max_lines cuts the line lists, not the counts.
	res, err = service.GetConfigDiff(context.Background(), GetConfigDiffArgs{NetworkID: "162112", BeforeSnapshot: "100", AfterSnapshot: "200", MaxLines: 1})
	if err != nil {
		t.Fatalf("GetConfigDiff with max_lines: %v", err)
	}
	out.Changes = nil // decode fresh: an omitted list would keep the earlier value
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		t.Fatal(err)
	}
	c := out.Changes[0]
	if c.AddedCount != 1 || c.RemovedCount != 1 || len(c.Added)+len(c.Removed) != 1 || !strings.Contains(res.Text, "max_lines=1") {
		t.Errorf("truncated change = %+v\n%s", c, res.Text)
	}
}

func TestGetConfigDiffRejectsBadInput(t *testing.T) {
	service := createTestService()
	cases := map[string]GetConfigDiffArgs{
		"missing after":  {NetworkID: "162112", BeforeSnapshot: "100"},
		"same snapshots": {NetworkID: "162112", BeforeSnapshot: "100", AfterSnapshot: "100"},
		"bad glob":       {NetworkID: "162112", BeforeSnapshot: "100", AfterSnapshot: "200", DeviceFilter: "sjc-["},
	}
	for name, args := range cases {
		if _, err := service.GetConfigDiff(context.Background(), args); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestFetchAllNQERowsReadsEveryPage(t *testing.T) {
	service := createTestService()
	mock := service.forwardClient.(*MockForwardClient)
	items := make([]map[string]interface{}, fetchPageSize+1500)
	for i := range items {
		items[i] = map[string]interface{}{"n": i}
	}
	mock.nqeResult = &domain.NQERunResult{SnapshotID: "s1", Items: items, TotalNumItems: int64(len(items))}

	rows, snap, err := service.fetchAllNQERows(context.Background(), &domain.NQEQueryParams{NetworkID: "162112", QueryID: "FQ_x"}, len(items))
	if err != nil {
		t.Fatalf("fetchAllNQERows: %v", err)
	}
	if len(rows) != len(items) || snap != "s1" {
		t.Errorf("got %d rows from %q, want %d from s1", len(rows), snap, len(items))
	}
	if _, _, err := service.fetchAllNQERows(context.Background(), &domain.NQEQueryParams{NetworkID: "162112", QueryID: "FQ_x"}, 100); err == nil {
		t.Error("expected an error past maxRows")
	}
}

func TestBuildBloomFilterIndexesEveryRow(t *testing.T) {
	service := createTestService()
	if service.bloomManager == nil {
		t.Skip("no bloom manager in the test service")
	}
	mock := service.forwardClient.(*MockForwardClient)
	items := make([]map[string]interface{}, 1500) // more than the old 1000-row cap
	for i := range items {
		items[i] = map[string]interface{}{"Device": "dev", "IP": i}
	}
	mock.nqeResult = &domain.NQERunResult{SnapshotID: "s1", Items: items, TotalNumItems: 1500}

	res, err := service.BuildBloomFilter(context.Background(), BuildBloomFilterArgs{NetworkID: "162112", FilterType: "interface", QueryID: "FQ_x"})
	if err != nil {
		t.Fatalf("BuildBloomFilter: %v", err)
	}
	if !strings.Contains(res.Text, "Items Processed: 1500") {
		t.Errorf("expected all 1500 rows indexed:\n%s", res.Text)
	}
}

func TestListDevicesReportsPaging(t *testing.T) {
	service := createTestService()
	mock := service.forwardClient.(*MockForwardClient)
	n := len(mock.devices)
	if n == 0 {
		t.Fatal("mock has no devices")
	}

	res, err := service.ListDevices(context.Background(), ListDevicesArgs{NetworkID: "162112", Limit: n})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "total") || !strings.Contains(res.Text, "set offset to") {
		t.Errorf("a full page should point to the next offset, not claim a total:\n%s", res.Text)
	}

	res, err = service.ListDevices(context.Background(), ListDevicesArgs{NetworkID: "162112", Limit: n + 10})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "last page") {
		t.Errorf("a short page should say it is the last:\n%s", res.Text)
	}
}
