package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/forward-mcp/internal/ports"
)

// failingMemoryStore wraps a real store and fails CreateEntity or
// AddObservation on demand.
type failingMemoryStore struct {
	ports.MemoryStore
	failCreate      bool
	failObservation bool
}

func (f *failingMemoryStore) CreateEntity(name, entityType string, metadata map[string]interface{}) (*ports.Entity, error) {
	if f.failCreate {
		return nil, errors.New("disk full")
	}
	return f.MemoryStore.CreateEntity(name, entityType, metadata)
}

func (f *failingMemoryStore) AddObservation(entityID, content, observationType string, metadata map[string]interface{}) (*ports.Observation, error) {
	if f.failObservation {
		return nil, errors.New("disk full")
	}
	return f.MemoryStore.AddObservation(entityID, content, observationType, metadata)
}

// TestListNetworksAllResultsReportsMemoryFailure checks that list_networks
// only claims the results were stored in memory when both writes succeeded.
func TestListNetworksAllResultsReportsMemoryFailure(t *testing.T) {
	tests := []struct {
		name            string
		failCreate      bool
		failObservation bool
		wantStored      bool
	}{
		{name: "stored", wantStored: true},
		{name: "create entity fails", failCreate: true},
		{name: "add observation fails", failObservation: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := createTestService()
			service.memorySystem = &failingMemoryStore{
				MemoryStore:     service.memorySystem,
				failCreate:      tt.failCreate,
				failObservation: tt.failObservation,
			}

			result, err := service.ListNetworks(context.Background(), ListNetworksArgs{AllResults: true})
			if err != nil {
				t.Fatalf("ListNetworks: %v", err)
			}

			stored := strings.Contains(result.Text, "Stored")
			notStored := strings.Contains(result.Text, "Could not store")
			if stored != tt.wantStored || notStored == tt.wantStored {
				t.Errorf("stored=%v, could-not-store=%v, want stored=%v; response:\n%s", stored, notStored, tt.wantStored, result.Text)
			}
		})
	}
}
