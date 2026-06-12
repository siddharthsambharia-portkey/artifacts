package auth

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

func TestGroupsJSON(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		want   string
	}{
		{"empty slice", []string{}, "[]"},
		{"single group", []string{"employees"}, `["employees"]`},
		{"two groups", []string{"employees", "admins"}, `["employees","admins"]`},
		{"group with comma", []string{"a,b"}, `["a,b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := groupsJSON(tt.groups); got != tt.want {
				t.Fatalf("groupsJSON(%v) = %q, want %q", tt.groups, got, tt.want)
			}
		})
	}
}

func TestParseGroups(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty array literal", "[]", []string{"[]"}},
		{"single group", `["employees"]`, []string{`["employees"]`}},
		{
			name: "two groups split wrongly",
			// BUG: hand-rolled parser mangles multi-group arrays
			raw:  `["employees","admins"]`,
			want: []string{`["employees`, `admins"]`},
		},
		{"group with comma preserved in one token", `["a,b"]`, []string{`["a,b"]`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGroups(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseGroups(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseGroupsEmptyString(t *testing.T) {
	got := parseGroups("")
	if len(got) != 0 {
		t.Fatalf("parseGroups(\"\") = %v, want empty slice", got)
	}
}

func TestSessionStoreCreateAndGet(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())

	store := NewSessionStore(database)
	user := &User{Email: "alice@co", Name: "Alice", Groups: []string{"employees", "admins"}}
	ctx := context.Background()

	id, err := store.Create(ctx, user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != user.Email || got.Name != user.Name {
		t.Fatalf("user mismatch: got %+v", got)
	}
	wantGroups := []string{`["employees`, `admins"]`}
	if !reflect.DeepEqual(got.Groups, wantGroups) {
		t.Fatalf("groups = %v, want %v (today's parseGroups behavior)", got.Groups, wantGroups)
	}
}

func TestSessionStoreGetUnknownID(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())

	store := NewSessionStore(database)
	_, err = store.Get(context.Background(), "nonexistent-session-id")
	if err == nil {
		t.Fatal("expected error for unknown session id")
	}
}

func TestSessionStoreExpiredSession(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())

	store := NewSessionStore(database)
	user := &User{Email: "bob@co", Name: "Bob", Groups: []string{"employees"}}
	ctx := context.Background()

	id, err := store.Create(ctx, user, -1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, id)
	// BUG: expired session returns nil error and nil user — plan 002 changes this to a non-nil error.
	if got != nil {
		t.Fatalf("expired session user = %v, want nil", got)
	}
	if err != nil {
		t.Fatalf("expired session err = %v, want nil (today's behavior)", err)
	}
}
