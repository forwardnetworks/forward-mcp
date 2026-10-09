package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

var testRows = []map[string]interface{}{
	{"device": "r1", "vendor": "cisco", "ifaces": 48},
	{"device": "r2", "vendor": "juniper", "ifaces": 24},
	{"device": "r3", "vendor": "cisco", "ifaces": 12},
}

func TestQueryRowsSelect(t *testing.T) {
	got, err := RowQuerier{}.QueryRows(context.Background(), testRows,
		"SELECT device FROM nqe_result WHERE vendor = 'cisco' ORDER BY device LIMIT 100")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(got) != 2 || got[0]["device"] != "r1" || got[1]["device"] != "r3" {
		t.Errorf("got %v, want devices r1 and r3", got)
	}
}

func TestQueryRowsAggregate(t *testing.T) {
	got, err := RowQuerier{}.QueryRows(context.Background(), testRows,
		"SELECT vendor, COUNT(*) AS n FROM nqe_result GROUP BY vendor ORDER BY vendor")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(got) != 2 || got[0]["vendor"] != "cisco" || got[0]["n"] != int64(2) {
		t.Errorf("got %v, want cisco=2 first", got)
	}
}

func TestQueryRowsBadSQL(t *testing.T) {
	if _, err := (RowQuerier{}).QueryRows(context.Background(), testRows, "SELEC nonsense"); err == nil {
		t.Error("want an error for invalid SQL")
	}
}

// The model writes the SQL, and prompt injection can steer the model. A query
// must not reach the file system: ATTACH creates a database file wherever the
// server process may write.
func TestQueryRowsCannotTouchFiles(t *testing.T) {
	target := filepath.Join(t.TempDir(), "pwned.db")
	for _, q := range []string{
		"ATTACH DATABASE '" + target + "' AS p",
		"SELECT 1; ATTACH DATABASE '" + target + "' AS p",
		"VACUUM INTO '" + target + "'",
	} {
		_, err := RowQuerier{}.QueryRows(context.Background(), testRows, q)
		if err == nil {
			t.Errorf("%q: want an error, got none", q)
		}
		if _, statErr := os.Stat(target); statErr == nil {
			t.Fatalf("%q created %s", q, target)
		}
	}
}

// Column names come from the API data. They must be quoted, not pasted into
// the CREATE TABLE statement.
func TestQueryRowsOddColumnNames(t *testing.T) {
	rows := []map[string]interface{}{{"interface name": "Gi0/1", "order": "1", `a"b`: "x"}}
	got, err := RowQuerier{}.QueryRows(context.Background(), rows, `SELECT "interface name", "order", "a""b" FROM nqe_result`)
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(got) != 1 || got[0]["interface name"] != "Gi0/1" || got[0]["order"] != "1" || got[0][`a"b`] != "x" {
		t.Errorf("got %v", got)
	}
}

// Read-only still allows real analysis, recursive CTEs included.
func TestQueryRowsAllowsReadOnlySQL(t *testing.T) {
	for q, want := range map[string]int{
		"WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 3) SELECT x FROM c": 3,
		"SELECT upper(device) AS d, length(vendor) AS l FROM nqe_result WHERE ifaces > 20":            2,
		"SELECT * FROM nqe_result a JOIN nqe_result b ON a.vendor = b.vendor":                         5,
	} {
		got, err := RowQuerier{}.QueryRows(context.Background(), testRows, q)
		if err != nil {
			t.Errorf("%q: %v", q, err)
			continue
		}
		if len(got) != want {
			t.Errorf("%q: got %d rows, want %d", q, len(got), want)
		}
	}
}

func TestQueryRowsRefusesWrites(t *testing.T) {
	for _, q := range []string{
		"DELETE FROM nqe_result",
		"UPDATE nqe_result SET vendor = 'x'",
		"INSERT INTO nqe_result (device) VALUES ('evil')",
		"CREATE TABLE t (x)",
		"DROP TABLE nqe_result",
		"PRAGMA table_info(nqe_result)",
	} {
		if _, err := (RowQuerier{}).QueryRows(context.Background(), testRows, q); err == nil {
			t.Errorf("%q: want an error, got none", q)
		}
	}
}
