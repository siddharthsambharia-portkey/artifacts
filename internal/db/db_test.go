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
	d := &DB{db: sqlDB, driver: "sqlite"}
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

func TestInsertFile(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	f := &FileRecord{
		ID:          "file-001",
		Site:        "mysite",
		Filename:    "photo.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		StoragePath: "uploads/mysite/file-001.png",
		UploadedBy:  "user@example.com",
		UploadedAt:  now,
	}
	if err := d.InsertFile(ctx, f); err != nil {
		t.Fatalf("InsertFile: %v", err)
	}
}

func TestListFiles(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	files := []FileRecord{
		{ID: "f1", Site: "site1", Filename: "a.png", ContentType: "image/png", SizeBytes: 100, StoragePath: "uploads/site1/f1.png", UploadedBy: "u@x.com", UploadedAt: now.Add(-2 * time.Second)},
		{ID: "f2", Site: "site1", Filename: "b.pdf", ContentType: "application/pdf", SizeBytes: 200, StoragePath: "uploads/site1/f2.pdf", UploadedBy: "u@x.com", UploadedAt: now.Add(-1 * time.Second)},
		{ID: "f3", Site: "site2", Filename: "other.png", ContentType: "image/png", SizeBytes: 50, StoragePath: "uploads/site2/f3.png", UploadedBy: "u@x.com", UploadedAt: now},
	}
	for i := range files {
		if err := d.InsertFile(ctx, &files[i]); err != nil {
			t.Fatalf("InsertFile: %v", err)
		}
	}

	got, err := d.ListFiles(ctx, "site1", 100)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListFiles(site1) returned %d records, want 2", len(got))
	}
	if got[0].ID != "f2" {
		t.Errorf("first result ID = %q, want f2 (newest first)", got[0].ID)
	}
	if got[1].ID != "f1" {
		t.Errorf("second result ID = %q, want f1", got[1].ID)
	}
}

func TestGetFileByID(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	f := &FileRecord{
		ID:          "getme",
		Site:        "testsite",
		Filename:    "doc.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512,
		StoragePath: "uploads/testsite/getme.pdf",
		UploadedBy:  "u@x.com",
		UploadedAt:  now,
	}
	if err := d.InsertFile(ctx, f); err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	got, err := d.GetFileByID(ctx, "testsite", "getme")
	if err != nil {
		t.Fatalf("GetFileByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetFileByID returned nil, want record")
	}
	if got.StoragePath != f.StoragePath {
		t.Errorf("StoragePath = %q, want %q", got.StoragePath, f.StoragePath)
	}
	if got.Filename != "doc.pdf" {
		t.Errorf("Filename = %q, want doc.pdf", got.Filename)
	}

	missing, err := d.GetFileByID(ctx, "testsite", "nope")
	if err != nil {
		t.Fatalf("GetFileByID missing: %v", err)
	}
	if missing != nil {
		t.Error("GetFileByID for missing id should return nil")
	}

	crossSite, err := d.GetFileByID(ctx, "othersite", "getme")
	if err != nil {
		t.Fatalf("GetFileByID cross-site: %v", err)
	}
	if crossSite != nil {
		t.Error("GetFileByID should not return record from different site")
	}
}
