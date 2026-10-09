package service

import (
	"context"
	"fmt"
	"time"

	"github.com/forward-mcp/internal/domain"
	"github.com/forward-mcp/internal/ports"
)

// queryLoader fills the query store from the Forward API: enhanced metadata
// when it can, the basic query list when it cannot, merged with what is
// already stored.
type queryLoader struct {
	ports.QueryStore
}

// loadWithSmartCachingContext implements the smart caching strategy with context support
func (db queryLoader) loadWithSmartCachingContext(ctx context.Context, client ports.ForwardAPI, logger ports.Logger) ([]domain.NQEQueryDetail, error) {
	logger.Info("Starting smart caching query load...")

	// Step 1: Load existing queries from database for immediate availability
	existingQueries, err := db.LoadQueries()
	if err != nil {
		logger.Debug("Failed to load from database: %v", err)
		existingQueries = []domain.NQEQueryDetail{} // Start with empty if database fails
	}

	logger.Info("Found %d existing queries in database", len(existingQueries))

	// Step 2: If we have sufficient queries, start background enhanced loading
	if len(existingQueries) >= 1000 {
		logger.Info("Starting background Enhanced API loading for metadata enrichment...")
		go db.backgroundEnhancedLoadWithContext(ctx, client, logger, existingQueries)

		// Return existing queries immediately for fast startup
		logger.Info("Returning %d cached queries for immediate use", len(existingQueries))
		return existingQueries, nil
	}

	// Step 3: If database is empty/incomplete, do synchronous loading
	logger.Info("Database incomplete, performing synchronous load...")
	return db.synchronousLoad(ctx, client, logger, existingQueries)
}

// backgroundEnhancedLoadWithContext runs Enhanced API loading in the background with context support
func (db queryLoader) backgroundEnhancedLoadWithContext(ctx context.Context, client ports.ForwardAPI, logger ports.Logger, existingQueries []domain.NQEQueryDetail) {
	logger.Info("🔄 Background Enhanced API loading started...")

	// Check for cancellation before starting
	select {
	case <-ctx.Done():
		logger.Info("🔄 Background Enhanced API loading cancelled before start")
		return
	default:
	}

	// Build commit ID map for incremental updates
	existingCommitIDs := make(map[string]string)
	for _, query := range existingQueries {
		if query.LastCommit.ID != "" {
			existingCommitIDs[query.Path] = query.LastCommit.ID
		}
	}

	// Use the passed context directly - don't create a new timeout
	// The service will handle cancellation and timeout as needed
	enhancedQueries, err := client.GetNQEAllQueriesEnhanced(ctx, existingCommitIDs)
	if err != nil {
		// Check if we were cancelled
		select {
		case <-ctx.Done():
			logger.Info("🔄 Background Enhanced API loading cancelled during API call")
			return
		default:
		}

		logger.Warn("🔄 Background Enhanced API failed: %v", err)
		logger.Info("🔄 Background fallback to Basic API...")

		// Fallback to Basic API in background
		basicQueries, err := db.loadFromBasicAPI(ctx, client, logger)
		if err != nil {
			logger.Error("🔄 Background Basic API also failed: %v", err)
			return
		}

		// Check for cancellation before save
		select {
		case <-ctx.Done():
			logger.Info("🔄 Background Basic API save cancelled")
			return
		default:
		}

		// Merge and save basic queries
		allQueries := db.mergeQueries(existingQueries, basicQueries)
		if err := db.SaveQueries(allQueries); err != nil {
			logger.Error("🔄 Background save failed: %v", err)
		} else {
			logger.Info("🔄 Background Basic API update complete: %d queries saved", len(allQueries))
		}
		return
	}

	// Check for cancellation before save
	select {
	case <-ctx.Done():
		logger.Info("🔄 Background Enhanced API save cancelled")
		return
	default:
	}

	// Merge enhanced queries with existing
	allQueries := db.mergeQueries(existingQueries, enhancedQueries)
	if err := db.SaveQueries(allQueries); err != nil {
		logger.Error("🔄 Background save failed: %v", err)
	} else {
		logger.Info("🔄 Background Enhanced API update complete: %d queries saved with full metadata", len(allQueries))
		if err := db.SetMetadata("last_sync", time.Now().Format(time.RFC3339)); err != nil {
			logger.Error("🔄 Failed to update sync time: %v", err)
		}
	}
}

// synchronousLoad performs synchronous loading when database is incomplete
func (db queryLoader) synchronousLoad(ctx context.Context, client ports.ForwardAPI, logger ports.Logger, existingQueries []domain.NQEQueryDetail) ([]domain.NQEQueryDetail, error) {
	// Build commit ID map for incremental updates
	existingCommitIDs := make(map[string]string)
	for _, query := range existingQueries {
		if query.LastCommit.ID != "" {
			existingCommitIDs[query.Path] = query.LastCommit.ID
		}
	}

	// Try Enhanced API with shorter timeout for synchronous operation
	logger.Info("Attempting Enhanced API (synchronous)...")
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	enhancedQueries, err := db.loadFromEnhancedAPIWithCommitCheck(ctx, client, logger, existingCommitIDs)
	if err != nil {
		logger.Warn("Enhanced API failed: %v", err)
		logger.Info("Falling back to Basic API...")
	} else {
		// Merge with existing queries and save
		allQueries := db.mergeQueries(existingQueries, enhancedQueries)
		if err := db.SaveQueries(allQueries); err != nil {
			logger.Error("Failed to save enhanced queries: %v", err)
		} else {
			logger.Info("Successfully saved %d total queries to database", len(allQueries))
			if err := db.SetMetadata("last_sync", time.Now().Format(time.RFC3339)); err != nil {
				logger.Error("Failed to update sync time: %v", err)
			}
		}
		return allQueries, nil
	}

	// Fallback to Basic API (both org and fwd repositories)
	basicQueries, err := db.loadFromBasicAPI(ctx, client, logger)
	if err != nil {
		logger.Error("Basic API also failed: %v", err)
		if len(existingQueries) > 0 {
			logger.Info("Using %d existing queries from database", len(existingQueries))
			return existingQueries, nil
		}
		return nil, fmt.Errorf("all API methods failed and no database fallback available: %w", err)
	}

	// Merge with existing and save
	allQueries := db.mergeQueries(existingQueries, basicQueries)
	if err := db.SaveQueries(allQueries); err != nil {
		logger.Error("Failed to save basic queries: %v", err)
	} else {
		logger.Info("Successfully saved %d total queries to database", len(allQueries))
		if err := db.SetMetadata("last_sync", time.Now().Format(time.RFC3339)); err != nil {
			logger.Error("Failed to update sync time: %v", err)
		}
	}

	return allQueries, nil
}

// loadFromEnhancedAPIWithCommitCheck loads queries using Enhanced API with commit-based incremental updates
func (db queryLoader) loadFromEnhancedAPIWithCommitCheck(ctx context.Context, client ports.ForwardAPI, logger ports.Logger, existingCommitIDs map[string]string) ([]domain.NQEQueryDetail, error) {
	// Channel to receive results
	resultChan := make(chan []domain.NQEQueryDetail, 1)
	errorChan := make(chan error, 1)

	// Start Enhanced API loading in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errorChan <- fmt.Errorf("enhanced API panic: %v", r)
			}
		}()

		// Load from both repositories with commit checking
		allQueries, err := client.GetNQEAllQueriesEnhanced(ctx, existingCommitIDs)
		if err != nil {
			errorChan <- fmt.Errorf("failed to get queries with commit checking: %w", err)
			return
		}

		logger.Info("Enhanced API with commit checking loaded %d queries", len(allQueries))
		resultChan <- allQueries
	}()

	// Wait for result or timeout
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("enhanced API timed out after 60 seconds")
	case err := <-errorChan:
		return nil, err
	case queries := <-resultChan:
		logger.Info("Enhanced API loaded %d queries successfully (with commit checking)", len(queries))
		return queries, nil
	}
}

// loadFromBasicAPI loads queries from Basic API (both repositories)
func (db queryLoader) loadFromBasicAPI(ctx context.Context, client ports.ForwardAPI, logger ports.Logger) ([]domain.NQEQueryDetail, error) {
	var allQueries []domain.NQEQueryDetail

	// Load from org repository
	logger.Info("Loading queries from org repository...")
	orgQueries, err := client.GetNQEOrgQueries(ctx)
	if err != nil {
		logger.Error("Failed to load org queries: %v", err)
	} else {
		logger.Info("Loaded %d queries from org repository", len(orgQueries))
		// Convert NQEQuery to NQEQueryDetail
		for _, query := range orgQueries {
			detail := domain.NQEQueryDetail{
				QueryID:    query.QueryID,
				Path:       query.Path,
				Intent:     query.Intent,
				Repository: query.Repository,
				// Basic queries don't have source code or commit info
				SourceCode:  "",
				Description: "",
			}
			allQueries = append(allQueries, detail)
		}
	}

	// Load from fwd repository
	logger.Info("Loading queries from fwd repository...")
	fwdQueries, err := client.GetNQEFwdQueries(ctx)
	if err != nil {
		logger.Error("Failed to load fwd queries: %v", err)
	} else {
		logger.Info("Loaded %d queries from fwd repository", len(fwdQueries))
		// Convert NQEQuery to NQEQueryDetail
		for _, query := range fwdQueries {
			detail := domain.NQEQueryDetail{
				QueryID:    query.QueryID,
				Path:       query.Path,
				Intent:     query.Intent,
				Repository: query.Repository,
				// Basic queries don't have source code or commit info
				SourceCode:  "",
				Description: "",
			}
			allQueries = append(allQueries, detail)
		}
	}

	if len(allQueries) == 0 {
		return nil, fmt.Errorf("no queries loaded from either repository")
	}

	// Remove duplicates
	uniqueQueries := db.deduplicateQueries(allQueries)
	logger.Info("After deduplication: %d unique queries", len(uniqueQueries))

	return uniqueQueries, nil
}

// mergeQueries merges existing and new queries, preferring newer data
func (db queryLoader) mergeQueries(existing, new []domain.NQEQueryDetail) []domain.NQEQueryDetail {
	queryMap := make(map[string]domain.NQEQueryDetail)

	// Add existing queries
	for _, query := range existing {
		queryMap[query.QueryID] = query
	}

	// Add/update with new queries (overwrites existing)
	for _, query := range new {
		queryMap[query.QueryID] = query
	}

	// Convert back to slice
	var result []domain.NQEQueryDetail
	for _, query := range queryMap {
		result = append(result, query)
	}

	return result
}

// deduplicateQueries removes duplicate queries by QueryID
func (db queryLoader) deduplicateQueries(queries []domain.NQEQueryDetail) []domain.NQEQueryDetail {
	seen := make(map[string]bool)
	var unique []domain.NQEQueryDetail

	for _, query := range queries {
		if !seen[query.QueryID] {
			seen[query.QueryID] = true
			unique = append(unique, query)
		}
	}

	return unique
}
