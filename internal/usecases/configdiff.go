package usecases

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/forward-mcp/internal/domain"
)

// configLinesQuery returns one row per configuration line, up to three levels
// deep. A nested line carries its parents, joined by " > ", so the same line
// under two different parents stays two different lines.
const configLinesQuery = `foreach d in network.devices
foreach l1 in d.files.config
let lines = [l1.text] + (foreach c in l1.children select l1.text + " > " + c.text) + (foreach c in l1.children foreach g in c.children select l1.text + " > " + c.text + " > " + g.text)
foreach x in lines
select {device: d.name, line: x}`

const (
	// maxConfigRows bounds the lines read from one snapshot.
	maxConfigRows = 300000
	// defaultConfigDiffLines and maxConfigDiffLines bound the changed lines returned.
	defaultConfigDiffLines = 300
	maxConfigDiffLines     = 2000
)

// deviceConfigChange is the change to one device's configuration.
type deviceConfigChange struct {
	Device       string   `json:"device"`
	AddedCount   int      `json:"addedCount"`
	RemovedCount int      `json:"removedCount"`
	Added        []string `json:"added,omitempty"`
	Removed      []string `json:"removed,omitempty"`
}

// GetConfigDiff compares device configurations between two snapshots. It reads
// every configuration line from each snapshot and diffs them per device. The
// library query named "Config Diff" does not do this: it compares devices
// against a pattern, within one snapshot.
func (s *Service) GetConfigDiff(ctx context.Context, args GetConfigDiffArgs) (*Result, error) {
	s.logToolCall("get_config_diff", args, nil)

	if args.BeforeSnapshot == "" || args.AfterSnapshot == "" {
		return nil, fmt.Errorf("before_snapshot and after_snapshot are both required. Use list_snapshots to find snapshot IDs")
	}
	if args.BeforeSnapshot == args.AfterSnapshot {
		return nil, fmt.Errorf("before_snapshot and after_snapshot are the same snapshot (%s). Give two different snapshots from list_snapshots", args.BeforeSnapshot)
	}
	networkID := s.getNetworkID(ctx, args.NetworkID)
	if err := s.validateNetworkID(networkID); err != nil {
		return nil, err
	}
	patterns, err := parseDeviceGlobs(args.DeviceFilter)
	if err != nil {
		return nil, err
	}
	maxLines := args.MaxLines
	if maxLines <= 0 {
		maxLines = defaultConfigDiffLines
	} else if maxLines > maxConfigDiffLines {
		maxLines = maxConfigDiffLines
	}

	// The two snapshots do not depend on each other, so read them at the same time.
	var before, after map[string][]string
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		before, err = s.configLines(gctx, networkID, args.BeforeSnapshot, patterns)
		return err
	})
	g.Go(func() (err error) {
		after, err = s.configLines(gctx, networkID, args.AfterSnapshot, patterns)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var onlyBefore, onlyAfter, both []string
	for d := range before {
		if _, ok := after[d]; ok {
			both = append(both, d)
		} else {
			onlyBefore = append(onlyBefore, d)
		}
	}
	for d := range after {
		if _, ok := before[d]; !ok {
			onlyAfter = append(onlyAfter, d)
		}
	}
	sort.Strings(both)
	sort.Strings(onlyBefore)
	sort.Strings(onlyAfter)

	changes := []deviceConfigChange{}
	budget, truncated := maxLines, false
	for _, d := range both {
		added, removed := diffConfigLines(before[d], after[d])
		if len(added) == 0 && len(removed) == 0 {
			continue
		}
		c := deviceConfigChange{Device: d, AddedCount: len(added), RemovedCount: len(removed)}
		c.Removed, budget = takeLines(removed, budget)
		c.Added, budget = takeLines(added, budget)
		if len(c.Added) < len(added) || len(c.Removed) < len(removed) {
			truncated = true
		}
		changes = append(changes, c)
	}

	out := map[string]interface{}{
		"beforeSnapshotId": args.BeforeSnapshot,
		"afterSnapshotId":  args.AfterSnapshot,
		"devicesCompared":  len(both),
		"devicesChanged":   len(changes),
		"changes":          changes,
	}
	if len(onlyBefore) > 0 {
		out["configOnlyInBefore"] = onlyBefore
	}
	if len(onlyAfter) > 0 {
		out["configOnlyInAfter"] = onlyAfter
	}
	notes := []string{"Lines nested under a parent read 'parent > child'; lines deeper than three levels are not compared."}
	if len(both) == 0 && len(onlyBefore) == 0 && len(onlyAfter) == 0 {
		notes = append(notes, "No device configuration was found in either snapshot. Check device_filter, which is case-sensitive.")
	} else if len(changes) == 0 {
		notes = append(notes, "No configuration differences between the snapshots for the devices compared.")
	}
	if truncated {
		notes = append(notes, fmt.Sprintf("Line lists stop at max_lines=%d; addedCount and removedCount are complete. Use device_filter to see one device in full.", maxLines))
	}
	out["note"] = strings.Join(notes, " ")
	return textResult(MarshalCompactJSONString(out)), nil
}

// configLines returns each device's configuration lines in one snapshot, in
// file order, for the devices that match patterns (all devices when empty).
func (s *Service) configLines(ctx context.Context, networkID, snapshotID string, patterns []string) (map[string][]string, error) {
	rows, _, err := s.fetchAllNQERows(ctx, &domain.NQEQueryParams{
		NetworkID:  networkID,
		SnapshotID: snapshotID,
		Query:      configLinesQuery,
	}, maxConfigRows)
	if err != nil {
		return nil, fmt.Errorf("failed to read configurations from snapshot %s: %w", snapshotID, err)
	}
	lines := map[string][]string{}
	for _, row := range rows {
		device, _ := row["device"].(string)
		line, _ := row["line"].(string)
		if device == "" || !matchesAnyGlob(device, patterns) {
			continue
		}
		lines[device] = append(lines[device], line)
	}
	return lines, nil
}

// diffConfigLines returns the lines added and removed between two versions of
// one configuration. Lines are counted, so a repeated line that appears once
// more is one added line. Each list keeps the order of its own version.
func diffConfigLines(before, after []string) (added, removed []string) {
	count := map[string]int{}
	for _, l := range before {
		count[l]++
	}
	for _, l := range after {
		if count[l] > 0 {
			count[l]--
		} else {
			added = append(added, l)
		}
	}
	for i := len(before) - 1; i >= 0; i-- { // the unmatched copies are the last ones
		if l := before[i]; count[l] > 0 {
			count[l]--
			removed = append(removed, l)
		}
	}
	for i, j := 0, len(removed)-1; i < j; i, j = i+1, j-1 {
		removed[i], removed[j] = removed[j], removed[i]
	}
	return added, removed
}

// takeLines returns up to budget lines and the budget left.
func takeLines(lines []string, budget int) ([]string, int) {
	if len(lines) > budget {
		lines = lines[:budget]
	}
	return lines, budget - len(lines)
}

// parseDeviceGlobs splits a comma-separated list of device name globs.
func parseDeviceGlobs(filter string) ([]string, error) {
	var patterns []string
	for _, p := range strings.Split(filter, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if _, err := path.Match(p, ""); err != nil {
			return nil, fmt.Errorf("device_filter pattern %q is not a valid glob. Use * as a wildcard, such as sjc-*", p)
		}
		patterns = append(patterns, p)
	}
	return patterns, nil
}

// matchesAnyGlob reports whether name matches one of patterns, or patterns is empty.
func matchesAnyGlob(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, _ := path.Match(p, name); ok {
			return true
		}
	}
	return false
}
