package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
)

func setupAdminTestDB(t *testing.T) (*db.DB, *config.Config) {
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
	return database, cfg
}

func withSiteParam(req *http.Request, site string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("site", site)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestSetVisibility(t *testing.T) {
	database, cfg := setupAdminTestDB(t)
	handler := NewHandler(cfg, database, governance.New(cfg))
	now := time.Now()
	if err := database.UpsertSite(context.Background(), &db.SiteRecord{
		Name: "demo", Owner: "alice@co", Visibility: "private",
		DeployID: "d1", DeployedBy: "alice@co", DeployedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("non-admin forbidden", func(t *testing.T) {
		body := `{"visibility":"public"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/sites/demo/visibility", bytes.NewReader([]byte(body)))
		req = withSiteParam(req, "demo")
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Email: "bob@co", Groups: []string{"employees"}}))
		w := httptest.NewRecorder()
		handler.SetVisibility(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("admin sets group visibility", func(t *testing.T) {
		body := `{"visibility":"group","groups":["hr-team","finance"]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/sites/demo/visibility", bytes.NewReader([]byte(body)))
		req = withSiteParam(req, "demo")
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Email: "admin@co", Groups: []string{"admins"}}))
		w := httptest.NewRecorder()
		handler.SetVisibility(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
		site, err := database.GetSite(context.Background(), "demo")
		if err != nil || site == nil {
			t.Fatal(err)
		}
		if site.Visibility != "group" {
			t.Fatalf("visibility %q", site.Visibility)
		}
		want := []string{"hr-team", "finance"}
		if len(site.VisibilityGroups) != len(want) {
			t.Fatalf("groups %v", site.VisibilityGroups)
		}
		for i, g := range want {
			if site.VisibilityGroups[i] != g {
				t.Fatalf("groups %v want %v", site.VisibilityGroups, want)
			}
		}
	})

	t.Run("invalid visibility", func(t *testing.T) {
		body := `{"visibility":"secret"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/sites/demo/visibility", bytes.NewReader([]byte(body)))
		req = withSiteParam(req, "demo")
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Email: "admin@co", Groups: []string{"admins"}}))
		w := httptest.NewRecorder()
		handler.SetVisibility(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("group with empty groups", func(t *testing.T) {
		body := `{"visibility":"group","groups":[]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/sites/demo/visibility", bytes.NewReader([]byte(body)))
		req = withSiteParam(req, "demo")
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Email: "admin@co", Groups: []string{"admins"}}))
		w := httptest.NewRecorder()
		handler.SetVisibility(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d", w.Code)
		}
	})
}
