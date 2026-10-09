package ports

import "context"

// RowQuerier runs one SQL query over the rows of an NQE result. The rows are
// loaded into a table named nqe_result whose columns are the keys of the first
// row.
type RowQuerier interface {
	QueryRows(ctx context.Context, rows []map[string]interface{}, query string) ([]map[string]interface{}, error)
}
