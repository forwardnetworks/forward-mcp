package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

// getWritableDataDirectory returns a directory where we can write the database
func getWritableDataDirectory() (string, error) {
	// Try different locations in order of preference for Claude Desktop compatibility
	candidates := []string{
		// 1. User's home directory (most consistent across runs)
		func() string {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, ".forward-mcp", "data")
			}
			return ""
		}(),
		// 2. Current directory (for development)
		"data",
		// 3. System temp directory (last resort)
		filepath.Join(os.TempDir(), "forward-mcp", "data"),
	}

	for _, dir := range candidates {
		if dir == "" {
			continue
		}

		// Test if we can create the directory and write to it
		if err := os.MkdirAll(dir, 0755); err == nil {
			// Test write permission with a temporary file
			testFile := filepath.Join(dir, ".write_test")
			if file, err := os.Create(testFile); err == nil {
				file.Close()
				os.Remove(testFile) // Clean up
				return dir, nil
			}
		}
	}

	return "", fmt.Errorf("no writable directory found for database storage")
}

// NQEDatabase manages the SQLite database for NQE queries
type NQEDatabase struct {
	db              *sql.DB
	logger          *logger.Logger
	dbPath          string
	instanceID      string   // Unique identifier for this Forward Networks instance
	updateCallbacks []func() // Callbacks to notify when data is updated
}

// AddUpdateCallback adds a callback that will be called when the database is updated
func (db *NQEDatabase) AddUpdateCallback(callback func()) {
	db.updateCallbacks = append(db.updateCallbacks, callback)
}

// notifyUpdateCallbacks calls all registered update callbacks
func (db *NQEDatabase) notifyUpdateCallbacks() {
	for _, callback := range db.updateCallbacks {
		go callback() // Run callbacks in goroutines to avoid blocking
	}
}

// NewNQEDatabase creates a new database instance
func NewNQEDatabase(logger *logger.Logger, instanceID string) (*NQEDatabase, error) {
	// Get a writable directory for the database
	dataDir, err := getWritableDataDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to determine writable data directory: %w", err)
	}

	logger.Info("Using database directory: %s", dataDir)

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "nqe_queries.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	nqeDB := &NQEDatabase{
		db:         db,
		logger:     logger,
		dbPath:     dbPath,
		instanceID: instanceID,
	}

	// FIRST: Check for and handle schema migration BEFORE creating schema
	if err := nqeDB.migrateToInstancePartitioning(); err != nil {
		logger.Warn("Failed to migrate existing data to instance partitioning: %v", err)
		// Continue anyway - worst case is we'll reload from API
	}

	// THEN: Initialize schema (will create new schema if migration dropped old tables)
	if err := nqeDB.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nqeDB, nil
}

// initSchema creates the database tables if they don't exist
func (db *NQEDatabase) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nqe_queries (
		instance_id TEXT NOT NULL,
		query_id TEXT NOT NULL,
		path TEXT NOT NULL,
		intent TEXT,
		source_code TEXT,
		description TEXT,
		repository TEXT,
		last_commit_id TEXT,
		last_commit_author TEXT,
		last_commit_date INTEGER,
		last_commit_title TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY (instance_id, query_id)
	);

	CREATE INDEX IF NOT EXISTS idx_instance_path ON nqe_queries(instance_id, path);
	CREATE INDEX IF NOT EXISTS idx_instance_repository ON nqe_queries(instance_id, repository);
	CREATE INDEX IF NOT EXISTS idx_instance_intent ON nqe_queries(instance_id, intent);
	CREATE INDEX IF NOT EXISTS idx_instance_id ON nqe_queries(instance_id);

	-- Metadata table for database info (partitioned by instance)
	CREATE TABLE IF NOT EXISTS db_metadata (
		instance_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT,
		updated_at INTEGER,
		PRIMARY KEY (instance_id, key)
	);

	CREATE INDEX IF NOT EXISTS idx_metadata_instance ON db_metadata(instance_id);
	`

	if _, err := db.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// migrateToInstancePartitioning migrates existing data to the new instance-partitioned schema
func (db *NQEDatabase) migrateToInstancePartitioning() error {
	// Check if nqe_queries table exists at all
	var tableExists int
	err := db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='nqe_queries'").Scan(&tableExists)
	if err != nil {
		db.logger.Debug("Could not check if table exists: %v", err)
		return nil
	}

	if tableExists == 0 {
		// No table exists yet - fresh installation, no migration needed
		db.logger.Debug("No existing tables found - fresh installation")
		return nil
	}

	// Check if the instance_id column exists in the existing table
	var hasInstanceColumn int
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('nqe_queries') WHERE name = 'instance_id'").Scan(&hasInstanceColumn)
	if err != nil {
		db.logger.Debug("Could not check schema: %v", err)
		return nil
	}

	if hasInstanceColumn == 0 {
		db.logger.Info("Old database schema detected (no instance_id column), migrating data...")

		// First, backup the existing data
		backupQueries := []struct {
			QueryID          string
			Path             string
			Intent           sql.NullString
			SourceCode       sql.NullString
			Description      sql.NullString
			Repository       sql.NullString
			LastCommitID     sql.NullString
			LastCommitAuthor sql.NullString
			LastCommitDate   sql.NullInt64
			LastCommitTitle  sql.NullString
			CreatedAt        int64
			UpdatedAt        int64
		}{}

		rows, err := db.db.Query(`
			SELECT query_id, path, intent, source_code, description, repository,
				   last_commit_id, last_commit_author, last_commit_date, last_commit_title,
				   created_at, updated_at
			FROM nqe_queries
		`)
		if err != nil {
			db.logger.Warn("Could not read existing data for migration: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var backup struct {
					QueryID          string
					Path             string
					Intent           sql.NullString
					SourceCode       sql.NullString
					Description      sql.NullString
					Repository       sql.NullString
					LastCommitID     sql.NullString
					LastCommitAuthor sql.NullString
					LastCommitDate   sql.NullInt64
					LastCommitTitle  sql.NullString
					CreatedAt        int64
					UpdatedAt        int64
				}

				err := rows.Scan(&backup.QueryID, &backup.Path, &backup.Intent, &backup.SourceCode,
					&backup.Description, &backup.Repository, &backup.LastCommitID, &backup.LastCommitAuthor,
					&backup.LastCommitDate, &backup.LastCommitTitle, &backup.CreatedAt, &backup.UpdatedAt)
				if err != nil {
					db.logger.Warn("Could not scan row during migration: %v", err)
					continue
				}
				backupQueries = append(backupQueries, backup)
			}
		}

		db.logger.Info("Backed up %d queries for migration", len(backupQueries))

		// Drop the old tables to force recreation with new schema
		_, err1 := db.db.Exec("DROP TABLE IF EXISTS nqe_queries")
		_, err2 := db.db.Exec("DROP TABLE IF EXISTS db_metadata")
		if err1 != nil || err2 != nil {
			db.logger.Warn("Could not drop old tables: %v, %v", err1, err2)
		}

		// Schema will be recreated by initSchema() - we need to call it here first
		if err := db.initSchema(); err != nil {
			db.logger.Error("Failed to create new schema: %v", err)
			return fmt.Errorf("failed to create new schema: %w", err)
		}

		// Now restore the data with the new schema including instance_id
		if len(backupQueries) > 0 {
			tx, err := db.db.Begin()
			if err != nil {
				db.logger.Error("Failed to begin migration transaction: %v", err)
				return fmt.Errorf("failed to begin migration transaction: %w", err)
			}
			defer tx.Rollback()

			stmt, err := tx.Prepare(`
				INSERT INTO nqe_queries (
					instance_id, query_id, path, intent, source_code, description, repository,
					last_commit_id, last_commit_author, last_commit_date, last_commit_title,
					created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`)
			if err != nil {
				db.logger.Error("Failed to prepare migration statement: %v", err)
				return fmt.Errorf("failed to prepare migration statement: %w", err)
			}
			defer stmt.Close()

			for _, backup := range backupQueries {
				_, err := stmt.Exec(
					db.instanceID,
					backup.QueryID,
					backup.Path,
					backup.Intent.String,
					backup.SourceCode.String,
					backup.Description.String,
					backup.Repository.String,
					backup.LastCommitID.String,
					backup.LastCommitAuthor.String,
					backup.LastCommitDate.Int64,
					backup.LastCommitTitle.String,
					backup.CreatedAt,
					backup.UpdatedAt,
				)
				if err != nil {
					db.logger.Error("Failed to migrate query %s: %v", backup.QueryID, err)
					return fmt.Errorf("failed to migrate query %s: %w", backup.QueryID, err)
				}
			}

			if err := tx.Commit(); err != nil {
				db.logger.Error("Failed to commit migration: %v", err)
				return fmt.Errorf("failed to commit migration: %w", err)
			}

			db.logger.Info("Successfully migrated %d queries to new schema with instance ID '%s'", len(backupQueries), db.instanceID)
		}

		return nil
	}

	// Check if there are any rows without instance_id (partial migration)
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM nqe_queries WHERE instance_id IS NULL OR instance_id = ''").Scan(&count)
	if err != nil {
		db.logger.Debug("Could not count unmigrated rows: %v", err)
		return nil
	}

	if count == 0 {
		// No migration needed
		return nil
	}

	db.logger.Info("Migrating %d existing queries to instance-partitioned schema", count)

	// Update existing rows to have the current instance_id
	_, err = db.db.Exec("UPDATE nqe_queries SET instance_id = ? WHERE instance_id IS NULL OR instance_id = ''", db.instanceID)
	if err != nil {
		return fmt.Errorf("failed to migrate nqe_queries: %w", err)
	}

	// Update metadata table too (check if it has instance_id column first)
	var hasMetadataInstanceColumn int
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('db_metadata') WHERE name = 'instance_id'").Scan(&hasMetadataInstanceColumn)
	if err == nil && hasMetadataInstanceColumn > 0 {
		_, err = db.db.Exec("UPDATE db_metadata SET instance_id = ? WHERE instance_id IS NULL OR instance_id = ''", db.instanceID)
		if err != nil {
			return fmt.Errorf("failed to migrate db_metadata: %w", err)
		}
	}

	db.logger.Info("Successfully migrated existing data to instance-partitioned schema")
	return nil
}

// SaveQueries saves or updates queries in the database using upsert
func (db *NQEDatabase) SaveQueries(queries []forward.NQEQueryDetail) error {
	if len(queries) == 0 {
		return nil
	}

	tx, err := db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO nqe_queries (
			instance_id, query_id, path, intent, source_code, description, repository,
			last_commit_id, last_commit_author, last_commit_date, last_commit_title,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, query := range queries {
		_, err := stmt.Exec(
			db.instanceID,
			query.QueryID,
			query.Path,
			query.Intent,
			query.SourceCode,
			query.Description,
			query.Repository,
			query.LastCommit.ID,
			query.LastCommit.AuthorEmail,
			query.LastCommit.CommittedAt,
			query.LastCommit.Title,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert query %s: %w", query.QueryID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	db.logger.Debug("Saved %d queries to database", len(queries))

	// Notify callbacks that data has been updated
	db.notifyUpdateCallbacks()

	return nil
}

// LoadQueries loads all queries from the database for this instance
func (db *NQEDatabase) LoadQueries() ([]forward.NQEQueryDetail, error) {
	rows, err := db.db.Query(`
		SELECT query_id, path, intent, source_code, description, repository,
			   last_commit_id, last_commit_author, last_commit_date, last_commit_title
		FROM nqe_queries
		WHERE instance_id = ?
		ORDER BY path
	`, db.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	defer rows.Close()

	var queries []forward.NQEQueryDetail
	for rows.Next() {
		var query forward.NQEQueryDetail
		var commitID, authorEmail, title sql.NullString
		var committedAt sql.NullInt64

		err := rows.Scan(
			&query.QueryID,
			&query.Path,
			&query.Intent,
			&query.SourceCode,
			&query.Description,
			&query.Repository,
			&commitID,
			&authorEmail,
			&committedAt,
			&title,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Populate commit info if available
		query.LastCommit = forward.NQECommitInfo{
			ID:          commitID.String,
			AuthorEmail: authorEmail.String,
			CommittedAt: committedAt.Int64,
			Title:       title.String,
		}

		queries = append(queries, query)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return queries, nil
}

// GetQueryCount returns the number of queries in the database for this instance
func (db *NQEDatabase) GetQueryCount() (int, error) {
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM nqe_queries WHERE instance_id = ?", db.instanceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count queries: %w", err)
	}
	return count, nil
}

// GetStatistics returns database statistics
func (db *NQEDatabase) GetStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total queries
	totalQueries, err := db.GetQueryCount()
	if err != nil {
		return nil, err
	}
	stats["total_queries"] = totalQueries

	// Queries by repository
	rows, err := db.db.Query(`
		SELECT repository, COUNT(*) 
		FROM nqe_queries 
		WHERE instance_id = ? AND repository IS NOT NULL AND repository != ''
		GROUP BY repository
	`, db.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query repository stats: %w", err)
	}
	defer rows.Close()

	repoStats := make(map[string]int)
	for rows.Next() {
		var repo string
		var count int
		if err := rows.Scan(&repo, &count); err != nil {
			continue
		}
		repoStats[repo] = count
	}
	stats["repositories"] = repoStats

	// Last sync time
	var lastSync sql.NullString
	err = db.db.QueryRow("SELECT value FROM db_metadata WHERE instance_id = ? AND key = 'last_sync'", db.instanceID).Scan(&lastSync)
	if err == nil && lastSync.Valid {
		stats["last_sync"] = lastSync.String
	} else {
		stats["last_sync"] = "never"
	}

	return stats, nil
}

// SetMetadata stores metadata in the database for this instance
func (db *NQEDatabase) SetMetadata(key, value string) error {
	_, err := db.db.Exec(`
		INSERT OR REPLACE INTO db_metadata (instance_id, key, value, updated_at)
		VALUES (?, ?, ?, ?)
	`, db.instanceID, key, value, time.Now().Unix())

	if err != nil {
		return fmt.Errorf("failed to set metadata %s: %w", key, err)
	}
	return nil
}

// GetMetadata retrieves metadata from the database
func (db *NQEDatabase) GetMetadata(key string) (string, error) {
	var value string
	err := db.db.QueryRow("SELECT value FROM db_metadata WHERE instance_id = ? AND key = ?", db.instanceID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get metadata %s: %w", key, err)
	}
	return value, nil
}

// Close closes the database connection
func (db *NQEDatabase) Close() error {
	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

// loadWithSmartCaching implements the smart caching strategy
func (db *NQEDatabase) loadWithSmartCaching(client forward.ClientInterface, logger *logger.Logger) ([]forward.NQEQueryDetail, error) {
	return db.loadWithSmartCachingContext(context.Background(), client, logger)
}

// loadWithSmartCachingContext implements the smart caching strategy with context support
func (db *NQEDatabase) loadWithSmartCachingContext(ctx context.Context, client forward.ClientInterface, logger *logger.Logger) ([]forward.NQEQueryDetail, error) {
	logger.Info("Starting smart caching query load...")

	// Step 1: Load existing queries from database for immediate availability
	existingQueries, err := db.LoadQueries()
	if err != nil {
		logger.Debug("Failed to load from database: %v", err)
		existingQueries = []forward.NQEQueryDetail{} // Start with empty if database fails
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

// backgroundEnhancedLoad runs Enhanced API loading in the background
func (db *NQEDatabase) backgroundEnhancedLoad(client forward.ClientInterface, logger *logger.Logger, existingQueries []forward.NQEQueryDetail) {
	logger.Info("🔄 Background Enhanced API loading started...")

	// Build commit ID map for incremental updates
	existingCommitIDs := make(map[string]string)
	for _, query := range existingQueries {
		if query.LastCommit.ID != "" {
			existingCommitIDs[query.Path] = query.LastCommit.ID
		}
	}

	// Try Enhanced API with extended timeout for background operation
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 minutes for background
	defer cancel()

	// Try Enhanced API loading
	enhancedQueries, err := client.GetNQEAllQueriesEnhancedWithCacheContext(ctx, existingCommitIDs)
	if err != nil {
		logger.Warn("🔄 Background Enhanced API failed: %v", err)
		logger.Info("🔄 Background fallback to Basic API...")

		// Fallback to Basic API in background
		basicQueries, err := db.loadFromBasicAPI(client, logger)
		if err != nil {
			logger.Error("🔄 Background Basic API also failed: %v", err)
			return
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

// backgroundEnhancedLoadWithContext runs Enhanced API loading in the background with context support
func (db *NQEDatabase) backgroundEnhancedLoadWithContext(ctx context.Context, client forward.ClientInterface, logger *logger.Logger, existingQueries []forward.NQEQueryDetail) {
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
	enhancedQueries, err := client.GetNQEAllQueriesEnhancedWithCacheContext(ctx, existingCommitIDs)
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
		basicQueries, err := db.loadFromBasicAPI(client, logger)
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
func (db *NQEDatabase) synchronousLoad(ctx context.Context, client forward.ClientInterface, logger *logger.Logger, existingQueries []forward.NQEQueryDetail) ([]forward.NQEQueryDetail, error) {
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
	basicQueries, err := db.loadFromBasicAPI(client, logger)
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
func (db *NQEDatabase) loadFromEnhancedAPIWithCommitCheck(ctx context.Context, client forward.ClientInterface, logger *logger.Logger, existingCommitIDs map[string]string) ([]forward.NQEQueryDetail, error) {
	// Channel to receive results
	resultChan := make(chan []forward.NQEQueryDetail, 1)
	errorChan := make(chan error, 1)

	// Start Enhanced API loading in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errorChan <- fmt.Errorf("enhanced API panic: %v", r)
			}
		}()

		// Load from both repositories with commit checking
		allQueries, err := client.GetNQEAllQueriesEnhancedWithCache(existingCommitIDs)
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
func (db *NQEDatabase) loadFromBasicAPI(client forward.ClientInterface, logger *logger.Logger) ([]forward.NQEQueryDetail, error) {
	var allQueries []forward.NQEQueryDetail

	// Load from org repository
	logger.Info("Loading queries from org repository...")
	orgQueries, err := client.GetNQEOrgQueries()
	if err != nil {
		logger.Error("Failed to load org queries: %v", err)
	} else {
		logger.Info("Loaded %d queries from org repository", len(orgQueries))
		// Convert NQEQuery to NQEQueryDetail
		for _, query := range orgQueries {
			detail := forward.NQEQueryDetail{
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
	fwdQueries, err := client.GetNQEFwdQueries()
	if err != nil {
		logger.Error("Failed to load fwd queries: %v", err)
	} else {
		logger.Info("Loaded %d queries from fwd repository", len(fwdQueries))
		// Convert NQEQuery to NQEQueryDetail
		for _, query := range fwdQueries {
			detail := forward.NQEQueryDetail{
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
func (db *NQEDatabase) mergeQueries(existing, new []forward.NQEQueryDetail) []forward.NQEQueryDetail {
	queryMap := make(map[string]forward.NQEQueryDetail)

	// Add existing queries
	for _, query := range existing {
		queryMap[query.QueryID] = query
	}

	// Add/update with new queries (overwrites existing)
	for _, query := range new {
		queryMap[query.QueryID] = query
	}

	// Convert back to slice
	var result []forward.NQEQueryDetail
	for _, query := range queryMap {
		result = append(result, query)
	}

	return result
}

// deduplicateQueries removes duplicate queries by QueryID
func (db *NQEDatabase) deduplicateQueries(queries []forward.NQEQueryDetail) []forward.NQEQueryDetail {
	seen := make(map[string]bool)
	var unique []forward.NQEQueryDetail

	for _, query := range queries {
		if !seen[query.QueryID] {
			seen[query.QueryID] = true
			unique = append(unique, query)
		}
	}

	return unique
}
