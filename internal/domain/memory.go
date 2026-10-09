package domain

import "time"

// Entity represents a node in the knowledge graph
type Entity struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Relation represents an edge between two entities
type Relation struct {
	ID         string                 `json:"id"`
	FromID     string                 `json:"from_id"`
	ToID       string                 `json:"to_id"`
	Type       string                 `json:"type"`
	CreatedAt  time.Time              `json:"created_at"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Observation represents additional information about an entity
type Observation struct {
	ID        string                 `json:"id"`
	EntityID  string                 `json:"entity_id"`
	Content   string                 `json:"content"`
	Type      string                 `json:"type"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// InstanceInfo represents information about a database instance
type InstanceInfo struct {
	ID         string    `json:"id"`
	QueryCount int       `json:"query_count"`
	LastSync   time.Time `json:"last_sync"`
	FirstSync  time.Time `json:"first_sync"`
}
