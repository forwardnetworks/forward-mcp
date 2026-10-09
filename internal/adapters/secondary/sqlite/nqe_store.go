// Package sqlite holds the SQLite adapters: the NQE query store and the
// knowledge-graph memory store, both under ~/.forward-mcp/data.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forward-mcp/internal/ports"
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
		// Security: Use restrictive permissions (owner-only access)
		if err := os.MkdirAll(dir, 0700); err == nil {
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
	logger          ports.Logger
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
func NewNQEDatabase(logger ports.Logger, instanceID string) (*NQEDatabase, error) {
	// Get a writable directory for the database
	dataDir, err := getWritableDataDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to determine writable data directory: %w", err)
	}

	logger.Info("Using database directory: %s", dataDir)

	// Create data directory if it doesn't exist
	// Security: Use restrictive permissions (owner-only access)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
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
func (db *NQEDatabase) SaveQueries(queries []ports.NQEQueryDetail) error {
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
func (db *NQEDatabase) LoadQueries() ([]ports.NQEQueryDetail, error) {
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

	var queries []ports.NQEQueryDetail
	for rows.Next() {
		var query ports.NQEQueryDetail
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
		query.LastCommit = ports.NQECommitInfo{
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

// GetAllInstanceIDs returns all instance IDs that have queries in the database
func (db *NQEDatabase) GetAllInstanceIDs() ([]ports.InstanceInfo, error) {
	rows, err := db.db.Query(`
		SELECT 
			instance_id,
			COUNT(*) as query_count,
			MAX(updated_at) as last_sync,
			MIN(created_at) as first_sync
		FROM nqe_queries 
		GROUP BY instance_id 
		ORDER BY last_sync DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query instance IDs: %w", err)
	}
	defer rows.Close()

	var instances []ports.InstanceInfo
	for rows.Next() {
		var instance ports.InstanceInfo
		var lastSync, firstSync int64
		err := rows.Scan(&instance.ID, &instance.QueryCount, &lastSync, &firstSync)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance info: %w", err)
		}

		instance.LastSync = time.Unix(lastSync, 0)
		instance.FirstSync = time.Unix(firstSync, 0)
		instances = append(instances, instance)
	}

	return instances, nil
}

// Close closes the database connection
func (db *NQEDatabase) Close() error {
	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

// Path is the SQLite file this store uses.
func (db *NQEDatabase) Path() string { return db.dbPath }

// NQEDatabase implements the QueryStore port.
var _ ports.QueryStore = (*NQEDatabase)(nil)
