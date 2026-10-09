package usecases

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/forward-mcp/internal/domain"
)

// SearchPathsBulkArgs represents arguments for bulk path search
type SearchPathsBulkArgs struct {
	NetworkID               string                `json:"network_id,omitempty" jsonschema:"Network ID to search in"`
	SnapshotID              string                `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID to use (optional, uses latest if omitted)"`
	Queries                 []PathSearchQueryArgs `json:"queries" jsonschema:"Array of path search queries to execute"`
	Intent                  string                `json:"intent,omitempty" jsonschema:"Search intent (PREFER_DELIVERED, PREFER_VIOLATIONS, VIOLATIONS_ONLY)"`
	MaxCandidates           int                   `json:"max_candidates,omitempty" jsonschema:"Maximum number of candidates to consider"`
	MaxResults              int                   `json:"max_results,omitempty" jsonschema:"Maximum number of results to return"`
	MaxReturnPathResults    int                   `json:"max_return_path_results,omitempty" jsonschema:"Maximum number of return path results"`
	MaxSeconds              int                   `json:"max_seconds,omitempty" jsonschema:"Maximum seconds per query"`
	MaxOverallSeconds       int                   `json:"max_overall_seconds,omitempty" jsonschema:"Maximum overall seconds for all queries"`
	IncludeNetworkFunctions bool                  `json:"include_network_functions,omitempty" jsonschema:"Include network functions in results"`
}

// PathSearchQueryArgs represents a single path search query in bulk request
type PathSearchQueryArgs struct {
	From    string `json:"from,omitempty" jsonschema:"Source device name"`
	SrcIP   string `json:"src_ip,omitempty" jsonschema:"Source IP address or subnet"`
	DstIP   string `json:"dst_ip" jsonschema:"Destination IP address or subnet"`
	IPProto *int   `json:"ip_proto,omitempty" jsonschema:"IP protocol number"`
	SrcPort string `json:"src_port,omitempty" jsonschema:"Source port"`
	DstPort string `json:"dst_port,omitempty" jsonschema:"Destination port"`
}

func (s *Service) searchPathsBulk(ctx context.Context, args SearchPathsBulkArgs) (*Result, error) {
	s.logToolCall("search_paths_bulk", args, nil)

	// Use defaults if not specified
	networkID := s.getNetworkID(args.NetworkID)
	snapshotID := s.getSnapshotID(args.SnapshotID)

	// Note: snapshotId is optional for bulk API - if omitted, the network's latest processed Snapshot is used
	// We only fetch it if explicitly requested
	if snapshotID == "latest" {
		s.logger.Info("searchPathsBulk - Latest snapshot requested, fetching for network %s", networkID)
		snapshot, err := s.forwardClient.GetLatestSnapshot(ctx, networkID)
		if err != nil {
			s.logger.Error("Failed to fetch latest snapshot for network %s: %v", networkID, err)
			return nil, fmt.Errorf("failed to get latest snapshot for network %s: %w", networkID, err)
		}
		if snapshot != nil && snapshot.ID != "" {
			snapshotID = snapshot.ID
			s.logger.Info("searchPathsBulk - Using latest snapshot ID: %s", snapshotID)
		} else {
			s.logger.Warn("No valid snapshot found for network %s", networkID)
			return nil, fmt.Errorf("no valid snapshot found for network %s - ensure the network has been processed", networkID)
		}
	}

	// Validate queries
	if len(args.Queries) == 0 {
		return nil, fmt.Errorf("at least one query must be provided in bulk path search")
	}

	// Convert queries to forward API format
	var bulkQueries []domain.PathSearchParams
	for i, query := range args.Queries {
		// Validate required fields
		if query.DstIP == "" {
			return nil, fmt.Errorf("query %d: dst_ip is required", i+1)
		}

		// Validate that we have either 'from' or 'src_ip'
		if query.From == "" && query.SrcIP == "" {
			return nil, fmt.Errorf("query %d: either 'from' or 'src_ip' must be specified", i+1)
		}

		// When 'from' is specified, srcIp is optional (API will use device as source)
		// When no 'from' is specified, srcIp is required
		srcIP := query.SrcIP
		if query.From == "" && srcIP == "" {
			return nil, fmt.Errorf("query %d: 'src_ip' is required when 'from' is not specified", i+1)
		}

		// Validate dst_ip - must be a valid IP address or CIDR (no device name resolution)
		dstIP := query.DstIP
		s.logger.Debug("Processing dst_ip: %s for query %d", dstIP, i+1)

		// Check if it's a valid IP address
		if net.ParseIP(dstIP) != nil {
			s.logger.Debug("dst_ip '%s' is a valid IP address", dstIP)
		} else if strings.Contains(dstIP, "/") {
			// Check if it's a valid CIDR
			if _, _, err := net.ParseCIDR(dstIP); err != nil {
				return nil, fmt.Errorf("query %d: dst_ip '%s' is not a valid IP address or CIDR: %w", i+1, dstIP, err)
			}
			s.logger.Debug("dst_ip '%s' is a valid CIDR", dstIP)
		} else {
			// Not an IP or CIDR - reject device names
			return nil, fmt.Errorf("query %d: dst_ip '%s' must be a valid IP address or CIDR (device names are not supported)", i+1, dstIP)
		}

		params := domain.PathSearchParams{
			From:    query.From,
			SrcIP:   srcIP,
			DstIP:   dstIP,
			SrcPort: query.SrcPort,
			DstPort: query.DstPort,
		}

		if query.IPProto != nil {
			params.IPProto = query.IPProto
		}

		bulkQueries = append(bulkQueries, params)
	}

	s.logger.Debug("Bulk path search: networkID=%s, snapshotID=%s, queries=%d",
		networkID, snapshotID, len(bulkQueries))

	// Create the bulk request with top-level parameters
	bulkRequest := &domain.PathSearchBulkRequest{
		Queries:                 bulkQueries,
		Intent:                  args.Intent,
		MaxCandidates:           args.MaxCandidates,
		MaxResults:              args.MaxResults,
		MaxReturnPathResults:    args.MaxReturnPathResults,
		MaxSeconds:              args.MaxSeconds,
		MaxOverallSeconds:       args.MaxOverallSeconds,
		IncludeNetworkFunctions: args.IncludeNetworkFunctions,
	}

	// Execute bulk path search
	// Pass empty string if no snapshotId is needed (API will use latest processed snapshot)
	apiSnapshotID := ""
	if snapshotID != "" && snapshotID != "latest" {
		apiSnapshotID = snapshotID
	}
	responses, err := s.forwardClient.SearchPathsBulk(ctx, networkID, bulkRequest, apiSnapshotID)
	if err != nil {
		s.logger.Error("Bulk path search failed: %v", err)
		return nil, fmt.Errorf("failed to execute bulk path search: %w", err)
	}

	s.logger.Debug("Bulk path search API returned %d responses", len(responses))
	if len(responses) > 0 {
		s.logger.Debug("First response structure: %+v", responses[0])
	}

	// Track bulk path search in memory system
	if s.apiTracker != nil {
		for i, response := range responses {
			if i < len(args.Queries) {
				query := args.Queries[i]
				// Convert bulk response to legacy format for tracking
				legacyResponse := &domain.PathSearchResponse{
					Paths: make([]domain.Path, len(response.Info.Paths)),
				}
				for j, bulkPath := range response.Info.Paths {
					legacyResponse.Paths[j] = domain.Path{
						Hops: make([]domain.Hop, len(bulkPath.Hops)),
					}
					for k, bulkHop := range bulkPath.Hops {
						legacyResponse.Paths[j].Hops[k] = domain.Hop{
							Device:    bulkHop.DeviceName,
							Interface: bulkHop.IngressInterface,
							Action:    bulkHop.DeviceType,
						}
					}
				}
				if trackErr := s.apiTracker.TrackPathSearch(networkID, query.SrcIP, query.DstIP, legacyResponse); trackErr != nil {
					s.logger.Debug("Failed to track bulk path search %d in memory system: %v", i+1, trackErr)
				}
			}
		}
	}

	// Build summary
	totalPaths := 0
	successfulQueries := 0
	var errors []string

	s.logger.Debug("Processing %d bulk path search responses", len(responses))
	for i, response := range responses {
		// For bulk responses, paths are in response.Info.Paths
		pathCount := len(response.Info.Paths)
		s.logger.Debug("Response %d: Paths=%d, DstIpLocationType=%s, TimedOut=%v",
			i+1, pathCount, response.DstIpLocationType, response.TimedOut)

		if pathCount > 0 {
			totalPaths += pathCount
			successfulQueries++
			s.logger.Debug("Response %d: Found %d paths", i+1, pathCount)
		} else {
			errors = append(errors, fmt.Sprintf("Query %d: No paths found", i+1))
			s.logger.Debug("Response %d: No paths found", i+1)
		}
	}

	// Enhanced response with debugging info
	debugInfo := ""
	if len(errors) > 0 {
		debugInfo += fmt.Sprintf("\n⚠️  Warnings: %d queries had issues\n", len(errors))
		for _, err := range errors {
			debugInfo += fmt.Sprintf("  - %s\n", err)
		}
	}

	// Check for missing "from" property usage
	missingFromCount := 0
	for _, query := range args.Queries {
		if query.From == "" {
			missingFromCount++
		}
	}
	if missingFromCount > 0 {
		debugInfo += fmt.Sprintf("\n💡 Tip: %d queries don't use the 'from' property. Consider adding it for more accurate results.\n", missingFromCount)
	}

	result := MarshalCompactJSONString(responses)

	return textResult(fmt.Sprintf("Bulk path search completed. %d/%d queries successful, found %d total paths:%s\n%s",
		successfulQueries, len(args.Queries), totalPaths, debugInfo, result)), nil
}

func (s *Service) PathSearchWorkflow(args PathSearchWorkflowArgs) (*Result, error) {
	sessionID := fmt.Sprintf("path_session_%v", args.SessionID)
	state := s.workflowManager.GetState(sessionID)

	switch state.CurrentStep {
	case "start":
		return s.startPathSearchWorkflow(sessionID)
	case "explain_best_practices":
		return s.explainPathSearchBestPractices(sessionID)
	case "show_bulk_example":
		return s.showBulkPathSearchExample(sessionID)
	case "guide_request_building":
		return s.guidePathSearchRequestBuilding(sessionID)
	case "network_scope_discovery":
		return s.guideNetworkScopeDiscovery(sessionID)
	default:
		return s.startPathSearchWorkflow(sessionID)
	}
}

func (s *Service) startPathSearchWorkflow(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "explain_best_practices",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `🚀 **Welcome to the Path Search Workflow!**

This workflow will guide you through effective path search using Forward Networks best practices.

**Key Principles:**
1. **Always use 'from' property** - Specify the source device for more accurate results
2. **Use bulk operations** - For multiple paths, use search_paths_bulk for better performance
3. **Set appropriate limits** - Control response size with max_results and max_candidates
4. **Choose the right intent** - PREFER_DELIVERED, PREFER_VIOLATIONS, or VIOLATIONS_ONLY

**Next Steps:**
- Learn about best practices for path search
- See examples of bulk path search requests
- Get guidance on building effective requests

Would you like to continue with the best practices explanation?`

	return textResult(content), nil
}

func (s *Service) explainPathSearchBestPractices(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "show_bulk_example",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `📋 **Path Search Best Practices**

**1. Use the 'from' Property (CRITICAL)**
- Always specify the source device using 'from' property
- This provides more accurate results than src_ip alone
- Example: "from": "router-01", "src_ip": "10.0.1.1"

**2. Choose the Right Tool**
- **Single path**: Use search_paths for one-off analysis
- **Multiple paths**: Use search_paths_bulk for concurrent execution

**3. Set Appropriate Intent**
- PREFER_DELIVERED: Find paths where traffic reaches destination
- PREFER_VIOLATIONS: Find paths with drops, blackholes, loops
- VIOLATIONS_ONLY: Only find problematic paths

**4. Control Response Size**
- max_results: Limit returned paths (default: 1)
- max_candidates: Limit computed candidates (default: 5000)
- max_seconds: Per-query timeout (default: 30s)

**5. Performance Tips**
- Use bulk operations for multiple queries
- Set reasonable timeouts
- Include network functions only when needed

Would you like to see a bulk path search example?`

	return textResult(content), nil
}

func (s *Service) showBulkPathSearchExample(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "guide_request_building",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `💡 **Bulk Path Search Example**

Here's how to structure a bulk path search request:

{
  "network_id": "your-network-id",
  "queries": [
    {
      "from": "router-01",
      "src_ip": "10.0.1.1",
      "dst_ip": "10.0.2.1",
      "src_port": "80",
      "dst_port": "443"
    },
    {
      "from": "switch-01", 
      "src_ip": "10.0.1.2",
      "dst_ip": "10.0.3.1"
    },
    {
      "from": "firewall-01",
      "src_ip": "192.168.1.1",
      "dst_ip": "8.8.8.8"
    }
  ],
  "intent": "PREFER_DELIVERED",
  "max_results": 5,
  "max_candidates": 1000,
  "max_seconds": 30
}

**Key Points:**
- Each query in the array uses the 'from' property
- All queries run concurrently for better performance
- Common parameters (intent, limits) apply to all queries
- Individual queries can override common parameters

Would you like guidance on building your own requests?`

	return textResult(content), nil
}

func (s *Service) guidePathSearchRequestBuilding(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "network_scope_discovery",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `🎯 **Building Effective Path Search Requests**

**Step 1: Choose Your Tool**
- Single path: search_paths
- Multiple paths: search_paths_bulk

**Step 2: Gather Required Information**
- Network ID (use list_networks to find)
- Source device name (use list_devices to find)
- Source IP address
- Destination IP address/subnet

**Step 3: Build Your Request**
{
  "network_id": "your-network-id",
  "from": "device-name",           // ALWAYS include this
  "src_ip": "source-ip",           // Combine with 'from'
  "dst_ip": "destination-ip",      // Required
  "intent": "PREFER_DELIVERED",    // Choose appropriate intent
  "max_results": 5                 // Control response size
}

**Step 4: For Bulk Requests**
- Create an array of queries
- Each query follows the same structure
- Set common parameters at the top level

**Common Mistakes to Avoid:**
❌ Not using the 'from' property
❌ Using single path search for multiple queries
❌ Not setting appropriate limits
❌ Using wrong intent for your use case

**Next: Discover Network Scopes for Better Planning**
Would you like to learn how to discover network scopes and locations for more effective path planning?`

	return textResult(content), nil
}

func (s *Service) guideNetworkScopeDiscovery(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "complete",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	content := `🌐 **Network Scope Discovery for Path Planning**

**Why Discover Network Scopes?**
Before planning complex path searches, it's valuable to understand your network's structure:
- **Location-based analysis**: Identify network scopes in each site/location
- **Aggregation opportunities**: Find /8, /16, /24 prefixes that can be tested together
- **Connectivity planning**: Understand which locations can reach each other
- **Efficient path testing**: Test connectivity between aggregated prefixes instead of individual IPs

**What Network Scope Discovery Does:**
1. **Discovers all network prefixes** in your network by location
2. **Identifies aggregation levels** (/8, /16, /24 for IPv4; /32, /48, /64 for IPv6)
3. **Maps devices to prefixes** and locations
4. **Tests connectivity** between different locations and aggregation levels
5. **Generates comprehensive reports** with insights and recommendations

**Example Output:**

Location: ATL-DC01
- /8: 10.0.0.0/8 (15 devices)
- /16: 10.110.0.0/16 (8 devices)
- /24: 10.110.37.0/24 (3 devices)

Location: SJC-DC01
- /8: 10.0.0.0/8 (12 devices)
- /16: 10.117.0.0/16 (6 devices)

Connectivity: ATL-DC01 ↔ SJC-DC01 ✅ CONNECTED

**How to Use:**
- Run analyze_network_prefixes to discover your network structure
- Use the discovered prefixes in your path search requests
- Test connectivity between locations at different aggregation levels

**Benefits for Path Search:**
- **Smarter planning**: Know which prefixes to test
- **Efficient testing**: Test aggregated prefixes instead of individual IPs
- **Location awareness**: Understand site-to-site connectivity
- **Better results**: Focus on meaningful network segments

**This completes the Path Search Workflow!** 🚀

You now have the tools and knowledge to:
1. ✅ Use best practices for path search
2. ✅ Build effective bulk path search requests
3. ✅ Discover network scopes for better planning
4. ✅ Execute comprehensive path analysis

Ready to start analyzing your network!`

	return textResult(content), nil
}

// Single path search entry point - converts to bulk format
func (s *Service) SearchPathsEntry(ctx context.Context, args SearchPathsArgs) (*Result, error) {
	// Convert single path search to bulk format
	bulkArgs := SearchPathsBulkArgs{
		NetworkID:               args.NetworkID,
		SnapshotID:              args.SnapshotID,
		Intent:                  args.Intent,
		MaxCandidates:           args.MaxCandidates,
		MaxResults:              args.MaxResults,
		MaxReturnPathResults:    args.MaxReturnPathResults,
		MaxSeconds:              args.MaxSeconds,
		IncludeNetworkFunctions: args.IncludeNetworkFunctions,
		Queries: []PathSearchQueryArgs{
			{
				From:    args.From,
				SrcIP:   args.SrcIP,
				DstIP:   args.DstIP,
				IPProto: args.IPProto,
				SrcPort: args.SrcPort,
				DstPort: args.DstPort,
			},
		},
	}
	return s.searchPathsBulk(ctx, bulkArgs)
}

// Update the searchPathsBulk entrypoint to route single queries to searchPaths
func (s *Service) SearchPathsBulkEntry(ctx context.Context, args SearchPathsBulkArgs) (*Result, error) {
	return s.searchPathsBulk(ctx, args)
}

// Network Prefix Discovery and Analysis Methods
