package sqlite

import (
	"context"
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
