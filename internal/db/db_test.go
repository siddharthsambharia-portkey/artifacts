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

func TestCountAIUsageSince(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	const email = "alice@example.com"
	now := time.Now()

	usages := []AIUsage{
		{UserEmail: email, Site: "s1", Timestamp: now},
		{UserEmail: email, Site: "s2", Timestamp: now.Add(-1 * time.Hour)},
		{UserEmail: email, Site: "s3", Timestamp: now.Add(-25 * time.Hour)},
		{UserEmail: "bob@example.com", Site: "s1", Timestamp: now},
	}
	for i := range usages {
		if err := d.InsertAIUsage(ctx, &usages[i]); err != nil {
			t.Fatal(err)
		}
	}

	cutoff := now.Add(-24 * time.Hour)
	n, err := d.CountAIUsageSince(ctx, email, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want 2, got %d", n)
	}
}

func TestListAIUsageSummary(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	usages := []AIUsage{
		{UserEmail: "alice@example.com", Site: "s1", Timestamp: now},
		{UserEmail: "alice@example.com", Site: "s1", Timestamp: now},
		{UserEmail: "alice@example.com", Site: "s1", Timestamp: now},
		{UserEmail: "bob@example.com", Site: "s2", Timestamp: now},
		{UserEmail: "bob@example.com", Site: "s2", Timestamp: now},
	}
	for i := range usages {
		if err := d.InsertAIUsage(ctx, &usages[i]); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := d.ListAIUsageSummary(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary) != 2 {
		t.Fatalf("want 2 rows, got %d", len(summary))
	}
	if summary[0].UserEmail != "alice@example.com" || summary[0].Requests != 3 {
		t.Fatalf("first row: want alice/3, got %s/%d", summary[0].UserEmail, summary[0].Requests)
	}
	if summary[1].UserEmail != "bob@example.com" || summary[1].Requests != 2 {
		t.Fatalf("second row: want bob/2, got %s/%d", summary[1].UserEmail, summary[1].Requests)
	}
}
