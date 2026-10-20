package usecases

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/forward-mcp/internal/domain"
)

func (s *Service) NetworkPrefixDiscoveryWorkflow(args NetworkPrefixDiscoveryArgs) (*Result, error) {
	sessionID := fmt.Sprintf("session_%v", args.SessionID)
	state := s.workflowManager.GetState(sessionID)

	switch state.CurrentStep {
	case "start":
		return s.startNetworkPrefixDiscovery(sessionID)
	case "explain_process":
		return s.explainNetworkPrefixProcess(sessionID)
	case "show_example":
		return s.showNetworkPrefixExample(sessionID)
	case "guide_analysis":
		return s.guideNetworkPrefixAnalysis(sessionID)
	default:
		return s.startNetworkPrefixDiscovery(sessionID)
	}
}

func (s *Service) startNetworkPrefixDiscovery(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "explain_process",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `🔍 **Network Prefix Discovery & Connectivity Analysis Workflow**

Welcome to the Network Prefix Discovery workflow! This powerful tool helps you:

**🎯 What We Can Discover:**
- Network prefixes (/8, /16, /24, etc.) and their device mappings
- Site-to-site connectivity using aggregated prefixes
- Network topology patterns and connectivity gaps
- Route aggregation verification across your network

**🚀 Key Capabilities:**
1. **Prefix Discovery**: Find all network prefixes and map them to devices
2. **Aggregation Analysis**: Test connectivity using different prefix levels
3. **Site Connectivity**: Analyze connectivity between different sites/locations
4. **Topology Mapping**: Create connectivity matrices for network planning

**📋 Available Steps:**
1. **explain_process** - Learn how the analysis works
2. **show_example** - See a practical example
3. **guide_analysis** - Get step-by-step guidance
4. **run_analysis** - Execute the actual analysis

**💡 Use Cases:**
- Multi-site network planning
- Network segmentation validation
- Route aggregation verification
- Connectivity gap analysis
- Network topology documentation

What would you like to explore first? Type the step name or ask questions about the process.`

	return textResult(content), nil
}

func (s *Service) explainNetworkPrefixProcess(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "show_example",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `🔬 **How Network Prefix Discovery Works**

**Step 1: Prefix Discovery**
- Query all devices in the network for their IP addresses
- Extract network prefixes from device interfaces
- Map prefixes to devices and locations
- Identify aggregation opportunities

**Step 2: Aggregation Analysis**
- Group prefixes by different levels (/8, /16, /24, etc.)
- Create connectivity test matrices
- Test paths between aggregated prefixes
- Identify connectivity patterns

**Step 3: Site Connectivity Analysis**
- Map devices to physical locations/sites
- Test connectivity between sites using aggregated prefixes
- Identify connectivity gaps and bottlenecks
- Generate connectivity reports

**Step 4: Topology Mapping**
- Create connectivity matrices for different aggregation levels
- Visualize network topology patterns
- Identify redundant paths and single points of failure
- Document network architecture

**🔧 Technical Process:**
1. Use NQE queries to discover device IP addresses
2. Extract and normalize network prefixes
3. Use bulk path search to test connectivity
4. Aggregate results by prefix levels
5. Generate comprehensive connectivity reports

**📊 Output Types:**
- Device-to-prefix mappings
- Connectivity matrices by aggregation level
- Site-to-site connectivity reports
- Network topology visualizations
- Gap analysis and recommendations

Ready to see an example? Type "show_example" to continue.`

	return textResult(content), nil
}

func (s *Service) showNetworkPrefixExample(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "guide_analysis",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := "📋 **Network Prefix Analysis Example**\n\n" +
		"**Example Scenario: Multi-Site Enterprise Network**\n\n" +
		"**Network Structure:**\n" +
		"- Site A: 10.1.0.0/16 (HQ)\n" +
		"- Site B: 10.2.0.0/16 (Branch 1)\n" +
		"- Site C: 10.3.0.0/16 (Branch 2)\n" +
		"- Site D: 192.168.1.0/24 (DMZ)\n\n" +
		"**Analysis Request:**\n" +
		"```json\n" +
		"{\n" +
		"  \"network_id\": \"162112\",\n" +
		"  \"prefix_levels\": [\"/8\", \"/16\", \"/24\"],\n" +
		"  \"from_devices\": [\"hq-router\", \"branch1-router\", \"branch2-router\"],\n" +
		"  \"to_devices\": [\"hq-router\", \"branch1-router\", \"branch2-router\", \"dmz-firewall\"],\n" +
		"  \"intent\": \"PREFER_DELIVERED\",\n" +
		"  \"max_results\": 10\n" +
		"}\n" +
		"```\n\n" +
		"**Expected Results:**\n" +
		"1. **Prefix Discovery:**\n" +
		"   - 10.0.0.0/8 (aggregated from all sites)\n" +
		"   - 10.1.0.0/16, 10.2.0.0/16, 10.3.0.0/16 (individual sites)\n" +
		"   - 192.168.1.0/24 (DMZ)\n\n" +
		"2. **Connectivity Matrix:**\n" +
		"   - Site A ↔ Site B: CONNECTED (via 10.0.0.0/8)\n" +
		"   - Site A ↔ Site C: CONNECTED (via 10.0.0.0/8)\n" +
		"   - Site A ↔ DMZ: PARTIAL (via specific routes)\n" +
		"   - All sites ↔ Internet: CONNECTED (via DMZ)\n\n" +
		"3. **Insights:**\n" +
		"   - All sites have connectivity at /8 level\n" +
		"   - DMZ has restricted access to internal sites\n" +
		"   - Redundant paths exist between major sites\n" +
		"   - Internet access is centralized through DMZ\n\n" +
		"**🔍 Key Benefits:**\n" +
		"- Understand network segmentation\n" +
		"- Validate routing policies\n" +
		"- Identify connectivity gaps\n" +
		"- Plan network expansions\n" +
		"- Document network architecture\n\n" +
		"Ready to run your own analysis? Type \"guide_analysis\" for step-by-step instructions."

	return textResult(content), nil
}

func (s *Service) guideNetworkPrefixAnalysis(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "run_analysis",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := "🎯 **Step-by-Step Network Prefix Analysis Guide**\n\n" +
		"**Step 1: Prepare Your Analysis**\n" +
		"1. Identify your target network (network_id)\n" +
		"2. Choose aggregation levels (prefix_levels)\n" +
		"3. Select source and destination devices\n" +
		"4. Set analysis parameters\n\n" +
		"**Step 2: Run the Analysis**\n" +
		"Use the analyze_network_prefixes tool with:\n" +
		"```json\n" +
		"{\n" +
		"  \"network_id\": \"your_network_id\",\n" +
		"  \"prefix_levels\": [\"/8\", \"/16\", \"/24\"],\n" +
		"  \"from_devices\": [\"device1\", \"device2\"],\n" +
		"  \"to_devices\": [\"device3\", \"device4\"],\n" +
		"  \"intent\": \"PREFER_DELIVERED\",\n" +
		"  \"max_results\": 10\n" +
		"}\n" +
		"```\n\n" +
		"**Step 3: Interpret Results**\n" +
		"- **DELIVERED**: The traced path reaches the destination and is permitted\n" +
		"- **DENIED**: The path reaches the destination but a security policy drops it\n" +
		"- **NOT_DELIVERED**: The path ends elsewhere (for example BLACKHOLE or DROPPED)\n" +
		"- **NO_PATH** / **TIMED_OUT**: The search found no path, or ran out of time\n\n" +
		"**Step 4: Generate Insights**\n" +
		"- Network segmentation analysis\n" +
		"- Connectivity gap identification\n" +
		"- Route aggregation verification\n" +
		"- Topology documentation\n\n" +
		"**🚀 Ready to Start?**\n" +
		"Use the analyze_network_prefixes tool with your specific parameters to begin the analysis.\n\n" +
		"**💡 Pro Tips:**\n" +
		"- Start with broader aggregation levels (/8, /16)\n" +
		"- Focus on key devices first\n" +
		"- Use PREFER_DELIVERED for normal connectivity testing\n" +
		"- Set reasonable max_results to avoid timeouts\n\n" +
		"Your analysis is ready to run! Use the tool with your network parameters."

	return textResult(content), nil
}

// prefixAddressesQuery lists every IPv4 interface address with its prefix length.
const prefixAddressesQuery = `foreach d in network.devices
foreach i in d.interfaces
foreach s in i.subinterfaces
foreach a in s.ipv4.addresses
select {device: d.name, ip: a.ip, prefixLength: a.prefixLength}`

const (
	// defaultPrefixTests and maxPrefixTests bound the path searches one call runs.
	defaultPrefixTests = 50
	maxPrefixTests     = 200
	// prefixBatchSize is the path searches sent in one bulk request.
	prefixBatchSize = 25
	// maxPrefixAddressRows bounds the interface addresses read.
	maxPrefixAddressRows = 200000
	// maxPrefixesListed bounds the prefixes listed per level; counts stay complete.
	maxPrefixesListed = 50
)

var validPrefixIntents = map[string]bool{"PREFER_DELIVERED": true, "PREFER_VIOLATIONS": true, "VIOLATIONS_ONLY": true}

// prefixMember is one interface address inside an aggregate prefix.
type prefixMember struct {
	device string
	ip     netip.Addr
}

// prefixGroup is the interface addresses that fall inside one aggregate prefix.
type prefixGroup struct {
	Prefix    string `json:"prefix"`
	Devices   int    `json:"devices"`
	Addresses int    `json:"addresses"`
	members   []prefixMember
}

// prefixTest is one path search between two aggregate prefixes and its outcome.
type prefixTest struct {
	Level             string `json:"level"`
	FromPrefix        string `json:"fromPrefix"`
	ToPrefix          string `json:"toPrefix"`
	FromDevice        string `json:"fromDevice"`
	SrcIP             string `json:"srcIp"`
	DstIP             string `json:"dstIp"`
	Status            string `json:"status"` // DELIVERED, DENIED, NOT_DELIVERED, NO_PATH, TIMED_OUT
	ForwardingOutcome string `json:"forwardingOutcome,omitempty"`
	SecurityOutcome   string `json:"securityOutcome,omitempty"`
	QueryURL          string `json:"queryUrl,omitempty"`
}

// AnalyzeNetworkPrefixes groups the network's IPv4 interface addresses into
// aggregate prefixes at each requested level, then traces a path between a
// representative address of each pair of prefixes and reports the outcome the
// path search returns.
func (s *Service) AnalyzeNetworkPrefixes(ctx context.Context, args NetworkPrefixAnalysisArgs) (*Result, error) {
	s.logToolCall("analyze_network_prefixes", args, nil)

	networkID := s.getNetworkID(ctx, args.NetworkID)
	if err := s.validateNetworkID(networkID); err != nil {
		return nil, err
	}
	levels, err := parsePrefixLevels(args.PrefixLevels)
	if err != nil {
		return nil, err
	}
	intent := args.Intent
	if intent == "" {
		intent = "PREFER_DELIVERED"
	}
	if !validPrefixIntents[intent] {
		return nil, fmt.Errorf("intent %q is not valid. Use PREFER_DELIVERED, PREFER_VIOLATIONS or VIOLATIONS_ONLY", intent)
	}
	maxTests := args.MaxResults
	if maxTests <= 0 {
		maxTests = defaultPrefixTests
	} else if maxTests > maxPrefixTests {
		maxTests = maxPrefixTests
	}

	rows, snapshotID, err := s.fetchAllNQERows(ctx, &domain.NQEQueryParams{
		NetworkID:  networkID,
		SnapshotID: s.getSnapshotID(args.SnapshotID),
		Query:      prefixAddressesQuery,
	}, maxPrefixAddressRows)
	if err != nil {
		return nil, fmt.Errorf("failed to read interface addresses: %w", err)
	}
	if snapshotID == "" {
		snapshotID = s.getSnapshotID(args.SnapshotID)
	}
	members := prefixMembers(rows)
	if len(members) == 0 {
		return textResult(MarshalCompactJSONString(map[string]interface{}{
			"snapshotId":       snapshotID,
			"addressesScanned": 0,
			"note":             "No IPv4 interface addresses were found, so there is nothing to group or trace. Check that the snapshot is processed with list_snapshots.",
		})), nil
	}

	fromSet, toSet := stringSet(args.FromDevices), stringSet(args.ToDevices)
	if err := requireDevicesPresent("from_devices", fromSet, members); err != nil {
		return nil, err
	}
	if err := requireDevicesPresent("to_devices", toSet, members); err != nil {
		return nil, err
	}

	// Group the addresses at each level and choose the pairs to trace.
	levelOut := []map[string]interface{}{}
	var tests []prefixTest
	candidates := 0
	for _, level := range levels {
		groups := groupByPrefix(members, level)
		listed := groups
		if len(listed) > maxPrefixesListed {
			listed = listed[:maxPrefixesListed]
		}
		levelOut = append(levelOut, map[string]interface{}{
			"level":       fmt.Sprintf("/%d", level),
			"prefixCount": len(groups),
			"prefixes":    listed,
		})
		for _, from := range groups {
			src, ok := pickMember(from.members, fromSet, "")
			if !ok {
				continue
			}
			for _, to := range groups {
				if to.Prefix == from.Prefix {
					continue
				}
				dst, ok := pickMember(to.members, toSet, src.device)
				if !ok {
					continue
				}
				candidates++
				if len(tests) < maxTests {
					tests = append(tests, prefixTest{
						Level:      fmt.Sprintf("/%d", level),
						FromPrefix: from.Prefix,
						ToPrefix:   to.Prefix,
						FromDevice: src.device,
						SrcIP:      src.ip.String(),
						DstIP:      dst.ip.String(),
					})
				}
			}
		}
	}

	if err := s.tracePrefixTests(ctx, networkID, snapshotID, intent, tests); err != nil {
		return nil, err
	}

	summary := map[string]int{"tested": len(tests)}
	for _, t := range tests {
		summary[strings.ToLower(t.Status)]++
	}
	out := map[string]interface{}{
		"snapshotId":       snapshotID,
		"addressesScanned": len(members),
		"levels":           levelOut,
		"tests":            tests,
		"summary":          summary,
		"candidatePairs":   candidates,
	}
	notes := []string{"Each test traces from one representative address in fromPrefix to one in toPrefix; it does not prove every address in the prefixes can reach each other. IPv6 is not analyzed."}
	if candidates > len(tests) {
		notes = append(notes, fmt.Sprintf("Traced %d of %d prefix pairs (max_results). Narrow with from_devices, to_devices or fewer prefix_levels to cover the rest.", len(tests), candidates))
	}
	for _, l := range levelOut {
		if l["prefixCount"].(int) > maxPrefixesListed {
			notes = append(notes, fmt.Sprintf("Prefix lists show the first %d per level; prefixCount is complete.", maxPrefixesListed))
			break
		}
	}
	out["note"] = strings.Join(notes, " ")
	return textResult(MarshalCompactJSONString(out)), nil
}

// tracePrefixTests runs the path searches for tests in batches and records
// each outcome in place.
func (s *Service) tracePrefixTests(ctx context.Context, networkID, snapshotID, intent string, tests []prefixTest) error {
	for start := 0; start < len(tests); start += prefixBatchSize {
		end := start + prefixBatchSize
		if end > len(tests) {
			end = len(tests)
		}
		batch := tests[start:end]
		req := &domain.PathSearchBulkRequest{Intent: intent, MaxResults: 1}
		for _, t := range batch {
			req.Queries = append(req.Queries, domain.PathSearchParams{From: t.FromDevice, SrcIP: t.SrcIP, DstIP: t.DstIP})
		}
		responses, err := s.forwardClient.SearchPathsBulk(ctx, networkID, req, snapshotID)
		if err != nil {
			return fmt.Errorf("failed to trace paths between prefixes: %w", err)
		}
		if len(responses) != len(batch) {
			return fmt.Errorf("path search returned %d results for %d queries. Retry, or lower max_results", len(responses), len(batch))
		}
		for i := range batch {
			classifyPrefixTest(&batch[i], responses[i])
		}
	}
	return nil
}

// classifyPrefixTest sets a test's status from its path search response. A
// delivered path wins over any other path returned.
func classifyPrefixTest(t *prefixTest, resp domain.PathSearchBulkResponse) {
	t.QueryURL = resp.QueryUrl
	paths := resp.Info.Paths
	if len(paths) == 0 {
		t.Status = "NO_PATH"
		if resp.TimedOut {
			t.Status = "TIMED_OUT"
		}
		return
	}
	p := paths[0]
	for _, candidate := range paths {
		if candidate.ForwardingOutcome == "DELIVERED" {
			p = candidate
			break
		}
	}
	t.ForwardingOutcome, t.SecurityOutcome = p.ForwardingOutcome, p.SecurityOutcome
	switch {
	case p.ForwardingOutcome != "DELIVERED":
		t.Status = "NOT_DELIVERED"
	case p.SecurityOutcome == "DENIED":
		t.Status = "DENIED"
	default:
		t.Status = "DELIVERED"
	}
}

// parsePrefixLevels turns levels such as "/16" or "16" into sorted, distinct
// IPv4 prefix lengths, defaulting to /8, /16 and /24.
func parsePrefixLevels(raw []string) ([]int, error) {
	if len(raw) == 0 {
		return []int{8, 16, 24}, nil
	}
	seen := map[int]bool{}
	var levels []int
	for _, r := range raw {
		n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(r), "/"))
		if err != nil || n < 1 || n > 32 {
			return nil, fmt.Errorf("prefix level %q is not valid. Use IPv4 prefix lengths from /1 to /32, such as [\"/8\", \"/16\", \"/24\"]", r)
		}
		if !seen[n] {
			seen[n] = true
			levels = append(levels, n)
		}
	}
	sort.Ints(levels)
	return levels, nil
}

// prefixMembers parses the address rows, dropping rows that do not parse and
// repeated device and address pairs.
func prefixMembers(rows []map[string]interface{}) []prefixMember {
	seen := map[prefixMember]bool{}
	var members []prefixMember
	for _, row := range rows {
		device, _ := row["device"].(string)
		ipText, _ := row["ip"].(string)
		ip, err := netip.ParseAddr(ipText)
		if device == "" || err != nil || !ip.Is4() {
			continue
		}
		m := prefixMember{device: device, ip: ip}
		if !seen[m] {
			seen[m] = true
			members = append(members, m)
		}
	}
	return members
}

// groupByPrefix groups members by their address masked to level bits, sorted
// by prefix. Members in a group are sorted by device, then address.
func groupByPrefix(members []prefixMember, level int) []*prefixGroup {
	byPrefix := map[netip.Prefix]*prefixGroup{}
	var keys []netip.Prefix
	for _, m := range members {
		p := netip.PrefixFrom(m.ip, level).Masked()
		g, ok := byPrefix[p]
		if !ok {
			g = &prefixGroup{Prefix: p.String()}
			byPrefix[p] = g
			keys = append(keys, p)
		}
		g.members = append(g.members, m)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Addr().Less(keys[j].Addr()) })
	groups := make([]*prefixGroup, 0, len(keys))
	for _, k := range keys {
		g := byPrefix[k]
		sort.Slice(g.members, func(i, j int) bool {
			if g.members[i].device != g.members[j].device {
				return g.members[i].device < g.members[j].device
			}
			return g.members[i].ip.Less(g.members[j].ip)
		})
		devices := map[string]bool{}
		for _, m := range g.members {
			devices[m.device] = true
		}
		g.Devices, g.Addresses = len(devices), len(g.members)
		groups = append(groups, g)
	}
	return groups
}

// pickMember returns the first member on a device in allowed (any device when
// allowed is empty) that is not on avoidDevice. Tracing from a device to its
// own address says nothing about connectivity, so such pairs are skipped.
func pickMember(members []prefixMember, allowed map[string]bool, avoidDevice string) (prefixMember, bool) {
	for _, m := range members {
		if (len(allowed) == 0 || allowed[m.device]) && m.device != avoidDevice {
			return m, true
		}
	}
	return prefixMember{}, false
}

// requireDevicesPresent fails when a named device has no IPv4 interface address.
func requireDevicesPresent(field string, names map[string]bool, members []prefixMember) error {
	if len(names) == 0 {
		return nil
	}
	present := map[string]bool{}
	for _, m := range members {
		present[m.device] = true
	}
	var missing []string
	for n := range names {
		if !present[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("%s names devices with no IPv4 interface address: %s. Device names are case-sensitive; use list_devices to check them", field, strings.Join(missing, ", "))
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			set[v] = true
		}
	}
	return set
}
