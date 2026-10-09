package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/forward-mcp/internal/domain"
)

// Network Observability Tool Implementations
func (s *Service) ListNetworks(ctx context.Context, args ListNetworksArgs) (*Result, error) {
	s.logToolCall("list_networks", args, nil)

	// Get all networks from API
	allNetworks, err := s.forwardClient.GetNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Apply pagination with safe defaults to prevent token overflow
	limit := args.Limit
	if limit <= 0 {
		limit = 25 // Conservative default limit to prevent token overflow
	}
	if limit > 100 {
		limit = 100 // Cap at 100 to prevent excessive responses
	}
	offset := args.Offset
	if offset < 0 {
		offset = 0
	}

	var networks []domain.Network
	var totalCount int
	var hasMore bool

	if args.AllResults {
		// Store all networks in memory system for large datasets
		networks = allNetworks
		totalCount = len(allNetworks)
		hasMore = false

		// Store in memory system if available
		if s.memorySystem != nil {
			entity, err := s.memorySystem.CreateEntity("network_list", "query_result", map[string]interface{}{
				"query_type":  "list_networks",
				"total_count": totalCount,
				"timestamp":   time.Now().Unix(),
			})
			if err == nil {
				// Store the networks data
				networksJSON, _ := json.Marshal(networks)
				s.memorySystem.AddObservation(entity.ID, string(networksJSON), "data", map[string]interface{}{
					"data_type": "networks_list",
					"count":     totalCount,
				})
			}
		}
	} else {
		// Apply pagination
		totalCount = len(allNetworks)
		start := offset
		end := offset + limit
		if start >= totalCount {
			networks = []domain.Network{}
		} else {
			if end > totalCount {
				end = totalCount
			}
			networks = allNetworks[start:end]
		}
		hasMore = offset+len(networks) < totalCount
	}

	// Build response
	var responseText strings.Builder
	responseText.WriteString(fmt.Sprintf("Found %d networks", totalCount))
	if !args.AllResults {
		responseText.WriteString(fmt.Sprintf(" (showing %d-%d)", offset+1, offset+len(networks)))
		if hasMore {
			responseText.WriteString(fmt.Sprintf(", %d more available", totalCount-offset-len(networks)))
		}
		if args.Limit <= 0 {
			responseText.WriteString(" [Note: Using default limit of 25 to prevent token overflow. Use 'limit' parameter to adjust.]")
		}
	}
	responseText.WriteString(":\n")

	if len(networks) > 0 {
		result := MarshalCompactJSONString(networks)
		responseText.WriteString(result)
	} else {
		responseText.WriteString("No networks found.")
	}

	if args.AllResults && s.memorySystem != nil {
		responseText.WriteString(fmt.Sprintf("\n\n💾 Stored %d networks in memory system for future reference.", totalCount))
	}

	return textResult(responseText.String()), nil
}

func (s *Service) CreateNetwork(ctx context.Context, args CreateNetworkArgs) (*Result, error) {
	s.logToolCall("create_network", args, nil)

	// Validate required fields
	if err := s.validateNonEmpty("network name", args.Name); err != nil {
		return nil, err
	}

	network, err := s.forwardClient.CreateNetwork(ctx, args.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return textResult(fmt.Sprintf("Network created successfully:\n%s", string(result))), nil
}

func (s *Service) deleteNetwork(ctx context.Context, args DeleteNetworkArgs) (*Result, error) {
	s.logToolCall("delete_network", args, nil)

	// Validate required fields
	if err := s.validateNonEmpty("network_id", args.NetworkID); err != nil {
		return nil, err
	}

	network, err := s.forwardClient.DeleteNetwork(ctx, args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return textResult(fmt.Sprintf("Network deleted successfully:\n%s", string(result))), nil
}

func (s *Service) UpdateNetwork(ctx context.Context, args UpdateNetworkArgs) (*Result, error) {
	s.logToolCall("update_network", args, nil)

	// Validate required fields
	if err := s.validateNonEmpty("network_id", args.NetworkID); err != nil {
		return nil, err
	}

	update := &domain.NetworkUpdate{}
	if args.Name != "" {
		update.Name = &args.Name
	}
	if args.Description != "" {
		update.Description = &args.Description
	}

	network, err := s.forwardClient.UpdateNetwork(ctx, args.NetworkID, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return textResult(fmt.Sprintf("Network updated successfully:\n%s", string(result))), nil
}

// Path Search Tool Implementations

// resolveNetworkIDByName resolves a network name to its networkId using a case-insensitive match.
func (s *Service) resolveNetworkIDByName(ctx context.Context, name string) (string, error) {
	networks, err := s.forwardClient.GetNetworks(ctx)
	if err != nil {
		return "", err
	}
	var matches []domain.Network
	for _, n := range networks {
		if strings.EqualFold(n.Name, name) {
			matches = append(matches, n)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	} else if len(matches) > 1 {
		return "", fmt.Errorf("multiple networks found with the name '%s'", name)
	}
	return "", fmt.Errorf("no network found with the name '%s'", name)
}

// First-Class Query Tool Implementations - Critical Network Operations
// These wrap the most important predefined queries as dedicated tools

func (s *Service) GetDefaultSettings(ctx context.Context, args GetDefaultSettingsArgs) (*Result, error) {
	s.logToolCall("get_default_settings", args, nil)

	// Get network name if possible
	networkName := "Not set"
	if s.defaults.NetworkID != "" {
		networks, err := s.forwardClient.GetNetworks(ctx)
		if err == nil {
			for _, network := range networks {
				if network.ID == s.defaults.NetworkID {
					networkName = fmt.Sprintf("%s (%s)", network.Name, network.ID)
					break
				}
			}
		}
	}

	settings := map[string]interface{}{
		"default_network_id":   s.defaults.NetworkID,
		"default_network_name": networkName,
		"default_snapshot_id":  s.defaults.SnapshotID,
		"default_query_limit":  s.defaults.QueryLimit,
		"environment_source":   "Loaded from environment variables and config files",
	}

	result := MarshalCompactJSONString(settings)

	response := fmt.Sprintf("Current default settings:\n%s\n\n", result)
	response += "To change defaults:\n"
	response += "• Use set_default_network to change the default network\n"
	response += "• Update environment variables (FORWARD_DEFAULT_NETWORK_ID, etc.)\n"
	response += "• Modify your .env file or config.json\n\n"

	if s.defaults.NetworkID == "" {
		response += " No default network is set. Consider setting FORWARD_DEFAULT_NETWORK_ID in your environment."
	}

	return textResult(response), nil
}

func (s *Service) SetDefaultNetwork(ctx context.Context, args SetDefaultNetworkArgs) (*Result, error) {
	s.logToolCall("set_default_network", args, nil)

	var networkID string
	var networkName string

	// Try to resolve the network identifier (could be ID or name)
	if args.NetworkIdentifier == "" {
		return textResult("Please provide either a network ID or network name."), nil
	}

	// First, try as network ID by listing networks and checking if it exists
	networks, err := s.forwardClient.GetNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	// Check if it's a direct network ID match
	for _, network := range networks {
		if network.ID == args.NetworkIdentifier {
			networkID = network.ID
			networkName = network.Name
			break
		}
	}

	// If not found as ID, try to resolve as name
	if networkID == "" {
		resolvedID, err := s.resolveNetworkIDByName(ctx, args.NetworkIdentifier)
		if err != nil {
			// List available networks for user reference
			availableNetworks := "Available networks:\n"
			for i, network := range networks {
				availableNetworks += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, network.Name, network.ID)
			}

			return textResult(fmt.Sprintf("Network '%s' not found.\n\n%s\nPlease use either a valid network ID or exact network name.", args.NetworkIdentifier, availableNetworks)), nil
		}

		networkID = resolvedID
		// Find the network name
		for _, network := range networks {
			if network.ID == networkID {
				networkName = network.Name
				break
			}
		}
	}

	// Update the default (for this session)
	s.defaults.NetworkID = networkID

	response := "Default network updated successfully!\n\n"
	response += fmt.Sprintf("New default: %s (ID: %s)\n\n", networkName, networkID)
	response += "This change applies to the current session. To make it permanent:\n"
	response += fmt.Sprintf("• Set FORWARD_DEFAULT_NETWORK_ID=%s in your environment\n", networkID)
	response += "• Or update your .env file or config.json\n\n"
	response += "All subsequent tool calls will now use this network by default when network_id is not specified."

	return textResult(response), nil
}

// Semantic Cache and AI Enhancement Tool Implementations
