package warehouse

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestScanRowsEnforcesRowCap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE t (id INTEGER); INSERT INTO t VALUES (1),(2),(3),(4),(5)`); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`SELECT id FROM t ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	result, err := scanRows(rows, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("scanRows cap: got %d rows, want 3", len(result))
	}
}

func TestPostgresQueryEnforcesRowCapDespiteHighLimit(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE t (id INTEGER); INSERT INTO t VALUES (1),(2),(3),(4),(5)`); err != nil {
		t.Fatal(err)
	}

	p := &postgresQuerier{db: db}
	result, err := p.Query(context.Background(), "SELECT id FROM t ORDER BY id LIMIT 999999", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("Query cap: got %d rows, want 3", len(result))
	}
}
