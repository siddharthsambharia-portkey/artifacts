package files

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

func setupFilesHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.Domain = "localhost"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "store")
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
	h := NewHandler(cfg, store, database)
	site := "testsite"
	return h, site
}

func uploadRequest(t *testing.T, site, filename string, content []byte, contentType string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Host = site + ".localhost"
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Email: "tester@example.com"}))
	return req
}

func TestUploadListRoundtrip(t *testing.T) {
	h, site := setupFilesHandler(t)

	uploadReq := uploadRequest(t, site, "hello.png", []byte("fakepngdata"), "image/png")
	uploadRec := httptest.NewRecorder()
	h.Upload(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("upload status %d body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var up UploadResponse
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &up); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if up.ID == "" || up.URL == "" {
		t.Fatalf("upload response missing id/url: %+v", up)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	listReq.Host = site + ".localhost"
	listReq = listReq.WithContext(auth.WithUser(listReq.Context(), &auth.User{Email: "tester@example.com"}))
	listRec := httptest.NewRecorder()
	h.List(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", listRec.Code, listRec.Body.String())
	}

	var listed []FileRecord
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d records, want 1", len(listed))
	}
	if listed[0].ID != up.ID {
		t.Errorf("listed ID = %q, want %q", listed[0].ID, up.ID)
	}
	if listed[0].Filename != "hello.png" {
		t.Errorf("listed Filename = %q, want hello.png", listed[0].Filename)
	}
	if listed[0].URL != up.URL {
		t.Errorf("listed URL = %q, want %q", listed[0].URL, up.URL)
	}
}

func TestUploadServeRoundtrip(t *testing.T) {
	h, site := setupFilesHandler(t)

	fileContent := []byte("binary file data")
	uploadReq := uploadRequest(t, site, "data.bin", fileContent, "application/octet-stream")
	uploadRec := httptest.NewRecorder()
	h.Upload(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("upload status %d body %s", uploadRec.Code, uploadRec.Body.String())
	}

	var up UploadResponse
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &up); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	serveReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/"+up.ID, nil)
	serveReq.Host = site + ".localhost"
	serveReq = serveReq.WithContext(auth.WithUser(serveReq.Context(), &auth.User{Email: "tester@example.com"}))
	serveRec := httptest.NewRecorder()
	h.Serve(serveRec, serveReq)
	if serveRec.Code != http.StatusOK {
		t.Fatalf("serve status %d body %s", serveRec.Code, serveRec.Body.String())
	}
	if !bytes.Equal(serveRec.Body.Bytes(), fileContent) {
		t.Errorf("served body = %q, want %q", serveRec.Body.Bytes(), fileContent)
	}
}

func TestIsDangerousContentType(t *testing.T) {
	tests := []struct {
		name string
		ct   string
		want bool
	}{
		{"text/html", "text/html", true},
		{"text/html with charset", "text/html; charset=utf-8", true},
		{"application/javascript", "application/javascript", true},
		{"image/png safe", "image/png", false},
		{"application/pdf safe", "application/pdf", false},
		{
			name: "uppercase html not flagged",
			ct:   "TEXT/HTML",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDangerousContentType(tt.ct); got != tt.want {
				t.Fatalf("isDangerousContentType(%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}
