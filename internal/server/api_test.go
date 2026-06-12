package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/realtime"
)

func TestAPICreateAndList(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())
	hub := realtime.NewHub(cfg)
	api := NewAPI(cfg, database, governance.New(cfg), hub)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"create entry", "POST", "/api/v1/db/entries", `{"message":"hi"}`, http.StatusOK},
		{"invalid json", "POST", "/api/v1/db/entries", `{bad`, http.StatusBadRequest},
		{"list entries", "GET", "/api/v1/db/entries", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(tt.body)))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			req.Host = "guestbook.localhost:8443"
			req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
			w := httptest.NewRecorder()
			switch tt.method {
			case "POST":
				api.handleCreate(w, req)
			case "GET":
				api.handleList(w, req)
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestKVGovernedMode(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.Governance.Mode = "governed"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())
	if err := database.UpsertSite(context.Background(), &db.SiteRecord{
		Name: "private-site", Owner: "alice@co", Visibility: "private",
	}); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(cfg, database, governance.New(cfg), nil)

	alice := &auth.User{Email: "alice@co", Groups: []string{"employees"}}
	bob := &auth.User{Email: "bob@co", Groups: []string{"employees"}}

	t.Run("kv set non-owner denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/kv/theme", bytes.NewReader([]byte(`{"value":"dark"}`)))
		req.Host = "private-site.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), bob))
		w := httptest.NewRecorder()
		api.handleKVSet(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})

	t.Run("kv set owner allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/kv/theme", bytes.NewReader([]byte(`{"value":"dark"}`)))
		req.Host = "private-site.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), alice))
		w := httptest.NewRecorder()
		api.handleKVSet(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})

	t.Run("kv get non-owner denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/kv/theme", nil)
		req.Host = "private-site.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), bob))
		w := httptest.NewRecorder()
		api.handleKVGet(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})
}

func TestKVGroupScopedVisibility(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.Governance.Mode = "governed"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())
	if err := database.UpsertSite(context.Background(), &db.SiteRecord{
		Name: "hr-site", Owner: "alice@co", Visibility: "group",
		VisibilityGroups: []string{"hr-team"},
		DeployID: "d1", DeployedBy: "alice@co", DeployedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(cfg, database, governance.New(cfg), nil)

	alice := &auth.User{Email: "alice@co", Groups: []string{"employees"}}
	carol := &auth.User{Email: "carol@co", Groups: []string{"hr-team"}}
	bob := &auth.User{Email: "bob@co", Groups: []string{"employees"}}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/kv/secret", bytes.NewReader([]byte(`{"value":"x"}`)))
	req.Host = "hr-site.localhost:8443"
	req = req.WithContext(auth.WithUser(req.Context(), alice))
	w := httptest.NewRecorder()
	api.handleKVSet(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("seed kv: status %d body %s", w.Code, w.Body.String())
	}

	t.Run("in-group non-owner allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/kv/secret", nil)
		req.Host = "hr-site.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), carol))
		w := httptest.NewRecorder()
		api.handleKVGet(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})

	t.Run("out-of-group denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/kv/secret", nil)
		req.Host = "hr-site.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), bob))
		w := httptest.NewRecorder()
		api.handleKVGet(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})
}

func TestGenerateIDUnique(t *testing.T) {
	a := generateID()
	b := generateID()
	if a == b {
		t.Fatal("expected unique ids")
	}
}
