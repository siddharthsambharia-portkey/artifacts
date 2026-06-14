package server

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/siddharthsambharia-portkey/artifacts/internal/realtime"
)

type fakePublisher struct {
	events []struct {
		site, collection, eventType string
		doc                         any
	}
}

func (f *fakePublisher) PublishDBEvent(e realtime.DBEvent) {}
func (f *fakePublisher) PublishDocumentEvent(site, collection, eventType string, doc any) {
	f.events = append(f.events, struct {
		site, collection, eventType string
		doc                         any
	}{site, collection, eventType, doc})
}

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

func TestAPICreateOversizedBody(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())
	api := NewAPI(cfg, database, governance.New(cfg), nil)

	body := make([]byte, 1<<20+1)
	for i := range body {
		body[i] = 'a'
	}
	payload := append([]byte(`{"x":"`), body...)
	payload = append(payload, `"}`...)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/db/entries", bytes.NewReader(payload))
	req.Host = "guestbook.localhost:8443"
	req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
	w := httptest.NewRecorder()
	api.handleCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
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
		DeployID:         "d1", DeployedBy: "alice@co", DeployedAt: time.Now(),
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

func TestAPIEventPublishing(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(context.Background())

	fp := &fakePublisher{}
	api := NewAPI(cfg, database, governance.New(cfg), fp)

	withChiParams := func(r *http.Request, params map[string]string) *http.Request {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}

	t.Run("create emits create event", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/db/items", bytes.NewReader([]byte(`{"x":1}`)))
		req.Host = "guestbook.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
		req = withChiParams(req, map[string]string{"collection": "items"})
		w := httptest.NewRecorder()
		api.handleCreate(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
		if len(fp.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(fp.events))
		}
		if fp.events[0].eventType != "create" {
			t.Fatalf("expected eventType %q, got %q", "create", fp.events[0].eventType)
		}
	})

	var docID string
	t.Run("update emits update event", func(t *testing.T) {
		fp.events = nil

		createReq := httptest.NewRequest(http.MethodPost, "/api/v1/db/items", bytes.NewReader([]byte(`{"y":2}`)))
		createReq.Host = "guestbook.localhost:8443"
		createReq = createReq.WithContext(auth.WithUser(createReq.Context(), auth.DevUser))
		createReq = withChiParams(createReq, map[string]string{"collection": "items"})
		cw := httptest.NewRecorder()
		api.handleCreate(cw, createReq)
		var created db.Document
		if err := json.NewDecoder(cw.Body).Decode(&created); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		docID = created.ID
		fp.events = nil

		req := httptest.NewRequest(http.MethodPut, "/api/v1/db/items/"+docID, bytes.NewReader([]byte(`{"y":3}`)))
		req.Host = "guestbook.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
		req = withChiParams(req, map[string]string{"collection": "items", "id": docID})
		w := httptest.NewRecorder()
		api.handleUpdate(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
		if len(fp.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(fp.events))
		}
		if fp.events[0].eventType != "update" {
			t.Fatalf("expected eventType %q, got %q", "update", fp.events[0].eventType)
		}
	})

	t.Run("delete emits delete event", func(t *testing.T) {
		fp.events = nil
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/items/"+docID, nil)
		req.Host = "guestbook.localhost:8443"
		req = req.WithContext(auth.WithUser(req.Context(), auth.DevUser))
		req = withChiParams(req, map[string]string{"collection": "items", "id": docID})
		w := httptest.NewRecorder()
		api.handleDelete(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
		if len(fp.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(fp.events))
		}
		if fp.events[0].eventType != "delete" {
			t.Fatalf("expected eventType %q, got %q", "delete", fp.events[0].eventType)
		}
	})
}
