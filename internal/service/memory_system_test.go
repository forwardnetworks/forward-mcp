package service

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forward-mcp/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

// Helper function to create a test memory system
func createTestMemorySystem(t *testing.T) *MemorySystem {
	// Create a temporary directory for test database
	tempDir := t.TempDir()

	logger := logger.New()

	// Create the memory system directly with a test database path
	dbPath := filepath.Join(tempDir, "memory.db")

	memorySystem := &MemorySystem{
		logger:     logger,
		dbPath:     dbPath,
		instanceID: "test-instance",
	}

	// Create the database directory
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	memorySystem.db = db

	// Initialize schema
	if err := memorySystem.initSchema(); err != nil {
		t.Fatalf("Failed to initialize test schema: %v", err)
	}

	return memorySystem
}

func TestNewMemorySystem(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	if memorySystem == nil {
		t.Fatal("Expected memory system to be created")
	}

	if memorySystem.instanceID != "test-instance" {
		t.Errorf("Expected instance ID 'test-instance', got '%s'", memorySystem.instanceID)
	}
}

func TestCreateEntity(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	metadata := map[string]interface{}{
		"role": "administrator",
		"team": "networking",
	}

	entity, err := memorySystem.CreateEntity("John Doe", "user", metadata)
	if err != nil {
		t.Fatalf("Failed to create entity: %v", err)
	}

	if entity.Name != "John Doe" {
		t.Errorf("Expected entity name 'John Doe', got '%s'", entity.Name)
	}

	if entity.Type != "user" {
		t.Errorf("Expected entity type 'user', got '%s'", entity.Type)
	}

	if entity.Metadata["role"] != "administrator" {
		t.Errorf("Expected metadata role 'administrator', got '%v'", entity.Metadata["role"])
	}

	if entity.ID == "" {
		t.Error("Expected entity ID to be generated")
	}
}

func TestCreateRelation(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create two entities first
	entity1, err := memorySystem.CreateEntity("Alice", "user", nil)
	if err != nil {
		t.Fatalf("Failed to create entity1: %v", err)
	}

	entity2, err := memorySystem.CreateEntity("Network-A", "network", nil)
	if err != nil {
		t.Fatalf("Failed to create entity2: %v", err)
	}

	properties := map[string]interface{}{
		"permission": "read-write",
		"since":      "2024-01-01",
	}

	relation, err := memorySystem.CreateRelation(entity1.ID, entity2.ID, "manages", properties)
	if err != nil {
		t.Fatalf("Failed to create relation: %v", err)
	}

	if relation.FromID != entity1.ID {
		t.Errorf("Expected FromID '%s', got '%s'", entity1.ID, relation.FromID)
	}

	if relation.ToID != entity2.ID {
		t.Errorf("Expected ToID '%s', got '%s'", entity2.ID, relation.ToID)
	}

	if relation.Type != "manages" {
		t.Errorf("Expected relation type 'manages', got '%s'", relation.Type)
	}

	if relation.Properties["permission"] != "read-write" {
		t.Errorf("Expected permission 'read-write', got '%v'", relation.Properties["permission"])
	}
}

func TestAddObservation(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create an entity first
	entity, err := memorySystem.CreateEntity("Bob", "user", nil)
	if err != nil {
		t.Fatalf("Failed to create entity: %v", err)
	}

	metadata := map[string]interface{}{
		"source":    "conversation",
		"timestamp": time.Now().Unix(),
	}

	observation, err := memorySystem.AddObservation(entity.ID, "Prefers working with Python and Go", "preference", metadata)
	if err != nil {
		t.Fatalf("Failed to add observation: %v", err)
	}

	if observation.EntityID != entity.ID {
		t.Errorf("Expected EntityID '%s', got '%s'", entity.ID, observation.EntityID)
	}

	if observation.Content != "Prefers working with Python and Go" {
		t.Errorf("Expected content 'Prefers working with Python and Go', got '%s'", observation.Content)
	}

	if observation.Type != "preference" {
		t.Errorf("Expected observation type 'preference', got '%s'", observation.Type)
	}

	if observation.Metadata["source"] != "conversation" {
		t.Errorf("Expected source 'conversation', got '%v'", observation.Metadata["source"])
	}
}

func TestSearchEntities(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create test entities
	entity1, _ := memorySystem.CreateEntity("Alice Johnson", "user", nil)
	_, _ = memorySystem.CreateEntity("Bob Smith", "user", nil)
	_, _ = memorySystem.CreateEntity("Production Network", "network", nil)

	// Add observation to make search more interesting
	memorySystem.AddObservation(entity1.ID, "Works on network security", "note", nil)

	// Test search by name
	entities, err := memorySystem.SearchEntities("Alice", "", 10)
	if err != nil {
		t.Fatalf("Failed to search entities: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(entities))
	}

	if entities[0].Name != "Alice Johnson" {
		t.Errorf("Expected 'Alice Johnson', got '%s'", entities[0].Name)
	}

	// Test search by type
	userEntities, err := memorySystem.SearchEntities("", "user", 10)
	if err != nil {
		t.Fatalf("Failed to search by type: %v", err)
	}

	if len(userEntities) != 2 {
		t.Errorf("Expected 2 user entities, got %d", len(userEntities))
	}

	// Test search by observation content
	securityEntities, err := memorySystem.SearchEntities("network security", "", 10)
	if err != nil {
		t.Fatalf("Failed to search by observation: %v", err)
	}

	if len(securityEntities) != 1 {
		t.Errorf("Expected 1 entity with security observation, got %d", len(securityEntities))
	}
}

func TestGetEntity(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create a test entity
	originalEntity, err := memorySystem.CreateEntity("Test Entity", "test", nil)
	if err != nil {
		t.Fatalf("Failed to create entity: %v", err)
	}

	// Test get by ID
	entityByID, err := memorySystem.GetEntity(originalEntity.ID)
	if err != nil {
		t.Fatalf("Failed to get entity by ID: %v", err)
	}

	if entityByID.ID != originalEntity.ID {
		t.Errorf("Expected ID '%s', got '%s'", originalEntity.ID, entityByID.ID)
	}

	// Test get by name
	entityByName, err := memorySystem.GetEntity("Test Entity")
	if err != nil {
		t.Fatalf("Failed to get entity by name: %v", err)
	}

	if entityByName.Name != "Test Entity" {
		t.Errorf("Expected name 'Test Entity', got '%s'", entityByName.Name)
	}

	// Test non-existent entity
	_, err = memorySystem.GetEntity("Non-existent")
	if err == nil {
		t.Error("Expected error for non-existent entity")
	}
}

func TestGetRelations(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create entities and relations
	user, _ := memorySystem.CreateEntity("User1", "user", nil)
	network1, _ := memorySystem.CreateEntity("Network1", "network", nil)
	network2, _ := memorySystem.CreateEntity("Network2", "network", nil)

	relation1, _ := memorySystem.CreateRelation(user.ID, network1.ID, "manages", nil)
	_, _ = memorySystem.CreateRelation(user.ID, network2.ID, "monitors", nil)

	// Test get all relations for user
	relations, err := memorySystem.GetRelations(user.ID, "")
	if err != nil {
		t.Fatalf("Failed to get relations: %v", err)
	}

	if len(relations) != 2 {
		t.Errorf("Expected 2 relations, got %d", len(relations))
	}

	// Test get relations by type
	manageRelations, err := memorySystem.GetRelations(user.ID, "manages")
	if err != nil {
		t.Fatalf("Failed to get manage relations: %v", err)
	}

	if len(manageRelations) != 1 {
		t.Errorf("Expected 1 manage relation, got %d", len(manageRelations))
	}

	if manageRelations[0].ID != relation1.ID {
		t.Errorf("Expected relation ID '%s', got '%s'", relation1.ID, manageRelations[0].ID)
	}
}

func TestGetObservations(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create entity and observations
	entity, _ := memorySystem.CreateEntity("Test User", "user", nil)
	obs1, _ := memorySystem.AddObservation(entity.ID, "First note", "note", nil)
	_, _ = memorySystem.AddObservation(entity.ID, "User preference", "preference", nil)

	// Test get all observations
	observations, err := memorySystem.GetObservations(entity.ID, "")
	if err != nil {
		t.Fatalf("Failed to get observations: %v", err)
	}

	if len(observations) != 2 {
		t.Errorf("Expected 2 observations, got %d", len(observations))
	}

	// Test get observations by type
	notes, err := memorySystem.GetObservations(entity.ID, "note")
	if err != nil {
		t.Fatalf("Failed to get note observations: %v", err)
	}

	if len(notes) != 1 {
		t.Errorf("Expected 1 note observation, got %d", len(notes))
	}

	if notes[0].ID != obs1.ID {
		t.Errorf("Expected observation ID '%s', got '%s'", obs1.ID, notes[0].ID)
	}
}

func TestDeleteEntity(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create entity with relations and observations
	entity, _ := memorySystem.CreateEntity("To Delete", "user", nil)
	otherEntity, _ := memorySystem.CreateEntity("Other", "user", nil)

	memorySystem.CreateRelation(entity.ID, otherEntity.ID, "knows", nil)
	memorySystem.AddObservation(entity.ID, "Test observation", "note", nil)

	// Delete the entity
	err := memorySystem.DeleteEntity(entity.ID)
	if err != nil {
		t.Fatalf("Failed to delete entity: %v", err)
	}

	// Verify entity is deleted
	_, err = memorySystem.GetEntity(entity.ID)
	if err == nil {
		t.Error("Expected error when getting deleted entity")
	}

	// Verify relations are deleted (cascading delete)
	relations, _ := memorySystem.GetRelations(entity.ID, "")
	if len(relations) != 0 {
		t.Errorf("Expected 0 relations after entity deletion, got %d", len(relations))
	}

	// Verify observations are deleted (cascading delete)
	observations, _ := memorySystem.GetObservations(entity.ID, "")
	if len(observations) != 0 {
		t.Errorf("Expected 0 observations after entity deletion, got %d", len(observations))
	}
}

func TestDeleteRelation(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create entities and relation
	entity1, _ := memorySystem.CreateEntity("Entity1", "user", nil)
	entity2, _ := memorySystem.CreateEntity("Entity2", "network", nil)
	relation, _ := memorySystem.CreateRelation(entity1.ID, entity2.ID, "manages", nil)

	// Delete the relation
	err := memorySystem.DeleteRelation(relation.ID)
	if err != nil {
		t.Fatalf("Failed to delete relation: %v", err)
	}

	// Verify relation is deleted
	relations, _ := memorySystem.GetRelations(entity1.ID, "")
	if len(relations) != 0 {
		t.Errorf("Expected 0 relations after deletion, got %d", len(relations))
	}
}

func TestDeleteObservation(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create entity and observation
	entity, _ := memorySystem.CreateEntity("Test Entity", "user", nil)
	observation, _ := memorySystem.AddObservation(entity.ID, "Test observation", "note", nil)

	// Delete the observation
	err := memorySystem.DeleteObservation(observation.ID)
	if err != nil {
		t.Fatalf("Failed to delete observation: %v", err)
	}

	// Verify observation is deleted
	observations, _ := memorySystem.GetObservations(entity.ID, "")
	if len(observations) != 0 {
		t.Errorf("Expected 0 observations after deletion, got %d", len(observations))
	}
}

func TestGetMemoryStats(t *testing.T) {
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create test data
	user1, _ := memorySystem.CreateEntity("User1", "user", nil)
	_, _ = memorySystem.CreateEntity("User2", "user", nil)
	network, _ := memorySystem.CreateEntity("Network1", "network", nil)

	memorySystem.CreateRelation(user1.ID, network.ID, "manages", nil)
	memorySystem.AddObservation(user1.ID, "Test observation", "note", nil)

	stats, err := memorySystem.GetMemoryStats()
	if err != nil {
		t.Fatalf("Failed to get memory stats: %v", err)
	}

	if stats["entity_count"] != 3 {
		t.Errorf("Expected 3 entities, got %v", stats["entity_count"])
	}

	if stats["relation_count"] != 1 {
		t.Errorf("Expected 1 relation, got %v", stats["relation_count"])
	}

	if stats["observation_count"] != 1 {
		t.Errorf("Expected 1 observation, got %v", stats["observation_count"])
	}

	entityTypes := stats["entity_types"].(map[string]int)
	if entityTypes["user"] != 2 {
		t.Errorf("Expected 2 user entities, got %d", entityTypes["user"])
	}

	if entityTypes["network"] != 1 {
		t.Errorf("Expected 1 network entity, got %d", entityTypes["network"])
	}
}

func TestMemorySystemClose(t *testing.T) {
	memorySystem := createTestMemorySystem(t)

	err := memorySystem.Close()
	if err != nil {
		t.Fatalf("Failed to close memory system: %v", err)
	}

	// Try to use closed memory system - should still work with new connection
	// SQLite allows this, but we test that Close() doesn't break anything
	err = memorySystem.Close() // Close again should not error
	if err != nil {
		t.Errorf("Expected no error on double close, got: %v", err)
	}
}
