package sites

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

func setupStaticHandler(t *testing.T) (*StaticHandler, storage.Store) {
	t.Helper()
	tmp := t.TempDir()
	store, err := storage.NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewDeployCache(store, 16)
	cfg := config.DefaultDev()
	handler := NewStaticHandler(cfg, store, cache)

	ctx := context.Background()
	if err := store.Put(ctx, "sites/demo/.artifact-current", strings.NewReader("123"), 3, "text/plain"); err != nil {
		t.Fatal(err)
	}
	indexHTML := "<html><body>hello</body></html>"
	if err := store.Put(ctx, "sites/demo/deploys/123/index.html", strings.NewReader(indexHTML), int64(len(indexHTML)), "text/html"); err != nil {
		t.Fatal(err)
	}
	return handler, store
}

func TestStaticHandlerServeHTTP(t *testing.T) {
	handler, _ := setupStaticHandler(t)

	tests := []struct {
		name       string
		host       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "root serves index.html",
			host:       "demo.localhost:8443",
			path:       "/",
			wantStatus: http.StatusOK,
			wantBody:   "hello",
		},
		{
			name:       "dot dot slash rejected",
			host:       "demo.localhost:8443",
			path:       "/../secret",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nested traversal rejected",
			host:       "demo.localhost:8443",
			path:       "/a/../../b",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "encoded dot dot rejected",
			host:       "demo.localhost:8443",
			path:       "/%2e%2e/x",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing file 404",
			host:       "demo.localhost:8443",
			path:       "/nonexistent.css",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "apex host no site",
			host:       "localhost:8443",
			path:       "/",
			wantStatus: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Host = tt.host
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %q", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantBody != "" {
				body, _ := io.ReadAll(w.Body)
				if !strings.Contains(string(body), tt.wantBody) {
					t.Fatalf("body = %q, want substring %q", body, tt.wantBody)
				}
			}
		})
	}
}

func TestStaticHandlerCORS(t *testing.T) {
	handler, _ := setupStaticHandler(t)

	tests := []struct {
		name            string
		origin          string
		wantACAO        string
		wantCredentials bool
	}{
		{
			name:            "allowed subdomain origin",
			origin:          "http://other.localhost",
			wantACAO:        "http://other.localhost",
			wantCredentials: true,
		},
		{
			name:     "external origin",
			origin:   "https://evil.example.com",
			wantACAO: "",
		},
		{
			name:     "no origin",
			origin:   "",
			wantACAO: "",
		},
		{
			name:            "apex origin",
			origin:          "http://localhost",
			wantACAO:        "http://localhost",
			wantCredentials: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = "demo.localhost:8443"
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			gotACAO := w.Header().Get("Access-Control-Allow-Origin")
			if gotACAO != tt.wantACAO {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", gotACAO, tt.wantACAO)
			}
			gotCreds := w.Header().Get("Access-Control-Allow-Credentials")
			if tt.wantCredentials {
				if gotCreds != "true" {
					t.Errorf("Access-Control-Allow-Credentials = %q, want true", gotCreds)
				}
			} else if gotCreds != "" {
				t.Errorf("Access-Control-Allow-Credentials = %q, want absent", gotCreds)
			}
			if gotVary := w.Header().Get("Vary"); gotVary != "Origin" {
				t.Errorf("Vary = %q, want Origin", gotVary)
			}
		})
	}
}
