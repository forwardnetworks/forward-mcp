package ports

import "github.com/forward-mcp/internal/domain"

// QueryStore persists NQE query metadata, partitioned by instance.
type QueryStore interface {
	SaveQueries(queries []NQEQueryDetail) error
	LoadQueries() ([]NQEQueryDetail, error)
	SetMetadata(key, value string) error
	GetMetadata(key string) (string, error)
	GetAllInstanceIDs() ([]InstanceInfo, error)
	// AddUpdateCallback registers a function to run after SaveQueries.
	AddUpdateCallback(callback func())
	// Path is where the store keeps its data, for status reports.
	Path() string
	Close() error
}

// MemoryStore is the knowledge graph: entities, the relations between them,
// and observations about them. It also keeps large NQE results in chunks.
type MemoryStore interface {
	CreateEntity(name, entityType string, metadata map[string]interface{}) (*Entity, error)
	CreateRelation(fromID, toID, relationType string, properties map[string]interface{}) (*Relation, error)
	AddObservation(entityID, content, observationType string, metadata map[string]interface{}) (*Observation, error)
	SearchEntities(query string, entityType string, limit int) ([]*Entity, error)
	// GetEntity finds an entity by ID, then by name.
	GetEntity(identifier string) (*Entity, error)
	// GetEntityByName finds the most recently updated entity with this name.
	GetEntityByName(name string) (*Entity, error)
	GetRelations(entityID string, relationType string) ([]*Relation, error)
	GetObservations(entityID string, observationType string) ([]*Observation, error)
	DeleteEntity(entityID string) error
	DeleteRelation(relationID string) error
	DeleteObservation(observationID string) error
	GetMemoryStats() (map[string]interface{}, error)
	StoreNQEResultWithChunking(queryID, networkID, snapshotID string, result *NQERunResult, chunkSize int) (string, error)
	GetNQEResultChunks(resultEntityID string) ([]string, error)
	Close() error
}

// Domain types carried by the stores.
type (
	Entity       = domain.Entity
	Relation     = domain.Relation
	Observation  = domain.Observation
	InstanceInfo = domain.InstanceInfo
)
