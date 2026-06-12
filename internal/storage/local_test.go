package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestFullPath(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		wantEmpty bool
	}{
		{"normal relative path", "sites/demo/file.txt", false},
		{"parent escape", "../escape", true},
		{"nested escape", "a/../../escape", true},
		{"absolute path joins under root", "/etc/passwd", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.fullPath(tt.input)
			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("fullPath(%q) = %q, want empty", tt.input, got)
				}
				return
			}
			if got == "" {
				t.Fatalf("fullPath(%q) = empty, want non-empty", tt.input)
			}
			if !strings.HasPrefix(got, tmp) {
				t.Fatalf("fullPath(%q) = %q, want under root %q", tt.input, got, tmp)
			}
		})
	}
}

func TestPutGetRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	content := "artifact storage test"
	path := "uploads/demo/file.txt"
	if err := store.Put(ctx, path, strings.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	rc, info, err := store.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if info.ETag == "" {
		t.Fatal("expected non-empty ETag")
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("content = %q, want %q", got, content)
	}
}
