package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/forward-mcp/internal/domain"
)

// Device Management Tool Implementations
func (s *Service) ListDevices(ctx context.Context, args ListDevicesArgs) (*Result, error) {
	s.logToolCall("list_devices", args, nil)

	// Apply default limit if not specified
	limit := args.Limit
	if limit == 0 {
		limit = s.getQueryLimit(0)
	}

	params := &domain.DeviceQueryParams{
		SnapshotID: args.SnapshotID,
		Limit:      limit,
		Offset:     args.Offset,
	}

	response, err := s.forwardClient.GetDevices(ctx, args.NetworkID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	// Track device discovery in memory system
	if s.apiTracker != nil {
		if trackErr := s.apiTracker.TrackDeviceDiscovery(args.NetworkID, response.Devices); trackErr != nil {
			s.logger.Debug("Failed to track device discovery in memory system: %v", trackErr)
		}
	}

	result := MarshalCompactJSONString(response)
	return textResult(fmt.Sprintf("Found %d devices (total: %d):\n%s", len(response.Devices), response.TotalCount, result)), nil
}

func (s *Service) GetDeviceLocations(ctx context.Context, args GetDeviceLocationsArgs) (*Result, error) {
	s.logToolCall("get_device_locations", args, nil)

	// Get all device locations from API
	allLocations, err := s.forwardClient.GetDeviceLocations(ctx, args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device locations: %w", err)
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

	var locations map[string]string
	var totalCount int
	var hasMore bool

	if args.AllResults {
		// Store all device locations in memory system for large datasets
		locations = allLocations
		totalCount = len(allLocations)
		hasMore = false

		// Store in memory system if available
		if s.memorySystem != nil {
			entity, err := s.memorySystem.CreateEntity("device_locations", "query_result", map[string]interface{}{
				"query_type":  "get_device_locations",
				"network_id":  args.NetworkID,
				"total_count": totalCount,
				"timestamp":   time.Now().Unix(),
			})
			if err == nil {
				// Store the device locations data
				locationsJSON, _ := json.Marshal(locations)
				s.memorySystem.AddObservation(entity.ID, string(locationsJSON), "data", map[string]interface{}{
					"data_type": "device_locations",
					"count":     totalCount,
				})
			}
		}
	} else {
		// Apply pagination to map
		totalCount = len(allLocations)
		start := offset
		end := offset + limit
		if start >= totalCount {
			locations = make(map[string]string)
		} else {
			locations = make(map[string]string)
			keys := make([]string, 0, len(allLocations))
			for k := range allLocations {
				keys = append(keys, k)
			}
			if end > totalCount {
				end = totalCount
			}
			for i := start; i < end; i++ {
				key := keys[i]
				locations[key] = allLocations[key]
			}
		}
		hasMore = offset+len(locations) < totalCount
	}

	// Build response
	var responseText strings.Builder
	responseText.WriteString(fmt.Sprintf("Found %d device locations", totalCount))
	if !args.AllResults {
		responseText.WriteString(fmt.Sprintf(" (showing %d-%d)", offset+1, offset+len(locations)))
		if hasMore {
			responseText.WriteString(fmt.Sprintf(", %d more available", totalCount-offset-len(locations)))
		}
	}
	responseText.WriteString(":\n")

	if len(locations) > 0 {
		result, _ := json.MarshalIndent(locations, "", "  ")
		responseText.WriteString(string(result))
	} else {
		responseText.WriteString("No device locations found.")
	}

	if args.AllResults && s.memorySystem != nil {
		responseText.WriteString(fmt.Sprintf("\n\n💾 Stored %d device locations in memory system for future reference.", totalCount))
	}

	return textResult(responseText.String()), nil
}

// Snapshot Management Tool Implementations
func (s *Service) ListSnapshots(ctx context.Context, args ListSnapshotsArgs) (*Result, error) {
	s.logToolCall("list_snapshots", args, nil)

	// Get all snapshots from API
	allSnapshots, err := s.forwardClient.GetSnapshots(ctx, args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
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

	var snapshots []domain.Snapshot
	var totalCount int
	var hasMore bool

	if args.AllResults {
		// Store all snapshots in memory system for large datasets
		snapshots = allSnapshots
		totalCount = len(allSnapshots)
		hasMore = false

		// Store in memory system if available
		if s.memorySystem != nil {
			entity, err := s.memorySystem.CreateEntity("snapshot_list", "query_result", map[string]interface{}{
				"query_type":  "list_snapshots",
				"network_id":  args.NetworkID,
				"total_count": totalCount,
				"timestamp":   time.Now().Unix(),
			})
			if err == nil {
				// Store the snapshots data
				snapshotsJSON, _ := json.Marshal(snapshots)
				s.memorySystem.AddObservation(entity.ID, string(snapshotsJSON), "data", map[string]interface{}{
					"data_type": "snapshots_list",
					"count":     totalCount,
				})
			}
		}
	} else {
		// Apply pagination
		totalCount = len(allSnapshots)
		start := offset
		end := offset + limit
		if start >= totalCount {
			snapshots = []domain.Snapshot{}
		} else {
			if end > totalCount {
				end = totalCount
			}
			snapshots = allSnapshots[start:end]
		}
		hasMore = offset+len(snapshots) < totalCount
	}

	// Build response
	var responseText strings.Builder
	responseText.WriteString(fmt.Sprintf("Found %d snapshots", totalCount))
	if !args.AllResults {
		responseText.WriteString(fmt.Sprintf(" (showing %d-%d)", offset+1, offset+len(snapshots)))
		if hasMore {
			responseText.WriteString(fmt.Sprintf(", %d more available", totalCount-offset-len(snapshots)))
		}
		if args.Limit <= 0 {
			responseText.WriteString(" [Note: Using default limit of 25 to prevent token overflow. Use 'limit' parameter to adjust.]")
		}
	}
	responseText.WriteString(":\n")

	if len(snapshots) > 0 {
		result, _ := json.MarshalIndent(snapshots, "", "  ")
		responseText.WriteString(string(result))
	} else {
		responseText.WriteString("No snapshots found.")
	}

	if args.AllResults && s.memorySystem != nil {
		responseText.WriteString(fmt.Sprintf("\n\n💾 Stored %d snapshots in memory system for future reference.", totalCount))
	}

	return textResult(responseText.String()), nil
}

func (s *Service) GetLatestSnapshot(ctx context.Context, args GetLatestSnapshotArgs) (*Result, error) {
	s.logToolCall("get_latest_snapshot", args, nil)
	snapshot, err := s.forwardClient.GetLatestSnapshot(ctx, args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	result, _ := json.MarshalIndent(snapshot, "", "  ")
	return textResult(fmt.Sprintf("Latest snapshot:\n%s", string(result))), nil
}

// Location Management Tool Implementations
func (s *Service) ListLocations(ctx context.Context, args ListLocationsArgs) (*Result, error) {
	s.logToolCall("list_locations", args, nil)

	// Get all locations from API
	allLocations, err := s.forwardClient.GetLocations(ctx, args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
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

	var locations []domain.Location
	var totalCount int
	var hasMore bool

	if args.AllResults {
		// Store all locations in memory system for large datasets
		locations = allLocations
		totalCount = len(allLocations)
		hasMore = false

		// Store in memory system if available
		if s.memorySystem != nil {
			entity, err := s.memorySystem.CreateEntity("location_list", "query_result", map[string]interface{}{
				"query_type":  "list_locations",
				"network_id":  args.NetworkID,
				"total_count": totalCount,
				"timestamp":   time.Now().Unix(),
			})
			if err == nil {
				// Store the locations data
				locationsJSON, _ := json.Marshal(locations)
				s.memorySystem.AddObservation(entity.ID, string(locationsJSON), "data", map[string]interface{}{
					"data_type": "locations_list",
					"count":     totalCount,
				})
			}
		}
	} else {
		// Apply pagination
		totalCount = len(allLocations)
		start := offset
		end := offset + limit
		if start >= totalCount {
			locations = []domain.Location{}
		} else {
			if end > totalCount {
				end = totalCount
			}
			locations = allLocations[start:end]
		}
		hasMore = offset+len(locations) < totalCount
	}

	// Build response
	var responseText strings.Builder
	responseText.WriteString(fmt.Sprintf("Found %d locations", totalCount))
	if !args.AllResults {
		responseText.WriteString(fmt.Sprintf(" (showing %d-%d)", offset+1, offset+len(locations)))
		if hasMore {
			responseText.WriteString(fmt.Sprintf(", %d more available", totalCount-offset-len(locations)))
		}
		if args.Limit <= 0 {
			responseText.WriteString(" [Note: Using default limit of 25 to prevent token overflow. Use 'limit' parameter to adjust.]")
		}
	}
	responseText.WriteString(":\n")

	if len(locations) > 0 {
		result, _ := json.MarshalIndent(locations, "", "  ")
		responseText.WriteString(string(result))
	} else {
		responseText.WriteString("No locations found.")
	}

	if args.AllResults && s.memorySystem != nil {
		responseText.WriteString(fmt.Sprintf("\n\n💾 Stored %d locations in memory system for future reference.", totalCount))
	}

	return textResult(responseText.String()), nil
}

func (s *Service) CreateLocation(ctx context.Context, args CreateLocationArgs) (*Result, error) {
	s.logToolCall("create_location", args, nil)

	// Log the received parameters for debugging
	s.logger.Info("Creating location with parameters: network_id=%s, name=%s, lat=%f, lng=%f, city=%s, adminDivision=%s, country=%s",
		args.NetworkID, args.Name, args.Lat, args.Lng, args.City, args.AdminDivision, args.Country)

	// Check if lat/lng are zero values (which might indicate they weren't provided)
	if args.Lat == 0 && args.Lng == 0 {
		s.logger.Warn("Latitude and longitude are both zero - this might indicate missing parameters")
	}

	// Validate latitude is within valid range (-90 to +90)
	if args.Lat < -90 || args.Lat > 90 {
		return nil, fmt.Errorf("latitude must be between -90 and +90 degrees, got: %f", args.Lat)
	}

	// Validate longitude is within valid range (-180 to +180)
	if args.Lng < -180 || args.Lng > 180 {
		return nil, fmt.Errorf("longitude must be between -180 and +180 degrees, got: %f", args.Lng)
	}

	location := &domain.LocationCreate{
		ID:            args.ID,
		Name:          args.Name,
		Lat:           args.Lat,
		Lng:           args.Lng,
		City:          args.City,
		AdminDivision: args.AdminDivision,
		Country:       args.Country,
	}

	// Log the location object being sent to the API
	locationJSON, _ := json.Marshal(location)
	s.logger.Info("Sending location to API: %s", string(locationJSON))

	newLocation, err := s.forwardClient.CreateLocation(ctx, args.NetworkID, location)
	if err != nil {
		s.logger.Error("Failed to create location: error=%v, network_id=%s", err, args.NetworkID)
		return nil, fmt.Errorf("failed to create location: %w", err)
	}

	result, _ := json.MarshalIndent(newLocation, "", "  ")
	return textResult(fmt.Sprintf("Location created successfully:\n%s", string(result))), nil
}

func (s *Service) CreateLocationsBulk(ctx context.Context, args CreateLocationsBulkArgs) (*Result, error) {
	s.logToolCall("create_locations_bulk", args, nil)

	if len(args.Locations) == 0 {
		return nil, fmt.Errorf("at least one location must be provided")
	}

	// Validate inputs and transform to domain.LocationBulkPatch slice as required by PATCH
	locations := make([]domain.LocationBulkPatch, 0, len(args.Locations))
	for i, item := range args.Locations {
		// For PATCH: either ID or Name must be provided
		if item.ID == "" && item.Name == "" {
			return nil, fmt.Errorf("location[%d] must have either id (for update) or name (for create)", i)
		}

		// Validate coordinates if provided
		if item.Lat != nil && (*item.Lat < -90 || *item.Lat > 90) {
			return nil, fmt.Errorf("location[%d] latitude must be between -90 and +90 degrees, got: %f", i, *item.Lat)
		}
		if item.Lng != nil && (*item.Lng < -180 || *item.Lng > 180) {
			return nil, fmt.Errorf("location[%d] longitude must be between -180 and +180 degrees, got: %f", i, *item.Lng)
		}

		// Build domain.LocationBulkPatch (PATCH expects partial objects)
		loc := domain.LocationBulkPatch{
			ID:            item.ID,
			Name:          item.Name,
			Lat:           item.Lat,
			Lng:           item.Lng,
			City:          item.City,
			AdminDivision: item.AdminDivision,
			Country:       item.Country,
		}
		locations = append(locations, loc)
	}

	// Execute bulk patch (create/update)
	err := s.forwardClient.CreateLocationsBulk(ctx, args.NetworkID, locations)
	if err != nil {
		return nil, fmt.Errorf("failed to patch locations in bulk: %w", err)
	}

	return textResult("Bulk locations patched successfully (204 No Content)."), nil
}

func (s *Service) UpdateLocation(ctx context.Context, args UpdateLocationArgs) (*Result, error) {
	s.logToolCall("update_location", args, nil)
	update := &domain.LocationUpdate{
		Name:          &args.Name,
		Lat:           args.Lat,
		Lng:           args.Lng,
		City:          &args.City,
		AdminDivision: &args.AdminDivision,
		Country:       &args.Country,
	}

	updatedLocation, err := s.forwardClient.UpdateLocation(ctx, args.NetworkID, args.LocationID, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	result, _ := json.MarshalIndent(updatedLocation, "", "  ")
	return textResult(fmt.Sprintf("Location updated successfully:\n%s", string(result))), nil
}

func (s *Service) DeleteLocation(ctx context.Context, args DeleteLocationArgs) (*Result, error) {
	s.logToolCall("delete_location", args, nil)
	deletedLocation, err := s.forwardClient.DeleteLocation(ctx, args.NetworkID, args.LocationID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete location: %w", err)
	}

	result, _ := json.MarshalIndent(deletedLocation, "", "  ")
	return textResult(fmt.Sprintf("Location deleted successfully:\n%s", string(result))), nil
}

func (s *Service) DeleteSnapshot(ctx context.Context, args DeleteSnapshotArgs) (*Result, error) {
	s.logToolCall("delete_snapshot", args, nil)
	err := s.forwardClient.DeleteSnapshot(ctx, args.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete snapshot: %w", err)
	}

	return textResult(fmt.Sprintf("Snapshot %s deleted successfully", args.SnapshotID)), nil
}

// isCloudDevice checks if a device name suggests it's a cloud device
func (s *Service) isCloudDevice(deviceName string) bool {
	deviceNameLower := strings.ToLower(deviceName)

	// Debug logging for troubleshooting
	// fmt.Printf("DEBUG: Checking device '%s' (lowercase: '%s')\n", deviceName, deviceNameLower)

	// Explicit cloud device patterns (exact matches or specific patterns)
	exactCloudPatterns := []string{
		"csr1kv", "csr-1kv", "csr_1kv",
		"pan-fw", "pan_fw", "panfw",
		"aws-", "azure-", "gcp-",
		"virtual-", "vm-", "container-",
	}

	// Check for exact cloud device patterns
	for _, pattern := range exactCloudPatterns {
		if strings.Contains(deviceNameLower, pattern) {
			return true
		}
	}

	// Check for cloud-specific naming patterns (more restrictive)
	cloudPatterns := []string{
		"aws-", "azure-", "gcp-", "cloud-", // Cloud provider prefixes
		"virtual-", "vm-", "container-", // Virtualization prefixes
		"-aws", "-azure", "-gcp", // Cloud provider suffixes (but not -cloud)
		"-virtual", "-vm", "-container", // Virtualization suffixes
	}

	for _, pattern := range cloudPatterns {
		if strings.Contains(deviceNameLower, pattern) {
			return true
		}
	}

	// Check for standalone cloud keywords (but not as part of infrastructure names)
	// This is more restrictive - only match if the keyword appears in a cloud context
	// Note: "cloud" and "kvm" are handled separately below
	standaloneCloudKeywords := []string{
		"aws", "azure", "gcp", "virtual", "vm", "container",
	}

	for _, keyword := range standaloneCloudKeywords {
		// Only match if the keyword appears as a standalone word or with cloud-specific prefixes
		if strings.Contains(deviceNameLower, keyword) {
			// Check if it's part of a cloud-specific pattern
			if strings.HasPrefix(deviceNameLower, keyword+"-") ||
				strings.HasSuffix(deviceNameLower, "-"+keyword) ||
				strings.Contains(deviceNameLower, "-"+keyword+"-") {
				return true
			}
		}
	}

	// Special case: "cloud" keyword - only match if it's clearly a cloud device
	// Don't match infrastructure devices that happen to contain "cloud" in their name
	if strings.Contains(deviceNameLower, "cloud") {
		// Only match if it's a clear cloud device pattern
		cloudSpecificPatterns := []string{
			"cloud-", "-cloud-", // Cloud prefixes and middle patterns
			"aws-cloud", "azure-cloud", "gcp-cloud",
			"cloud-aws", "cloud-azure", "cloud-gcp",
		}

		for _, pattern := range cloudSpecificPatterns {
			if strings.Contains(deviceNameLower, pattern) {
				return true
			}
		}

		// Don't match infrastructure devices like "fel-wps1-cloud2s02" or "fel-wps1-cloudm3s01"
		// These are physical devices with "cloud" in their infrastructure role name
		return false
	}

	// Special case: "kvm" keyword - only match if it's clearly a virtual device
	// Don't match physical KVM devices like "fel-wps1-kvmd2s01"
	if strings.Contains(deviceNameLower, "kvm") {
		// Only match if it's a clear virtual device pattern
		kvmVirtualPatterns := []string{
			"kvm-", "-kvm-", // KVM prefixes and middle patterns
			"virtual-kvm", "vm-kvm", "container-kvm",
			"kvm-virtual", "kvm-vm", "kvm-container",
		}

		for _, pattern := range kvmVirtualPatterns {
			if strings.Contains(deviceNameLower, pattern) {
				return true
			}
		}

		// Don't match physical KVM devices like "fel-wps1-kvmd2s01"
		// These are physical devices with "kvm" in their infrastructure role name
		// Only match if it's clearly a virtual device (not just ending with -kvm)
		return false
	}

	return false
}

func (s *Service) UpdateDeviceLocations(ctx context.Context, args UpdateDeviceLocationsArgs) (*Result, error) {
	s.logToolCall("update_device_locations", args, nil)

	// Log the devices being moved for debugging
	s.logger.Info("Updating device locations for %d devices: %v", len(args.Locations), args.Locations)

	// Pre-validate for cloud devices
	var cloudDevices []string
	var physicalDevices = make(map[string]string)

	for deviceName, locationID := range args.Locations {
		if s.isCloudDevice(deviceName) {
			cloudDevices = append(cloudDevices, deviceName)
			s.logger.Warn("Detected cloud device in location update: %s", deviceName)
		} else {
			physicalDevices[deviceName] = locationID
		}
	}

	// If we found cloud devices, provide a helpful warning
	if len(cloudDevices) > 0 {
		s.logger.Warn("Cloud devices detected: %v. These will be excluded from the location update.", cloudDevices)

		// If all devices are cloud devices, return an error
		if len(physicalDevices) == 0 {
			return nil, fmt.Errorf("all devices are cloud devices and cannot be moved to physical locations: %v\n\nNote: Cloud devices (CSR1KV, PAN-FW, etc.) cannot be moved to physical locations. Please use only physical devices for location assignments.", cloudDevices)
		}

		// Update only physical devices
		args.Locations = physicalDevices
		s.logger.Info("Proceeding with %d physical devices, excluding %d cloud devices", len(physicalDevices), len(cloudDevices))
	}

	err := s.forwardClient.UpdateDeviceLocations(ctx, args.NetworkID, args.Locations)
	if err != nil {
		// Check if this is a cloud device error
		if strings.Contains(err.Error(), "Unrecognized devices cannot be moved") {
			// Extract device names from the error message
			errorMsg := err.Error()
			s.logger.Warn("Cloud devices detected in location update request: %s", errorMsg)

			// Provide a more helpful error message
			return nil, fmt.Errorf("failed to update device locations: %w\n\nNote: Cloud devices (like CSR1KV, PAN-FW, etc.) cannot be moved to physical locations. Please exclude cloud devices from the location assignment.", err)
		}

		s.logger.Error("Failed to update device locations: error=%v, network_id=%s", err, args.NetworkID)
		return nil, fmt.Errorf("failed to update device locations: %w", err)
	}

	// Build success message
	successMsg := fmt.Sprintf("Updated locations for %d devices", len(args.Locations))
	if len(cloudDevices) > 0 {
		successMsg += fmt.Sprintf("\nNote: %d cloud devices were excluded: %v", len(cloudDevices), cloudDevices)
	}

	return textResult(successMsg), nil
}

func (s *Service) GetDeviceBasicInfo(ctx context.Context, args GetDeviceBasicInfoArgs) (*Result, error) {
	s.logToolCall("get_device_basic_info", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029", // Device Basic Info
		Options:    args.Options,
	}

	return s.RunNQEQueryByID(ctx, queryArgs)
}

func (s *Service) GetDeviceHardware(ctx context.Context, args GetDeviceHardwareArgs) (*Result, error) {
	s.logToolCall("get_device_hardware", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_7ec4a8148b48a91271f342c512b2af1cdb276744", // Device Hardware
		Options:    args.Options,
	}

	return s.RunNQEQueryByID(ctx, queryArgs)
}

func (s *Service) GetHardwareSupport(ctx context.Context, args GetHardwareSupportArgs) (*Result, error) {
	s.logToolCall("get_hardware_support", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_f0984b777b940b4376ed3ec4317ad47437426e7c", // Hardware Support
		Options:    args.Options,
	}

	return s.RunNQEQueryByID(ctx, queryArgs)
}

func (s *Service) GetOSSupport(ctx context.Context, args GetOSSupportArgs) (*Result, error) {
	s.logToolCall("get_os_support", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_fc33d9fd70ba19a18455b0e4d26ca8420003d9cc", // OS Support
		Options:    args.Options,
	}

	return s.RunNQEQueryByID(ctx, queryArgs)
}

func (s *Service) SearchConfigs(ctx context.Context, args SearchConfigsArgs) (*Result, error) {
	s.logToolCall("search_configs", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_e636c47826ad7144f09eaf6bc14dfb0b560e7cc9", // Config Search
		Parameters: map[string]interface{}{
			"searchPattern": args.SearchTerm,
		},
		Options: args.Options,
	}

	return s.RunNQEQueryByID(ctx, queryArgs)
}

func (s *Service) GetConfigDiff(ctx context.Context, args GetConfigDiffArgs) (*Result, error) {
	s.logToolCall("get_config_diff", args, nil)

	params := map[string]interface{}{}
	if args.AfterSnapshot != "" {
		params["compareSnapshotId"] = args.AfterSnapshot
	}

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.BeforeSnapshot,
		QueryID:    "FQ_51f090cbea069b4049eb283716ab3bbb3f578aea", // Config Diff
		Parameters: params,
		Options:    args.Options,
	}

	return s.RunNQEQueryByID(ctx, queryArgs)
}

// Default Settings Management Tool Implementations
