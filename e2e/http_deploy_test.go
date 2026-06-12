package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/server"
)

func TestHTTPDeployRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "sites")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("site", "e2e-site"); err != nil {
		t.Fatal(err)
	}
	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("<html><body>e2e deploy</body></html>")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/deploy", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("deploy status %d body %s", resp.StatusCode, b)
	}

	var deployResp struct {
		Site string `json:"site"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&deployResp); err != nil {
		t.Fatal(err)
	}
	if deployResp.Site != "e2e-site" {
		t.Fatalf("site = %q", deployResp.Site)
	}

	siteReq, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	siteReq.Host = "e2e-site.localhost"
	siteResp, err := http.DefaultClient.Do(siteReq)
	if err != nil {
		t.Fatal(err)
	}
	defer siteResp.Body.Close()
	if siteResp.StatusCode != http.StatusOK {
		t.Fatalf("site status %d", siteResp.StatusCode)
	}
	page, err := io.ReadAll(siteResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(page, []byte("e2e deploy")) {
		t.Fatalf("unexpected page body: %s", page)
	}
}
