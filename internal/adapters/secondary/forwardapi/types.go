// Package forwardapi is the Forward Networks REST client, the adapter behind
// ports.ForwardAPI.
package forwardapi

import "github.com/forward-mcp/internal/ports"

// The types this adapter speaks, named through the port.
type (
	Network                = ports.Network
	NetworkUpdate          = ports.NetworkUpdate
	PathSearchParams       = ports.PathSearchParams
	PathSearchBulkRequest  = ports.PathSearchBulkRequest
	PathSearchResponse     = ports.PathSearchResponse
	PathSearchBulkResponse = ports.PathSearchBulkResponse
	PathSearchInfo         = ports.PathSearchInfo
	TotalHits              = ports.TotalHits
	BulkPath               = ports.BulkPath
	BulkHop                = ports.BulkHop
	Path                   = ports.Path
	Hop                    = ports.Hop
	NQEQueryParams         = ports.NQEQueryParams
	NQEQueryOptions        = ports.NQEQueryOptions
	NQESortBy              = ports.NQESortBy
	NQEColumnFilter        = ports.NQEColumnFilter
	NQERunResult           = ports.NQERunResult
	NQEQuery               = ports.NQEQuery
	NQEOrgQuerySummary     = ports.NQEOrgQuerySummary
	NQEOrgQueriesResponse  = ports.NQEOrgQueriesResponse
	NQECommitInfo          = ports.NQECommitInfo
	NQEQueryDetail         = ports.NQEQueryDetail
	NQEDiffRequest         = ports.NQEDiffRequest
	NQEDiffResult          = ports.NQEDiffResult
	DeviceQueryParams      = ports.DeviceQueryParams
	DeviceResponse         = ports.DeviceResponse
	Device                 = ports.Device
	DeviceInterface        = ports.DeviceInterface
	Snapshot               = ports.Snapshot
	SnapshotsResponse      = ports.SnapshotsResponse
	Location               = ports.Location
	LocationCreate         = ports.LocationCreate
	LocationUpdate         = ports.LocationUpdate
	LocationBulkPatch      = ports.LocationBulkPatch
)
