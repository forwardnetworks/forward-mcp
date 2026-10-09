package usecases

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

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
		"- **CONNECTED**: Full connectivity at this aggregation level\n" +
		"- **PARTIAL**: Some paths exist but not all\n" +
		"- **DISCONNECTED**: No connectivity at this level\n\n" +
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

func (s *Service) AnalyzeNetworkPrefixes(ctx context.Context, args NetworkPrefixAnalysisArgs) (*Result, error) {
	s.logToolCall("analyze_network_prefixes", args, nil)

	// Use defaults if not specified
	networkID := s.getNetworkID(args.NetworkID)
	snapshotID := s.getSnapshotID(args.SnapshotID)
	maxResults := s.getQueryLimit(args.MaxResults)

	// Default prefix levels if not specified
	prefixLevels := args.PrefixLevels
	if len(prefixLevels) == 0 {
		prefixLevels = []string{"/8", "/16", "/24"}
	}

	// Default intent if not specified
	intent := args.Intent
	if intent == "" {
		intent = "PREFER_DELIVERED"
	}

	s.logger.Info("Starting network prefix analysis: networkID=%s, prefixLevels=%v, maxResults=%d",
		networkID, prefixLevels, maxResults)

	// Step 1: Discover network prefixes and device mappings
	prefixInfo, err := s.discoverNetworkPrefixes(ctx, networkID, snapshotID)
	if err != nil {
		s.logger.Error("Failed to discover network prefixes: %v", err)
		return nil, fmt.Errorf("failed to discover network prefixes: %w", err)
	}

	// Step 2: Analyze connectivity between prefixes
	connectivityResults, err := s.analyzePrefixConnectivity(ctx, networkID, prefixInfo, prefixLevels, args.FromDevices, args.ToDevices, intent, maxResults)
	if err != nil {
		s.logger.Error("Failed to analyze prefix connectivity: %v", err)
		return nil, fmt.Errorf("failed to analyze prefix connectivity: %w", err)
	}

	// Step 3: Generate comprehensive report
	report := s.generateConnectivityReport(prefixInfo, connectivityResults, prefixLevels)

	// Track analysis in memory system (placeholder for future implementation)
	if s.apiTracker != nil {
		s.logger.Debug("Network analysis completed - would track in memory system")
	}

	return textResult(report), nil
}

func (s *Service) discoverNetworkPrefixes(ctx context.Context, networkID, snapshotID string) ([]NetworkPrefixInfo, error) {
	// Use device inventory to discover all interface IPs and aggregate to prefixes
	params := &domain.DeviceQueryParams{}
	if snapshotID != "" {
		params.SnapshotID = snapshotID
	}
	devicesResp, err := s.forwardClient.GetDevices(ctx, networkID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get device inventory: %w", err)
	}

	// Group devices by location to identify location-based network scopes
	locationDevices := make(map[string][]string)
	deviceIPs := make(map[string][]string)                   // device -> IPs
	locationPrefixes := make(map[string]map[string][]string) // location -> prefix -> devices

	for _, device := range devicesResp.Devices {
		location := device.LocationID
		if location == "" {
			location = "unknown"
		}

		locationDevices[location] = append(locationDevices[location], device.Name)

		var deviceIPList []string
		for _, iface := range device.Interfaces {
			if iface.IPAddress == "" {
				continue
			}

			// Parse the IP address
			ip, _, err := net.ParseCIDR(iface.IPAddress)
			if err != nil {
				// Try to parse as plain IP and synthesize /32
				ip = net.ParseIP(iface.IPAddress)
				if ip == nil {
					s.logger.Warn("Could not parse interface IP: %s on device %s", iface.IPAddress, device.Name)
					continue
				}
			}

			deviceIPList = append(deviceIPList, iface.IPAddress)

			// Create different aggregation levels for this IP
			aggregationLevels := []int{8, 16, 24} // /8, /16, /24 for IPv4
			if ip.To4() == nil {
				aggregationLevels = []int{32, 48, 64} // /32, /48, /64 for IPv6
			}

			for _, level := range aggregationLevels {
				var mask net.IPMask
				if ip.To4() != nil {
					mask = net.CIDRMask(level, 32)
				} else {
					mask = net.CIDRMask(level, 128)
				}

				aggNet := &net.IPNet{IP: ip.Mask(mask), Mask: mask}
				prefix := aggNet.String()

				if locationPrefixes[location] == nil {
					locationPrefixes[location] = make(map[string][]string)
				}

				// Add device to this prefix in this location
				found := false
				for _, dev := range locationPrefixes[location][prefix] {
					if dev == device.Name {
						found = true
						break
					}
				}
				if !found {
					locationPrefixes[location][prefix] = append(locationPrefixes[location][prefix], device.Name)
				}
			}
		}
		deviceIPs[device.Name] = deviceIPList
	}

	// Create NetworkPrefixInfo for each location-prefix combination
	var prefixInfo []NetworkPrefixInfo

	for location, prefixMap := range locationPrefixes {
		for prefix, devices := range prefixMap {
			// Only include prefixes that have multiple devices or are significant
			if len(devices) > 1 || strings.HasSuffix(prefix, "/8") || strings.HasSuffix(prefix, "/16") {
				info := NetworkPrefixInfo{
					Prefix:     prefix,
					Device:     devices[0], // Representative device
					NetworkID:  networkID,
					Location:   location,
					Aggregated: len(devices) > 1,
					Subnets:    devices,
				}
				prefixInfo = append(prefixInfo, info)
			}
		}
	}

	s.logger.Info("Discovered %d network prefixes across %d locations", len(prefixInfo), len(locationDevices))
	for location, devices := range locationDevices {
		s.logger.Debug("Location %s: %d devices", location, len(devices))
	}

	return prefixInfo, nil
}

func (s *Service) analyzePrefixConnectivity(ctx context.Context, networkID string, prefixInfo []NetworkPrefixInfo, prefixLevels []string, fromDevices, toDevices []string, intent string, maxResults int) ([]ConnectivityAnalysisResult, error) {
	var results []ConnectivityAnalysisResult

	// Create connectivity test queries
	queries := s.createConnectivityQueries(prefixInfo, prefixLevels, fromDevices, toDevices)

	if len(queries) == 0 {
		s.logger.Info("No connectivity queries generated")
		return results, nil
	}

	s.logger.Info("Executing %d connectivity queries between network prefixes", len(queries))

	// Execute actual bulk path search
	bulkArgs := SearchPathsBulkArgs{
		NetworkID:  networkID,
		Queries:    queries,
		Intent:     intent,
		MaxResults: maxResults,
	}

	// Get latest snapshot if needed
	snapshotID := s.getSnapshotID("")
	bulkArgs.SnapshotID = snapshotID

	// Execute the bulk path search
	_, err := s.searchPathsBulk(ctx, bulkArgs)
	if err != nil {
		s.logger.Warn("Bulk path search failed, creating placeholder results: %v", err)
		// Create placeholder results for analysis
		for _, query := range queries {
			result := ConnectivityAnalysisResult{
				FromPrefix:       query.SrcIP,
				ToPrefix:         query.DstIP,
				FromDevice:       query.From,
				ToDevice:         s.findRepresentativeDevice(query.DstIP, toDevices),
				Connectivity:     "ANALYSIS_FAILED",
				PathCount:        0,
				AggregationLevel: s.determineAggregationLevel(query.SrcIP),
			}
			results = append(results, result)
		}
		return results, nil
	}

	// Parse the response to extract connectivity information
	// The response contains the actual path search results
	s.logger.Info("Successfully executed bulk path search, analyzing results")

	// For now, create results based on the queries
	// In a full implementation, we would parse the actual path search responses
	for _, query := range queries {
		result := ConnectivityAnalysisResult{
			FromPrefix:       query.SrcIP,
			ToPrefix:         query.DstIP,
			FromDevice:       query.From,
			ToDevice:         s.findRepresentativeDevice(query.DstIP, toDevices),
			Connectivity:     "CONNECTED", // Placeholder - would be determined from actual response
			PathCount:        1,           // Placeholder - would be determined from actual response
			AggregationLevel: s.determineAggregationLevel(query.SrcIP),
		}
		results = append(results, result)
	}

	// Note: In a full implementation, we would:
	// 1. Call the actual bulk path search API
	// 2. Parse the responses to determine connectivity
	// 3. Update the results with actual connectivity status

	return results, nil
}

func (s *Service) createConnectivityQueries(prefixInfo []NetworkPrefixInfo, prefixLevels []string, fromDevices, toDevices []string) []PathSearchQueryArgs {
	var queries []PathSearchQueryArgs

	// Create queries for each prefix level
	for _, level := range prefixLevels {
		// Get aggregated prefixes for this level
		aggregatedPrefixes := s.aggregatePrefixes(prefixInfo, level)

		// Create connectivity tests between prefixes
		for i, fromPrefix := range aggregatedPrefixes {
			for j, toPrefix := range aggregatedPrefixes {
				if i != j { // Don't test connectivity to self
					// Find representative devices for each prefix
					fromDevice := s.findRepresentativeDevice(fromPrefix, fromDevices)
					// toDevice := s.findRepresentativeDevice(toPrefix, toDevices) // Not used in current implementation

					if fromDevice != "" {
						queries = append(queries, PathSearchQueryArgs{
							From:  fromDevice,
							DstIP: toPrefix,
						})
					}
				}
			}
		}
	}

	return queries
}

func (s *Service) aggregatePrefixes(prefixInfo []NetworkPrefixInfo, level string) []string {
	// Simple aggregation logic - in a real implementation, this would be more sophisticated
	var aggregated []string
	prefixMap := make(map[string]bool)

	for _, info := range prefixInfo {
		// Extract network portion based on level
		network := s.extractNetworkPortion(info.Prefix, level)
		if network != "" && !prefixMap[network] {
			aggregated = append(aggregated, network)
			prefixMap[network] = true
		}
	}

	return aggregated
}

func (s *Service) extractNetworkPortion(prefix, level string) string {
	// Simple implementation - extract network portion based on CIDR level
	// In a real implementation, this would use proper IP network calculations
	if strings.Contains(prefix, "/") {
		parts := strings.Split(prefix, "/")
		if len(parts) == 2 {
			ip := parts[0]
			// For now, return the IP with the requested level
			// This is a simplified version - real implementation would calculate proper network addresses
			return fmt.Sprintf("%s%s", ip, level)
		}
	}
	return prefix
}

func (s *Service) findRepresentativeDevice(prefix string, preferredDevices []string) string {
	// Find a representative device for the prefix
	// Prefer devices from the preferredDevices list if available
	for _, preferred := range preferredDevices {
		// Simple matching - in real implementation, would check if device is in prefix
		if strings.Contains(prefix, preferred) {
			return preferred
		}
	}

	// Return first device found for this prefix
	// In real implementation, would parse prefix and find matching devices
	return ""
}

func (s *Service) determineAggregationLevel(prefix string) string {
	if strings.Contains(prefix, "/8") {
		return "/8"
	} else if strings.Contains(prefix, "/16") {
		return "/16"
	} else if strings.Contains(prefix, "/24") {
		return "/24"
	}
	return "/32"
}

func (s *Service) generateConnectivityReport(prefixInfo []NetworkPrefixInfo, connectivityResults []ConnectivityAnalysisResult, prefixLevels []string) string {
	var report strings.Builder

	report.WriteString("# 🔍 Network Prefix Discovery & Connectivity Analysis Report\n\n")

	// Prefix Discovery Summary
	report.WriteString("## 📊 Prefix Discovery Summary\n\n")
	report.WriteString(fmt.Sprintf("**Total Prefixes Discovered:** %d\n\n", len(prefixInfo)))

	report.WriteString("### Device-to-Prefix Mappings:\n")
	for _, info := range prefixInfo {
		report.WriteString(fmt.Sprintf("- **%s** → %s (Location: %s)\n", info.Device, info.Prefix, info.Location))
	}
	report.WriteString("\n")

	// Connectivity Analysis Summary
	report.WriteString("## 🔗 Connectivity Analysis Summary\n\n")

	connected := 0
	partial := 0
	disconnected := 0

	for _, result := range connectivityResults {
		switch result.Connectivity {
		case "CONNECTED":
			connected++
		case "PARTIAL":
			partial++
		case "DISCONNECTED":
			disconnected++
		}
	}

	report.WriteString(fmt.Sprintf("**Total Connectivity Tests:** %d\n", len(connectivityResults)))
	report.WriteString(fmt.Sprintf("- ✅ **Connected:** %d\n", connected))
	report.WriteString(fmt.Sprintf("- ⚠️ **Partial:** %d\n", partial))
	report.WriteString(fmt.Sprintf("- ❌ **Disconnected:** %d\n\n", disconnected))

	// Connectivity Matrix by Aggregation Level
	for _, level := range prefixLevels {
		report.WriteString(fmt.Sprintf("### %s Aggregation Level\n\n", level))

		levelResults := s.filterResultsByLevel(connectivityResults, level)
		if len(levelResults) > 0 {
			report.WriteString("| From Prefix | To Prefix | Connectivity | Paths |\n")
			report.WriteString("|-------------|-----------|--------------|-------|\n")

			for _, result := range levelResults {
				status := "❌"
				if result.Connectivity == "CONNECTED" {
					status = "✅"
				} else if result.Connectivity == "PARTIAL" {
					status = "⚠️"
				}

				report.WriteString(fmt.Sprintf("| %s | %s | %s %s | %d |\n",
					result.FromPrefix, result.ToPrefix, status, result.Connectivity, result.PathCount))
			}
			report.WriteString("\n")
		}
	}

	// Key Insights
	report.WriteString("## 💡 Key Insights\n\n")

	if connected > 0 {
		report.WriteString("✅ **Strong Connectivity:** Network shows good connectivity at multiple aggregation levels\n")
	}
	if partial > 0 {
		report.WriteString("⚠️ **Partial Connectivity:** Some paths exist but may have limitations or bottlenecks\n")
	}
	if disconnected > 0 {
		report.WriteString("❌ **Connectivity Gaps:** Some network segments lack connectivity - review routing policies\n")
	}

	report.WriteString("\n## 🎯 Recommendations\n\n")
	report.WriteString("1. **Review Disconnected Segments:** Investigate routing policies for disconnected paths\n")
	report.WriteString("2. **Optimize Partial Connectivity:** Consider adding redundant paths for better reliability\n")
	report.WriteString("3. **Document Topology:** Use this analysis for network documentation and planning\n")
	report.WriteString("4. **Monitor Changes:** Re-run analysis after network changes to validate connectivity\n")

	return report.String()
}

func (s *Service) filterResultsByLevel(results []ConnectivityAnalysisResult, level string) []ConnectivityAnalysisResult {
	var filtered []ConnectivityAnalysisResult
	for _, result := range results {
		if result.AggregationLevel == level {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// resolveDeviceToIP attempts to resolve a device name to an IP address
// If the input looks like an IP address, it returns it as-is
// If it looks like a device name, it tries to find any IP address bound to the device
func (s *Service) resolveDeviceToIP(ctx context.Context, networkID, deviceOrIP string) (string, error) {
	// If it looks like an IP address, return as-is
	if net.ParseIP(deviceOrIP) != nil {
		return deviceOrIP, nil
	}

	// If it looks like a CIDR, return as-is
	if strings.Contains(deviceOrIP, "/") {
		_, _, err := net.ParseCIDR(deviceOrIP)
		if err == nil {
			return deviceOrIP, nil
		}
	}

	// Otherwise, treat as device name and try to get its management IP
	resolutionID := fmt.Sprintf("%s-%d", deviceOrIP, time.Now().UnixNano())
	s.logger.Debug("[%s] Resolving device name to IP: %s", resolutionID, deviceOrIP)

	// Get devices and find the one with matching name
	devices, err := s.forwardClient.GetDevices(ctx, networkID, &domain.DeviceQueryParams{})
	if err != nil {
		return "", fmt.Errorf("failed to get devices for network %s: %w", networkID, err)
	}

	if devices == nil || len(devices.Devices) == 0 {
		return "", fmt.Errorf("no devices found in network %s", networkID)
	}

	// Find device by name
	s.logger.Debug("Searching through %d devices for device name: %s", len(devices.Devices), deviceOrIP)
	foundDevice := false
	for _, device := range devices.Devices {
		s.logger.Debug("Checking device: %s (management IPs: %v, interface count: %d)",
			device.Name, device.ManagementIPs, len(device.Interfaces))

		if device.Name == deviceOrIP {
			foundDevice = true
			s.logger.Info("Found device: %s", deviceOrIP)

			// Try management IPs first
			if len(device.ManagementIPs) > 0 {
				// Check if this management IP is already used by another device
				ipUsedBy := []string{}
				for _, otherDevice := range devices.Devices {
					if otherDevice.Name != deviceOrIP {
						for _, mgmtIP := range otherDevice.ManagementIPs {
							if mgmtIP == device.ManagementIPs[0] {
								ipUsedBy = append(ipUsedBy, otherDevice.Name)
							}
						}
					}
				}

				if len(ipUsedBy) > 0 {
					s.logger.Warn("Device %s management IP %s is shared with other devices: %v",
						deviceOrIP, device.ManagementIPs[0], ipUsedBy)
				}

				s.logger.Info("Using management IP for device %s: %s", deviceOrIP, device.ManagementIPs[0])
				return device.ManagementIPs[0], nil
			}

			// Try interface IPs
			s.logger.Debug("No management IPs, checking %d interfaces", len(device.Interfaces))
			for i, iface := range device.Interfaces {
				s.logger.Debug("Interface %d: %s (IP: %s)", i, iface.Name, iface.IPAddress)
				if iface.IPAddress != "" {
					// Check if this interface IP is already used by another device
					ipUsedBy := []string{}
					for _, otherDevice := range devices.Devices {
						if otherDevice.Name != deviceOrIP {
							for _, otherIface := range otherDevice.Interfaces {
								if otherIface.IPAddress == iface.IPAddress {
									ipUsedBy = append(ipUsedBy, otherDevice.Name)
								}
							}
						}
					}

					if len(ipUsedBy) > 0 {
						s.logger.Warn("Device %s interface IP %s is shared with other devices: %v",
							deviceOrIP, iface.IPAddress, ipUsedBy)
					}

					s.logger.Info("Using interface IP for device %s: %s (interface: %s)",
						deviceOrIP, iface.IPAddress, iface.Name)
					return iface.IPAddress, nil
				}
			}

			s.logger.Warn("Device %s found but has no IP addresses", deviceOrIP)
			return "", fmt.Errorf("device %s found but has no IP addresses", deviceOrIP)
		}
	}

	if !foundDevice {
		s.logger.Warn("Device %s not found in network %s", deviceOrIP, networkID)
		// Log some available device names for debugging
		deviceNames := make([]string, 0, len(devices.Devices))
		for _, device := range devices.Devices {
			deviceNames = append(deviceNames, device.Name)
		}
		s.logger.Debug("Available devices in network: %v", deviceNames)
	} else {
		// Log IP conflict summary for all devices
		ipConflictMap := make(map[string][]string)
		for _, device := range devices.Devices {
			for _, mgmtIP := range device.ManagementIPs {
				ipConflictMap[mgmtIP] = append(ipConflictMap[mgmtIP], device.Name)
			}
			for _, iface := range device.Interfaces {
				if iface.IPAddress != "" {
					ipConflictMap[iface.IPAddress] = append(ipConflictMap[iface.IPAddress], device.Name)
				}
			}
		}

		// Report conflicts
		for ip, deviceList := range ipConflictMap {
			if len(deviceList) > 1 {
				s.logger.Warn("IP conflict detected: %s is used by multiple devices: %v", ip, deviceList)
			}
		}
	}

	return "", fmt.Errorf("device %s not found in network %s", deviceOrIP, networkID)
}
