package usecases

import (
	"context"
	"fmt"
	"testing"

	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/domain"
	"github.com/forward-mcp/internal/ports"
)

// memQueryStore is an in-memory ports.QueryStore.
type memQueryStore struct {
	ports.QueryStore
	queries  []domain.NQEQueryDetail
	metadata map[string]string
	saves    int
}

func (m *memQueryStore) LoadQueries() ([]domain.NQEQueryDetail, error) { return m.queries, nil }
func (m *memQueryStore) SaveQueries(q []domain.NQEQueryDetail) error {
	m.queries = q
	m.saves++
	return nil
}
func (m *memQueryStore) SetMetadata(key, value string) error {
	if m.metadata == nil {
		m.metadata = map[string]string{}
	}
	m.metadata[key] = value
	return nil
}

// libraryAPI returns full metadata for every library query whose commit the
// caller does not already have, and records the commit IDs it was given.
type libraryAPI struct {
	ports.ForwardAPI
	library   []domain.NQEQueryDetail
	gotCommit map[string]string
	calls     int
}

func (l *libraryAPI) GetNQEAllQueriesEnhanced(ctx context.Context, existing map[string]string) ([]domain.NQEQueryDetail, error) {
	l.calls++
	l.gotCommit = existing
	var out []domain.NQEQueryDetail
	for _, q := range l.library {
		if existing[q.Path] != q.LastCommit.ID {
			out = append(out, q)
		}
	}
	return out, nil
}

func testLibrary(n int) []domain.NQEQueryDetail {
	lib := make([]domain.NQEQueryDetail, n)
	for i := range lib {
		lib[i] = domain.NQEQueryDetail{
			QueryID:    fmt.Sprintf("FQ_%d", i),
			Path:       fmt.Sprintf("/Q/%d", i),
			Intent:     fmt.Sprintf("Intent %d", i),
			LastCommit: domain.NQECommitInfo{ID: fmt.Sprintf("commit-%d", i)},
		}
	}
	return lib
}

// TestHydrateQueryStore_FillsMissingMetadata covers a store filled by the
// basic loader: paths and IDs only. Hydration must fetch the metadata even
// though the store is not empty.
func TestHydrateQueryStore_FillsMissingMetadata(t *testing.T) {
	library := testLibrary(150)
	store := &memQueryStore{}
	for _, q := range library {
		store.queries = append(store.queries, domain.NQEQueryDetail{QueryID: q.QueryID, Path: q.Path})
	}
	api := &libraryAPI{library: library}

	queries, fetched, err := HydrateQueryStore(context.Background(), store, api, logger.New())
	if err != nil {
		t.Fatalf("HydrateQueryStore: %v", err)
	}
	if !fetched || api.calls != 1 || store.saves != 1 {
		t.Fatalf("fetched=%v api calls=%d saves=%d, want a fetch and a save", fetched, api.calls, store.saves)
	}
	if len(queries) != 150 {
		t.Fatalf("got %d queries, want 150", len(queries))
	}
	for _, q := range store.queries {
		if q.Intent == "" || q.LastCommit.ID == "" {
			t.Fatalf("query %s still has no metadata", q.Path)
		}
	}
	if store.metadata["last_sync"] == "" {
		t.Error("last_sync was not recorded")
	}
}

// TestHydrateQueryStore_ResumesAndSkips checks that a store partly filled by
// an interrupted run fetches only the rest, and that a complete store is left
// alone.
func TestHydrateQueryStore_ResumesAndSkips(t *testing.T) {
	library := testLibrary(150)
	store := &memQueryStore{}
	for i, q := range library {
		if i < 100 {
			store.queries = append(store.queries, q) // fetched before the interruption
		} else {
			store.queries = append(store.queries, domain.NQEQueryDetail{QueryID: q.QueryID, Path: q.Path})
		}
	}
	api := &libraryAPI{library: library}

	if _, _, err := HydrateQueryStore(context.Background(), store, api, logger.New()); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(api.gotCommit) != 100 {
		t.Errorf("API got %d known commits, want 100 so it skips them", len(api.gotCommit))
	}
	for _, q := range store.queries {
		if q.Intent == "" {
			t.Fatalf("query %s still has no metadata after resume", q.Path)
		}
	}

	// Now complete: a second run must not call the API.
	if _, fetched, err := HydrateQueryStore(context.Background(), store, api, logger.New()); err != nil || fetched || api.calls != 1 {
		t.Errorf("complete store: fetched=%v err=%v api calls=%d, want no fetch", fetched, err, api.calls)
	}
}
