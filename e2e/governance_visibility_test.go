package e2e

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/server"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"log/slog"
)

func TestGovernedModeStaticReadDenied(t *testing.T) {
	const proxySecret = "e2e-proxy-secret"
	t.Setenv("ARTIFACT_E2E_PROXY_SECRET", proxySecret)

	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.Governance.Mode = "governed"
	cfg.Listen = ":19557"
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "storage")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	cfg.Auth.Mode = "header-trust"
	cfg.Auth.HeaderTrust.ProxySecretEnv = "ARTIFACT_E2E_PROXY_SECRET"
	cfg.Auth.HeaderTrust.EmailHeader = "X-Test-Email"
	cfg.Auth.HeaderTrust.NameHeader = "X-Test-Name"

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	srv, err := server.New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())
	time.Sleep(500 * time.Millisecond)

	store := srv.Store()
	database := srv.DB()
	cache := sites.NewDeployCache(store, 64)
	deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)

	alice := &auth.User{Email: "alice@co", Name: "Alice", Groups: []string{"employees"}}
	// Seed private visibility before deploy; UpsertSite ON CONFLICT does not overwrite visibility.
	if err := database.UpsertSite(context.Background(), &db.SiteRecord{
		Name: "private-hr", Owner: alice.Email, Visibility: "private",
	}); err != nil {
		t.Fatal(err)
	}
	exampleDir := filepath.Join("..", "examples", "guestbook")
	_, err = deployer.Deploy(context.Background(), "private-hr", exampleDir, alice)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodGet, "http://private-hr.localhost:19557/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Artifact-Proxy-Auth", proxySecret)
	req.Header.Set("X-Test-Email", "bob@co")
	req.Header.Set("X-Test-Name", "Bob")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner static fetch, got %d", resp.StatusCode)
	}
}
