package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/forward-mcp/internal/ports"
	sqlite3 "github.com/mattn/go-sqlite3"
)

// RowQuerier runs SQL over NQE result rows in a throwaway in-memory database.
//
// The query text comes from the model, and prompt injection in network data
// can steer the model, so the query runs read-only: once the rows are loaded,
// an authorizer lets SQLite read and compute but refuses every other
// operation, including ATTACH, PRAGMA and any write.
type RowQuerier struct{}

// RowQuerier implements the RowQuerier port.
var _ ports.RowQuerier = RowQuerier{}

const resultTable = "nqe_result"

// QueryRows loads data into a table named nqe_result and runs query on it.
func (RowQuerier) QueryRows(ctx context.Context, data []map[string]interface{}, query string) ([]map[string]interface{}, error) {
	// Create in-memory SQLite DB. An in-memory database belongs to one
	// connection, so every statement below runs on the same one.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory sqlite db: %w", err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory sqlite db: %w", err)
	}
	defer conn.Close()

	// Infer columns from the first row, in a stable order. Column names come
	// from API data, so they are quoted, never pasted into SQL.
	var columns []string
	for k := range data[0] {
		columns = append(columns, k)
	}
	sort.Strings(columns)
	quoted := make([]string, len(columns))
	for i, col := range columns {
		quoted[i] = quoteIdent(col)
	}

	createStmt := fmt.Sprintf("CREATE TABLE %s (%s TEXT);", resultTable, strings.Join(quoted, " TEXT, "))
	if _, err := conn.ExecContext(ctx, createStmt); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	insertStmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", resultTable, strings.Join(quoted, ", "),
		strings.TrimRight(strings.Repeat("?,", len(columns)), ","))
	for _, row := range data {
		vals := make([]interface{}, len(columns))
		for i, col := range columns {
			if v, ok := row[col]; ok {
				vals[i] = fmt.Sprintf("%v", v)
			}
		}
		if _, err := conn.ExecContext(ctx, insertStmt, vals...); err != nil {
			return nil, fmt.Errorf("failed to insert row: %w", err)
		}
	}

	// From here on the connection is read-only.
	if err := conn.Raw(lockDown); err != nil {
		return nil, fmt.Errorf("failed to make the query read-only: %w", err)
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("SQL query error: %w", err)
	}
	defer rows.Close()

	resultRows := []map[string]interface{}{}
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		if err := rows.Scan(valPtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		rowMap := map[string]interface{}{}
		for i, col := range cols {
			rowMap[col] = vals[i]
		}
		resultRows = append(resultRows, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SQL query error: %w", err)
	}
	return resultRows, nil
}

// quoteIdent quotes a column name for SQLite.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// sqliteRecursive is SQLITE_RECURSIVE, which go-sqlite3 does not export.
const sqliteRecursive = 33

// deniedFunctions touch files or load code. Most are absent from this build;
// they are refused in case a later build includes them.
var deniedFunctions = map[string]bool{
	"load_extension": true,
	"readfile":       true,
	"writefile":      true,
	"edit":           true,
	"fts3_tokenizer": true,
}

// lockDown makes a raw SQLite connection read-only: no attached databases,
// and an authorizer that allows only reading and computing.
func lockDown(driverConn interface{}) error {
	c, ok := driverConn.(*sqlite3.SQLiteConn)
	if !ok {
		return fmt.Errorf("unexpected driver connection %T", driverConn)
	}
	c.SetLimit(sqlite3.SQLITE_LIMIT_ATTACHED, 0)
	c.RegisterAuthorizer(func(op int, _, arg2, _ string) int {
		switch op {
		case sqlite3.SQLITE_SELECT, sqlite3.SQLITE_READ, sqliteRecursive:
			return sqlite3.SQLITE_OK
		case sqlite3.SQLITE_FUNCTION:
			if deniedFunctions[strings.ToLower(arg2)] {
				return sqlite3.SQLITE_DENY
			}
			return sqlite3.SQLITE_OK
		default:
			return sqlite3.SQLITE_DENY
		}
	})
	return nil
}
