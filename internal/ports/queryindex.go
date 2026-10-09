package ports

import "github.com/forward-mcp/internal/domain"

// QueryIndex is the NQE query library, searchable by what a query is for.
type QueryIndex interface {
	IsReady() bool
	IsLoading() bool
	// LoadFromSpec loads the bundled query specification.
	LoadFromSpec() error
	// SpecPath locates the bundled query specification that LoadFromSpec reads.
	SpecPath() (string, error)
	// LoadFromQueries replaces the index with queries from the database or API.
	LoadFromQueries(queries []NQEQueryDetail) error
	// GenerateEmbeddings embeds every query that has no embedding yet.
	GenerateEmbeddings() error
	// UsesSyntheticEmbeddings reports whether GenerateEmbeddings would refuse.
	UsesSyntheticEmbeddings() bool
	SearchQueries(searchText string, limit int) ([]*QuerySearchResult, error)
	GetQueryByID(queryID string) (*NQEQueryIndexEntry, error)
	FilterQueriesByDirectory(directory string) []*NQEQueryIndexEntry
	GetStatistics() map[string]interface{}
}

// Domain types carried by QueryIndex.
type (
	NQEQueryIndexEntry = domain.NQEQueryIndexEntry
	QuerySearchResult  = domain.QuerySearchResult
)
