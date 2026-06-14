package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	d := &DB{DB: sqlDB, driver: "sqlite"}
	if err := d.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCountWarehouseQueriesSince(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	old := now.Add(-2 * time.Hour)
	recent := now.Add(-30 * time.Minute)

	entries := []AuditEntry{
		{Timestamp: old, UserEmail: "a@x.com", Site: "s1", Action: "warehouse_query", Detail: "SELECT 1"},
		{Timestamp: recent, UserEmail: "b@x.com", Site: "s1", Action: "warehouse_query", Detail: "SELECT 2"},
		{Timestamp: recent, UserEmail: "c@x.com", Site: "s1", Action: "warehouse_query", Detail: "SELECT 3"},
		{Timestamp: recent, UserEmail: "a@x.com", Site: "s1", Action: "deploy", Detail: "not a query"},
	}
	for i := range entries {
		if err := d.InsertAudit(ctx, &entries[i]); err != nil {
			t.Fatalf("InsertAudit: %v", err)
		}
	}

	tests := []struct {
		name   string
		cutoff time.Time
		want   int
	}{
		{
			name:   "cutoff before all entries counts all warehouse_query rows",
			cutoff: now.Add(-3 * time.Hour),
			want:   3,
		},
		{
			name:   "cutoff between old and recent excludes the old row",
			cutoff: now.Add(-1 * time.Hour),
			want:   2,
		},
		{
			name:   "cutoff after all entries counts zero",
			cutoff: now.Add(1 * time.Hour),
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.CountWarehouseQueriesSince(ctx, tt.cutoff)
			if err != nil {
				t.Fatalf("CountWarehouseQueriesSince: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
