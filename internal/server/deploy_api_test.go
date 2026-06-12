package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

func setupDeployServer(t *testing.T) (*Server, storage.Store, *db.DB) {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "sites")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")

	store, err := storage.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	cache := sites.NewDeployCache(store, 16)
	deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)
	srv := &Server{cfg: cfg, db: database, deployer: deployer, store: store}
	return srv, store, database
}

func deployTestUser() *auth.User {
	return &auth.User{
		Email:  "deployer@example.com",
		Name:   "Deploy Tester",
		Groups: []string{"employees", "admins"},
	}
}

func multipartDeployRequest(t *testing.T, site string, fields map[string]string, files map[string][]byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		part, err := w.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deploy", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(auth.WithUser(req.Context(), deployTestUser()))
	return req
}

func zipDeployRequest(t *testing.T, site string, fields map[string]string, entries map[string][]byte) *http.Request {
	t.Helper()
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for name, content := range entries {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if fields == nil {
		fields = map[string]string{}
	}
	fields["site"] = site
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	part, err := w.CreateFormFile("zip", "site.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, &zipBuf); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deploy", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(auth.WithUser(req.Context(), deployTestUser()))
	return req
}

func parseDeployResponse(t *testing.T, body string) deployResponse {
	t.Helper()
	var resp deployResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, body)
	}
	return resp
}

func TestDeployAPIFilesUpload(t *testing.T) {
	srv, store, database := setupDeployServer(t)
	site := "my-site"

	req := multipartDeployRequest(t, site, map[string]string{"site": site}, map[string][]byte{
		"index.html":    []byte("<html>hello</html>"),
		"assets/app.js": []byte("console.log('hi')"),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	resp := parseDeployResponse(t, w.Body.String())
	if resp.URL == "" || resp.DeployID == "" {
		t.Fatalf("missing url/deploy_id: %+v", resp)
	}
	if resp.FileCount != 2 {
		t.Fatalf("file_count = %d, want 2", resp.FileCount)
	}

	objPath := "sites/" + site + "/deploys/" + resp.DeployID + "/index.html"
	ok, err := store.Exists(context.Background(), objPath)
	if err != nil || !ok {
		t.Fatalf("index.html not in store at %s: ok=%v err=%v", objPath, ok, err)
	}

	rec, err := database.GetSite(context.Background(), site)
	if err != nil || rec == nil {
		t.Fatal("site record missing")
	}
	if rec.Owner != deployTestUser().Email {
		t.Fatalf("owner = %q, want %q", rec.Owner, deployTestUser().Email)
	}
}

func TestDeployAPIZipUpload(t *testing.T) {
	srv, store, _ := setupDeployServer(t)
	site := "zip-site"

	req := zipDeployRequest(t, site, nil, map[string][]byte{
		"index.html":    []byte("<html>zip</html>"),
		"assets/app.js": []byte("x=1"),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	resp := parseDeployResponse(t, w.Body.String())
	objPath := "sites/" + site + "/deploys/" + resp.DeployID + "/index.html"
	ok, _ := store.Exists(context.Background(), objPath)
	if !ok {
		t.Fatal("index.html missing from zip deploy")
	}
}

func TestDeployAPIZippedFolderUnwrap(t *testing.T) {
	srv, store, _ := setupDeployServer(t)
	site := "folder-site"

	req := zipDeployRequest(t, site, nil, map[string][]byte{
		"my-site/index.html": []byte("<html>nested</html>"),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	resp := parseDeployResponse(t, w.Body.String())
	objPath := "sites/" + site + "/deploys/" + resp.DeployID + "/index.html"
	ok, _ := store.Exists(context.Background(), objPath)
	if !ok {
		t.Fatal("unwrapped index.html missing")
	}
}

func TestDeployAPIPathTraversal(t *testing.T) {
	srv, _, _ := setupDeployServer(t)

	t.Run("files traversal", func(t *testing.T) {
		req := multipartDeployRequest(t, "evil", map[string]string{"site": "evil"}, map[string][]byte{
			`..\evil.html`: []byte("bad"),
		})
		w := httptest.NewRecorder()
		srv.handleDeploy(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})

	t.Run("zip traversal", func(t *testing.T) {
		req := zipDeployRequest(t, "evil2", nil, map[string][]byte{
			"../../evil": []byte("bad"),
		})
		w := httptest.NewRecorder()
		srv.handleDeploy(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})
}

func TestDeployAPIOverwriteConfirmation(t *testing.T) {
	srv, _, database := setupDeployServer(t)
	site := "existing"
	now := time.Now()
	if err := database.UpsertSite(context.Background(), &db.SiteRecord{
		Name: site, Owner: "other@co", DeployID: "old",
		DeployedBy: "other@co", DeployedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("without confirm", func(t *testing.T) {
		req := multipartDeployRequest(t, site, map[string]string{"site": site}, map[string][]byte{
			"index.html": []byte("<html>x</html>"),
		})
		w := httptest.NewRecorder()
		srv.handleDeploy(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
		var resp deployConflictResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if !resp.Exists || resp.Owner != "other@co" {
			t.Fatalf("unexpected conflict body: %+v", resp)
		}
	})

	t.Run("with confirm", func(t *testing.T) {
		req := multipartDeployRequest(t, site, map[string]string{
			"site": site, "confirm_overwrite": "true",
		}, map[string][]byte{
			"index.html": []byte("<html>new</html>"),
		})
		w := httptest.NewRecorder()
		srv.handleDeploy(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
	})
}

func TestDeployAPISourceProjectRejection(t *testing.T) {
	srv, _, _ := setupDeployServer(t)
	req := multipartDeployRequest(t, "src", map[string]string{"site": "src"}, map[string][]byte{
		"package.json": []byte(`{"name":"app"}`),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestDeployAPINoHTML(t *testing.T) {
	srv, _, _ := setupDeployServer(t)
	req := multipartDeployRequest(t, "nhtml", map[string]string{"site": "nhtml"}, map[string][]byte{
		"style.css": []byte("body{}"),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestDeployAPIRootPagePromotion(t *testing.T) {
	srv, store, _ := setupDeployServer(t)
	site := "about-site"

	req := multipartDeployRequest(t, site, map[string]string{"site": site}, map[string][]byte{
		"about.html": []byte("<html>about</html>"),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	resp := parseDeployResponse(t, w.Body.String())
	if len(resp.Warnings) == 0 || !strings.Contains(resp.Warnings[0], "about.html") {
		t.Fatalf("expected promotion warning, got %+v", resp.Warnings)
	}
	objPath := "sites/" + site + "/deploys/" + resp.DeployID + "/index.html"
	ok, _ := store.Exists(context.Background(), objPath)
	if !ok {
		t.Fatal("promoted index.html missing from storage")
	}
}

func TestDeployAPIMultipartPathsPreserved(t *testing.T) {
	srv, _, _ := setupDeployServer(t)
	req := multipartDeployRequest(t, "paths", map[string]string{"site": "paths"}, map[string][]byte{
		"nested/deep/file.txt": []byte("ok"),
		"index.html":           []byte("<html>p</html>"),
	})
	w := httptest.NewRecorder()
	srv.handleDeploy(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
