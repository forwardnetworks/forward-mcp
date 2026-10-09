// Package ports declares every capability the use cases need from outside the
// process, and re-exports the domain types those capabilities carry, so that an
// adapter depends on this package alone.
package ports

import (
	"context"

	"github.com/forward-mcp/internal/domain"
)

// ForwardAPI is the Forward Networks platform as the use cases see it. Every
// method takes the caller's context: when an MCP client cancels a tool call,
// the HTTP request stops.
type ForwardAPI interface {
	// Networks
	GetNetworks(ctx context.Context) ([]Network, error)
	CreateNetwork(ctx context.Context, name string) (*Network, error)
	DeleteNetwork(ctx context.Context, networkID string) (*Network, error)
	UpdateNetwork(ctx context.Context, networkID string, update *NetworkUpdate) (*Network, error)

	// Path search
	SearchPathsBulk(ctx context.Context, networkID string, request *PathSearchBulkRequest, snapshotID string) ([]PathSearchBulkResponse, error)

	// NQE
	RunNQEQueryByID(ctx context.Context, params *NQEQueryParams) (*NQERunResult, error)
	DiffNQEQuery(ctx context.Context, before, after string, request *NQEDiffRequest) (*NQEDiffResult, error)
	GetNQEQueries(ctx context.Context, dir string) ([]NQEQuery, error)
	GetNQEOrgQueries(ctx context.Context) ([]NQEQuery, error)
	GetNQEFwdQueries(ctx context.Context) ([]NQEQuery, error)
	// GetNQEAllQueriesEnhanced fetches source code and metadata for every query.
	// existingCommitIDs maps a query path to the commit already stored; queries
	// whose commit has not changed are not fetched again. It may be nil.
	GetNQEAllQueriesEnhanced(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error)

	// Devices
	GetDevices(ctx context.Context, networkID string, params *DeviceQueryParams) (*DeviceResponse, error)
	GetDeviceLocations(ctx context.Context, networkID string) (map[string]string, error)
	UpdateDeviceLocations(ctx context.Context, networkID string, locations map[string]string) error

	// Snapshots
	GetSnapshots(ctx context.Context, networkID string) ([]Snapshot, error)
	GetLatestSnapshot(ctx context.Context, networkID string) (*Snapshot, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error

	// Locations
	GetLocations(ctx context.Context, networkID string) ([]Location, error)
	CreateLocation(ctx context.Context, networkID string, location *LocationCreate) (*Location, error)
	CreateLocationsBulk(ctx context.Context, networkID string, locations []LocationBulkPatch) error
	UpdateLocation(ctx context.Context, networkID string, locationID string, update *LocationUpdate) (*Location, error)
	DeleteLocation(ctx context.Context, networkID string, locationID string) (*Location, error)
}

// Domain types carried by ForwardAPI.
type (
	Network                = domain.Network
	NetworkUpdate          = domain.NetworkUpdate
	PathSearchParams       = domain.PathSearchParams
	PathSearchBulkRequest  = domain.PathSearchBulkRequest
	PathSearchResponse     = domain.PathSearchResponse
	PathSearchBulkResponse = domain.PathSearchBulkResponse
	PathSearchInfo         = domain.PathSearchInfo
	TotalHits              = domain.TotalHits
	BulkPath               = domain.BulkPath
	BulkHop                = domain.BulkHop
	Path                   = domain.Path
	Hop                    = domain.Hop
	NQEQueryParams         = domain.NQEQueryParams
	NQEQueryOptions        = domain.NQEQueryOptions
	NQESortBy              = domain.NQESortBy
	NQEColumnFilter        = domain.NQEColumnFilter
	NQERunResult           = domain.NQERunResult
	NQEQuery               = domain.NQEQuery
	NQEOrgQuerySummary     = domain.NQEOrgQuerySummary
	NQEOrgQueriesResponse  = domain.NQEOrgQueriesResponse
	NQECommitInfo          = domain.NQECommitInfo
	NQEQueryDetail         = domain.NQEQueryDetail
	NQEDiffRequest         = domain.NQEDiffRequest
	NQEDiffResult          = domain.NQEDiffResult
	DeviceQueryParams      = domain.DeviceQueryParams
	DeviceResponse         = domain.DeviceResponse
	Device                 = domain.Device
	DeviceInterface        = domain.DeviceInterface
	Snapshot               = domain.Snapshot
	SnapshotsResponse      = domain.SnapshotsResponse
	Location               = domain.Location
	LocationCreate         = domain.LocationCreate
	LocationUpdate         = domain.LocationUpdate
	LocationBulkPatch      = domain.LocationBulkPatch
)
