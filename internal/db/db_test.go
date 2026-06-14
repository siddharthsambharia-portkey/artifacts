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
