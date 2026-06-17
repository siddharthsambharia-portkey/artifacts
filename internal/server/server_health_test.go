package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

func TestHealthzIsDepFree(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Artifact-Version", ServerVersion)
		w.Write([]byte("ok"))
	}
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", body)
	}
}

func TestReadyzOK(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	s := &Server{db: database}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	s.readyzHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", body)
	}
	if v := w.Header().Get("X-Artifact-Version"); v != ServerVersion {
		t.Fatalf("expected X-Artifact-Version %q, got %q", ServerVersion, v)
	}
}

func TestReadyzDBUnavailable(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Close the underlying connection so the ping will fail.
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	s := &Server{db: database}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	s.readyzHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body %s", w.Code, w.Body.String())
	}
}
