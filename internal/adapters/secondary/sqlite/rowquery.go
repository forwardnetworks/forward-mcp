package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/forward-mcp/internal/ports"
)

// RowQuerier runs SQL over NQE result rows in a throwaway in-memory database.
type RowQuerier struct{}

// RowQuerier implements the RowQuerier port.
var _ ports.RowQuerier = RowQuerier{}

// QueryRows loads data into a table named nqe_result and runs query on it.
func (RowQuerier) QueryRows(ctx context.Context, data []map[string]interface{}, query string) ([]map[string]interface{}, error) {
	// Create in-memory SQLite DB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory sqlite db: %w", err)
	}
	defer db.Close()
	// Infer columns from first row
	firstRow := data[0]
	var columns []string
	for k := range firstRow {
		columns = append(columns, k)
	}
	// Create table
	tableCols := ""
	for i, col := range columns {
		if i > 0 {
			tableCols += ", "
		}
		tableCols += fmt.Sprintf("%s TEXT", col)
	}
	tableName := "nqe_result"
	createStmt := fmt.Sprintf("CREATE TABLE %s (%s);", tableName, tableCols)
	_, err = db.ExecContext(ctx, createStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	// Insert rows
	insertStmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(columns, ", "), strings.TrimRight(strings.Repeat("?,", len(columns)), ","))
	for _, row := range data {
		vals := make([]interface{}, len(columns))
		for i, col := range columns {
			if v, ok := row[col]; ok {
				vals[i] = fmt.Sprintf("%v", v)
			} else {
				vals[i] = nil
			}
		}
		_, err := db.ExecContext(ctx, insertStmt, vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert row: %w", err)
		}
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("SQL query error: %w", err)
	}
	defer rows.Close()
	// Read results
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
	return resultRows, nil
}
