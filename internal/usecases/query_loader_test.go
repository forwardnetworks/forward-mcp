package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/adapters/secondary/queryindex"
	"github.com/forward-mcp/internal/adapters/secondary/sqlite"
	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/domain"
)

// BenchmarkDatabaseHydration measures the time to load queries from API and save to database
func BenchmarkDatabaseHydration(b *testing.B) {
	log := stderrlog.New()
	mockClient := NewMockForwardClient()

	// Create temporary database for each iteration
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		instanceID := "benchmark-hydration"
		db, err := sqlite.NewNQEDatabase(log, instanceID)
		if err != nil {
			b.Fatalf("Failed to create database: %v", err)
		}
		loader := queryLoader{db}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		b.StartTimer()

		// Measure synchronous load (first-time hydration)
		queries, err := loader.synchronousLoad(ctx, mockClient, log, []domain.NQEQueryDetail{})

		b.StopTimer()
		cancel()
		if err != nil {
			b.Fatalf("Failed to load queries: %v", err)
		}
		if len(queries) == 0 {
			b.Fatal("Expected queries, got empty result")
		}

		// Cleanup
		db.Close()
	}
}

// BenchmarkIncrementalUpdate measures the time for background refresh with existing commit IDs
func BenchmarkIncrementalUpdate(b *testing.B) {
	log := stderrlog.New()
	mockClient := NewMockForwardClient()
	instanceID := "benchmark-incremental"

	// Setup: create database with existing queries
	db, err := sqlite.NewNQEDatabase(log, instanceID)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	loader := queryLoader{db}
	ctx := context.Background()

	// Load initial queries
	initialQueries, err := loader.synchronousLoad(ctx, mockClient, log, []domain.NQEQueryDetail{})
	if err != nil {
		b.Fatalf("Failed initial load: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Measure incremental update with existing commit IDs
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		loader.backgroundEnhancedLoadWithContext(bgCtx, mockClient, log, initialQueries)
		cancel()
	}
}

// BenchmarkQueryStoreOperations measures database read/write performance
func BenchmarkQueryStoreOperations(b *testing.B) {
	log := stderrlog.New()
	instanceID := "benchmark-store-ops"

	db, err := sqlite.NewNQEDatabase(log, instanceID)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Generate test queries
	mockClient := NewMockForwardClient()
	ctx := context.Background()
	queries, err := mockClient.GetNQEAllQueriesEnhanced(ctx, nil)
	if err != nil {
		b.Fatalf("Failed to get queries: %v", err)
	}

	b.Run("SaveQueries", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := db.SaveQueries(queries); err != nil {
				b.Fatalf("Failed to save queries: %v", err)
			}
		}
	})

	b.Run("LoadQueries", func(b *testing.B) {
		// Ensure queries are saved first
		if err := db.SaveQueries(queries); err != nil {
			b.Fatalf("Failed to save queries: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			loaded, err := db.LoadQueries()
			if err != nil {
				b.Fatalf("Failed to load queries: %v", err)
			}
			if len(loaded) == 0 {
				b.Fatal("Expected queries, got empty result")
			}
		}
	})
}

// BenchmarkSmartCachingStrategy measures the complete smart caching flow
func BenchmarkSmartCachingStrategy(b *testing.B) {
	log := stderrlog.New()
	mockClient := NewMockForwardClient()
	instanceID := "benchmark-smart-cache"

	db, err := sqlite.NewNQEDatabase(log, instanceID)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	loader := queryLoader{db}

	b.Run("EmptyDatabase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := loader.loadWithSmartCachingContext(ctx, mockClient, log)
			cancel()
			if err != nil {
				b.Fatalf("Failed smart caching load: %v", err)
			}
		}
	})

	b.Run("PopulatedDatabase", func(b *testing.B) {
		// Pre-populate database
		ctx := context.Background()
		queries, _ := mockClient.GetNQEAllQueriesEnhanced(ctx, nil)
		db.SaveQueries(queries)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := loader.loadWithSmartCachingContext(ctx, mockClient, log)
			cancel()
			if err != nil {
				b.Fatalf("Failed smart caching load: %v", err)
			}
		}
	})
}

// BenchmarkCompleteAutoHydration measures the full auto-hydration flow (database + embeddings)
func BenchmarkCompleteAutoHydration(b *testing.B) {
	log := stderrlog.New()
	mockClient := NewMockForwardClient()
	instanceID := "benchmark-auto-hydrate"

	// Use keyword embedder (what auto-hydration actually uses)
	embedder := embeddings.New("keyword", "", log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create fresh database and index for each iteration
		db, err := sqlite.NewNQEDatabase(log, instanceID)
		if err != nil {
			b.Fatalf("Failed to create database: %v", err)
		}

		queryIndex := queryindex.NewNQEQueryIndex(embedder, log)
		b.StartTimer()

		// Simulate autoHydrateDatabase from main.go
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)

		// Load queries
		queries, err := db.LoadQueries()
		queryCount := 0
		if err == nil {
			queryCount = len(queries)
		}

		// Hydrate if sparse
		if err != nil || queryCount < 100 {
			freshQueries, err := mockClient.GetNQEAllQueriesEnhanced(ctx, nil)
			if err != nil {
				cancel()
				db.Close()
				b.Fatalf("Failed to fetch queries: %v", err)
			}

			if err := db.SaveQueries(freshQueries); err != nil {
				cancel()
				db.Close()
				b.Fatalf("Failed to save queries: %v", err)
			}
			queries = freshQueries
		}

		// Generate embeddings
		if len(queries) > 0 {
			if err := queryIndex.LoadFromQueries(queries); err != nil {
				cancel()
				db.Close()
				b.Fatalf("Failed to load into index: %v", err)
			}

			if err := queryIndex.GenerateEmbeddings(); err != nil {
				cancel()
				db.Close()
				b.Fatalf("Failed to generate embeddings: %v", err)
			}
		}

		b.StopTimer()
		cancel()
		db.Close()
	}
}
