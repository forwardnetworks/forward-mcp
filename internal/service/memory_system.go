package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forward-mcp/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

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

// MemorySystem manages the knowledge graph memory using SQLite
type MemorySystem struct {
	db         *sql.DB
	logger     *logger.Logger
	dbPath     string
	instanceID string
}

// NewMemorySystem creates a new memory system instance
func NewMemorySystem(logger *logger.Logger, instanceID string) (*MemorySystem, error) {
	// Use same data directory approach as NQE database
	dataDir, err := getWritableDataDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to determine writable data directory: %w", err)
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "memory.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open memory database: %w", err)
	}

	memory := &MemorySystem{
		db:         db,
		logger:     logger,
		dbPath:     dbPath,
		instanceID: instanceID,
	}

	// Initialize schema
	if err := memory.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize memory schema: %w", err)
	}

	logger.Info("Memory system initialized at: %s", dbPath)
	return memory, nil
}

// initSchema creates the database tables for the memory system
func (m *MemorySystem) initSchema() error {
	schema := `
	-- Entities table
	CREATE TABLE IF NOT EXISTS entities (
		id TEXT PRIMARY KEY,
		instance_id TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		metadata TEXT,
		UNIQUE(instance_id, name, type)
	);

	-- Relations table
	CREATE TABLE IF NOT EXISTS relations (
		id TEXT PRIMARY KEY,
		instance_id TEXT NOT NULL,
		from_id TEXT NOT NULL,
		to_id TEXT NOT NULL,
		type TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		properties TEXT,
		FOREIGN KEY(from_id) REFERENCES entities(id) ON DELETE CASCADE,
		FOREIGN KEY(to_id) REFERENCES entities(id) ON DELETE CASCADE,
		UNIQUE(instance_id, from_id, to_id, type)
	);

	-- Observations table
	CREATE TABLE IF NOT EXISTS observations (
		id TEXT PRIMARY KEY,
		instance_id TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		content TEXT NOT NULL,
		type TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		metadata TEXT,
		FOREIGN KEY(entity_id) REFERENCES entities(id) ON DELETE CASCADE
	);

	-- Indexes for better performance
	CREATE INDEX IF NOT EXISTS idx_entities_instance_type ON entities(instance_id, type);
	CREATE INDEX IF NOT EXISTS idx_entities_instance_name ON entities(instance_id, name);
	CREATE INDEX IF NOT EXISTS idx_relations_instance_from ON relations(instance_id, from_id);
	CREATE INDEX IF NOT EXISTS idx_relations_instance_to ON relations(instance_id, to_id);
	CREATE INDEX IF NOT EXISTS idx_observations_instance_entity ON observations(instance_id, entity_id);
	CREATE INDEX IF NOT EXISTS idx_observations_content_fts ON observations(content);
	`

	_, err := m.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create memory schema: %w", err)
	}

	return nil
}

// CreateEntity creates a new entity in the knowledge graph
func (m *MemorySystem) CreateEntity(name, entityType string, metadata map[string]interface{}) (*Entity, error) {
	entityID := fmt.Sprintf("entity_%d", time.Now().UnixNano())
	now := time.Now()

	var metadataJSON string
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO entities (id, instance_id, name, type, created_at, updated_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, entityID, m.instanceID, name, entityType, now.Unix(), now.Unix(), metadataJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	entity := &Entity{
		ID:        entityID,
		Name:      name,
		Type:      entityType,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  metadata,
	}

	m.logger.Debug("Created entity: %s (%s)", name, entityType)
	return entity, nil
}

// CreateRelation creates a new relation between two entities
func (m *MemorySystem) CreateRelation(fromID, toID, relationType string, properties map[string]interface{}) (*Relation, error) {
	relationID := fmt.Sprintf("relation_%d", time.Now().UnixNano())
	now := time.Now()

	var propertiesJSON string
	if properties != nil {
		data, err := json.Marshal(properties)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal properties: %w", err)
		}
		propertiesJSON = string(data)
	}

	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO relations (id, instance_id, from_id, to_id, type, created_at, properties)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, relationID, m.instanceID, fromID, toID, relationType, now.Unix(), propertiesJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to create relation: %w", err)
	}

	relation := &Relation{
		ID:         relationID,
		FromID:     fromID,
		ToID:       toID,
		Type:       relationType,
		CreatedAt:  now,
		Properties: properties,
	}

	m.logger.Debug("Created relation: %s -> %s (%s)", fromID, toID, relationType)
	return relation, nil
}

// AddObservation adds an observation to an entity
func (m *MemorySystem) AddObservation(entityID, content, observationType string, metadata map[string]interface{}) (*Observation, error) {
	observationID := fmt.Sprintf("observation_%d", time.Now().UnixNano())
	now := time.Now()

	var metadataJSON string
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	_, err := m.db.Exec(`
		INSERT INTO observations (id, instance_id, entity_id, content, type, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, observationID, m.instanceID, entityID, content, observationType, now.Unix(), metadataJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to add observation: %w", err)
	}

	observation := &Observation{
		ID:        observationID,
		EntityID:  entityID,
		Content:   content,
		Type:      observationType,
		CreatedAt: now,
		Metadata:  metadata,
	}

	m.logger.Debug("Added observation to entity %s: %s", entityID, content)
	return observation, nil
}

// SearchEntities searches for entities by name, type, or content
func (m *MemorySystem) SearchEntities(query string, entityType string, limit int) ([]*Entity, error) {
	if limit <= 0 {
		limit = 50
	}

	var args []interface{}
	whereClause := "WHERE e.instance_id = ?"
	args = append(args, m.instanceID)

	if entityType != "" {
		whereClause += " AND e.type = ?"
		args = append(args, entityType)
	}

	if query != "" {
		whereClause += " AND (e.name LIKE ? OR EXISTS (SELECT 1 FROM observations o WHERE o.entity_id = e.id AND o.content LIKE ?))"
		queryPattern := "%" + query + "%"
		args = append(args, queryPattern, queryPattern)
	}

	sql := fmt.Sprintf(`
		SELECT DISTINCT e.id, e.name, e.type, e.created_at, e.updated_at, e.metadata
		FROM entities e
		%s
		ORDER BY e.updated_at DESC
		LIMIT ?
	`, whereClause)
	args = append(args, limit)

	rows, err := m.db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		entity, err := m.scanEntity(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// GetEntity retrieves an entity by ID or name
func (m *MemorySystem) GetEntity(identifier string) (*Entity, error) {
	var entity *Entity
	var err error

	// Try by ID first
	entity, err = m.getEntityByID(identifier)
	if err == nil {
		return entity, nil
	}

	// Try by name
	entity, err = m.getEntityByName(identifier)
	if err == nil {
		return entity, nil
	}

	return nil, fmt.Errorf("entity not found: %s", identifier)
}

// getEntityByID retrieves an entity by ID
func (m *MemorySystem) getEntityByID(id string) (*Entity, error) {
	row := m.db.QueryRow(`
		SELECT id, name, type, created_at, updated_at, metadata
		FROM entities
		WHERE instance_id = ? AND id = ?
	`, m.instanceID, id)

	return m.scanEntityRow(row)
}

// getEntityByName retrieves an entity by name
func (m *MemorySystem) getEntityByName(name string) (*Entity, error) {
	row := m.db.QueryRow(`
		SELECT id, name, type, created_at, updated_at, metadata
		FROM entities
		WHERE instance_id = ? AND name = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, m.instanceID, name)

	return m.scanEntityRow(row)
}

// GetRelations retrieves relations for an entity
func (m *MemorySystem) GetRelations(entityID string, relationType string) ([]*Relation, error) {
	var whereClause string
	var args []interface{}

	if relationType != "" {
		whereClause = "WHERE instance_id = ? AND (from_id = ? OR to_id = ?) AND type = ?"
		args = []interface{}{m.instanceID, entityID, entityID, relationType}
	} else {
		whereClause = "WHERE instance_id = ? AND (from_id = ? OR to_id = ?)"
		args = []interface{}{m.instanceID, entityID, entityID}
	}

	rows, err := m.db.Query(fmt.Sprintf(`
		SELECT id, from_id, to_id, type, created_at, properties
		FROM relations
		%s
		ORDER BY created_at DESC
	`, whereClause), args...)

	if err != nil {
		return nil, fmt.Errorf("failed to get relations: %w", err)
	}
	defer rows.Close()

	var relations []*Relation
	for rows.Next() {
		relation, err := m.scanRelation(rows)
		if err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}

	return relations, nil
}

// GetObservations retrieves observations for an entity
func (m *MemorySystem) GetObservations(entityID string, observationType string) ([]*Observation, error) {
	var whereClause string
	var args []interface{}

	if observationType != "" {
		whereClause = "WHERE instance_id = ? AND entity_id = ? AND type = ?"
		args = []interface{}{m.instanceID, entityID, observationType}
	} else {
		whereClause = "WHERE instance_id = ? AND entity_id = ?"
		args = []interface{}{m.instanceID, entityID}
	}

	rows, err := m.db.Query(fmt.Sprintf(`
		SELECT id, entity_id, content, type, created_at, metadata
		FROM observations
		%s
		ORDER BY created_at DESC
	`, whereClause), args...)

	if err != nil {
		return nil, fmt.Errorf("failed to get observations: %w", err)
	}
	defer rows.Close()

	var observations []*Observation
	for rows.Next() {
		observation, err := m.scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}

	return observations, nil
}

// DeleteEntity removes an entity and all its relations and observations
func (m *MemorySystem) DeleteEntity(entityID string) error {
	_, err := m.db.Exec(`
		DELETE FROM entities WHERE instance_id = ? AND id = ?
	`, m.instanceID, entityID)

	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	m.logger.Debug("Deleted entity: %s", entityID)
	return nil
}

// DeleteRelation removes a specific relation
func (m *MemorySystem) DeleteRelation(relationID string) error {
	_, err := m.db.Exec(`
		DELETE FROM relations WHERE instance_id = ? AND id = ?
	`, m.instanceID, relationID)

	if err != nil {
		return fmt.Errorf("failed to delete relation: %w", err)
	}

	m.logger.Debug("Deleted relation: %s", relationID)
	return nil
}

// DeleteObservation removes a specific observation
func (m *MemorySystem) DeleteObservation(observationID string) error {
	_, err := m.db.Exec(`
		DELETE FROM observations WHERE instance_id = ? AND id = ?
	`, m.instanceID, observationID)

	if err != nil {
		return fmt.Errorf("failed to delete observation: %w", err)
	}

	m.logger.Debug("Deleted observation: %s", observationID)
	return nil
}

// GetMemoryStats returns statistics about the memory system
func (m *MemorySystem) GetMemoryStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count entities
	var entityCount int
	err := m.db.QueryRow("SELECT COUNT(*) FROM entities WHERE instance_id = ?", m.instanceID).Scan(&entityCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count entities: %w", err)
	}
	stats["entity_count"] = entityCount

	// Count relations
	var relationCount int
	err = m.db.QueryRow("SELECT COUNT(*) FROM relations WHERE instance_id = ?", m.instanceID).Scan(&relationCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count relations: %w", err)
	}
	stats["relation_count"] = relationCount

	// Count observations
	var observationCount int
	err = m.db.QueryRow("SELECT COUNT(*) FROM observations WHERE instance_id = ?", m.instanceID).Scan(&observationCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count observations: %w", err)
	}
	stats["observation_count"] = observationCount

	// Entity types
	rows, err := m.db.Query(`
		SELECT type, COUNT(*) 
		FROM entities 
		WHERE instance_id = ? 
		GROUP BY type
	`, m.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity types: %w", err)
	}
	defer rows.Close()

	entityTypes := make(map[string]int)
	for rows.Next() {
		var entityType string
		var count int
		if err := rows.Scan(&entityType, &count); err != nil {
			continue
		}
		entityTypes[entityType] = count
	}
	stats["entity_types"] = entityTypes

	stats["database_path"] = m.dbPath
	stats["instance_id"] = m.instanceID

	return stats, nil
}

// Close closes the memory database connection
func (m *MemorySystem) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Helper methods for scanning database rows

func (m *MemorySystem) scanEntity(rows *sql.Rows) (*Entity, error) {
	var entity Entity
	var metadataJSON sql.NullString
	var createdAt, updatedAt int64

	err := rows.Scan(
		&entity.ID,
		&entity.Name,
		&entity.Type,
		&createdAt,
		&updatedAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan entity: %w", err)
	}

	entity.CreatedAt = time.Unix(createdAt, 0)
	entity.UpdatedAt = time.Unix(updatedAt, 0)

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &entity.Metadata); err != nil {
			m.logger.Warn("Failed to unmarshal entity metadata: %v", err)
		}
	}

	return &entity, nil
}

func (m *MemorySystem) scanEntityRow(row *sql.Row) (*Entity, error) {
	var entity Entity
	var metadataJSON sql.NullString
	var createdAt, updatedAt int64

	err := row.Scan(
		&entity.ID,
		&entity.Name,
		&entity.Type,
		&createdAt,
		&updatedAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan entity: %w", err)
	}

	entity.CreatedAt = time.Unix(createdAt, 0)
	entity.UpdatedAt = time.Unix(updatedAt, 0)

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &entity.Metadata); err != nil {
			m.logger.Warn("Failed to unmarshal entity metadata: %v", err)
		}
	}

	return &entity, nil
}

func (m *MemorySystem) scanRelation(rows *sql.Rows) (*Relation, error) {
	var relation Relation
	var propertiesJSON sql.NullString
	var createdAt int64

	err := rows.Scan(
		&relation.ID,
		&relation.FromID,
		&relation.ToID,
		&relation.Type,
		&createdAt,
		&propertiesJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan relation: %w", err)
	}

	relation.CreatedAt = time.Unix(createdAt, 0)

	if propertiesJSON.Valid && propertiesJSON.String != "" {
		if err := json.Unmarshal([]byte(propertiesJSON.String), &relation.Properties); err != nil {
			m.logger.Warn("Failed to unmarshal relation properties: %v", err)
		}
	}

	return &relation, nil
}

func (m *MemorySystem) scanObservation(rows *sql.Rows) (*Observation, error) {
	var observation Observation
	var metadataJSON sql.NullString
	var createdAt int64

	err := rows.Scan(
		&observation.ID,
		&observation.EntityID,
		&observation.Content,
		&observation.Type,
		&createdAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan observation: %w", err)
	}

	observation.CreatedAt = time.Unix(createdAt, 0)

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &observation.Metadata); err != nil {
			m.logger.Warn("Failed to unmarshal observation metadata: %v", err)
		}
	}

	return &observation, nil
}
