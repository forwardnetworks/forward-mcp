package usecases

import (
	"context"
	"fmt"
	"sync"

	"github.com/forward-mcp/internal/domain"
)

// GetNetworkSummaryArgs represents arguments for the network summary.
type GetNetworkSummaryArgs struct {
	NetworkID  string `json:"network_id,omitempty" jsonschema:"Network ID. Optional if default network is set."`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID. Optional; defaults to latest."`
}

// summaryCounts are the counts the summary reports. Each one is a row count of
// a small NQE query, so the definition is exact and written in the query.
var summaryCounts = []struct{ key, query string }{
	{"devices", "foreach device in network.devices\nselect {Device: device.name}"},
	{"interfaces", "foreach device in network.devices\nforeach interface in device.interfaces\nselect {Device: device.name, Interface: interface.name}"},
	{"deviceToDeviceLinks", "foreach device in network.devices\nforeach interface in device.interfaces\nforeach link in interface.links\nwhere device.name < link.deviceName || (device.name == link.deviceName && interface.name < link.ifaceName)\nselect {A: device.name, B: link.deviceName}"},
	{"vrfInstances", "foreach device in network.devices\nforeach networkInstance in device.networkInstances\nselect {Device: device.name, VRF: networkInstance.name}"},
	{"acls", "foreach device in network.devices\nforeach aclEntry in device.aclEntries\nselect distinct {Device: device.name, ACL: aclEntry.name}"},
}

const summaryVendorOSQuery = "foreach device in network.devices\ngroup device.name as deviceNames by {Vendor: device.platform.vendor, OS: device.platform.os} as key\nselect {Vendor: key.Vendor, OS: key.OS, Count: length(deviceNames)}\norder by Count desc"

// GetNetworkSummary reports the size of a network in one call: the counts above
// and the device count by vendor and OS. It is read-only.
func (s *Service) GetNetworkSummary(ctx context.Context, args GetNetworkSummaryArgs) (*Result, error) {
	s.logToolCall("get_network_summary", args, nil)

	networkID := s.getNetworkID(ctx, args.NetworkID)
	if err := s.validateNetworkID(networkID); err != nil {
		return nil, err
	}
	snapshotID := s.getSnapshotID(args.SnapshotID)

	run := func(query string, limit int) (*domain.NQERunResult, error) {
		return s.forwardClient.RunNQEQueryByString(ctx, &domain.NQEQueryParams{
			NetworkID:  networkID,
			SnapshotID: snapshotID,
			Query:      query,
			Options:    &domain.NQEQueryOptions{Limit: limit},
		})
	}

	// The counts do not depend on each other, so run them at the same time.
	// A cold first call took 22 s one after another.
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		out  = map[string]interface{}{}
		errs = make([]error, len(summaryCounts)+1) // one slot per query, so the error reported is the first in declared order
	)
	for i, c := range summaryCounts {
		wg.Add(1)
		go func(i int, key, query string) {
			defer wg.Done()
			res, err := run(query, 1) // one row is enough: the result states the total
			if err != nil {
				errs[i] = fmt.Errorf("failed to count %s: %w", key, err)
				return
			}
			if res.TotalNumItems == 0 && len(res.Items) > 0 {
				errs[i] = fmt.Errorf("failed to count %s: the API returned rows with no total", key)
				return
			}
			mu.Lock()
			out[key] = res.TotalNumItems
			if res.SnapshotID != "" {
				out["snapshotId"] = res.SnapshotID
			}
			mu.Unlock()
		}(i, c.key, c.query)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		byOS, err := run(summaryVendorOSQuery, maxPageLimit)
		if err != nil {
			errs[len(summaryCounts)] = fmt.Errorf("failed to count devices by vendor and OS: %w", err)
			return
		}
		mu.Lock()
		out["devicesByVendorAndOS"] = byOS.Items
		mu.Unlock()
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	out["note"] = "Counts come from NQE: deviceToDeviceLinks counts each link once; vrfInstances counts device and VRF pairs; acls counts distinct device and ACL name pairs. These definitions can differ from the totals shown in the Forward UI."
	return textResult(MarshalCompactJSONString(out)), nil
}
