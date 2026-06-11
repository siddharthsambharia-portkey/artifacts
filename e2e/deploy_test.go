package e2e

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

func TestDeployGuestbook(t *testing.T) {
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
	cache := sites.NewDeployCache(store, 512)
	deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)
	exampleDir := filepath.Join("..", "examples", "guestbook")
	url, err := deployer.Deploy(context.Background(), "guestbook", exampleDir, auth.DevUser)
	if err != nil {
		t.Fatal(err)
	}
	if url == "" {
		t.Fatal("empty deploy URL")
	}
}

func findBinary(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "bin", "artifact"),
		filepath.Join("bin", "artifact"),
	}
	if p, err := exec.LookPath("artifact"); err == nil {
		return p
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	t.Skip("artifact binary not found — run make build")
	return ""
}

func TestDevServerHealth(t *testing.T) {
	bin := findBinary(t)
	root, _ := filepath.Abs("..")
	cmd := exec.Command(bin, "dev")
	cmd.Dir = root
	port := "19443"
	cmd.Env = append(os.Environ(), "ARTIFACT_LISTEN=:"+port)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	deadline := time.Now().Add(10 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		resp, err = http.Get("http://127.0.0.1:" + port + "/healthz")
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz status %d", resp.StatusCode)
	}
}

func TestDeployTiming(t *testing.T) {
	start := time.Now()
	tmp := t.TempDir()
	cfg := config.DefaultDev()
	cfg.DataDir = filepath.Join(tmp, "data")
	cfg.Storage.Path = filepath.Join(tmp, "sites")
	cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
	store, _ := storage.New(cfg)
	database, _ := db.Open(cfg)
	database.Migrate(context.Background())
	cache := sites.NewDeployCache(store, 512)
	deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)
	exampleDir := filepath.Join("..", "examples", "guestbook")
	_, err := deployer.Deploy(context.Background(), "guestbook", exampleDir, auth.DevUser)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed > 60*time.Second {
		t.Errorf("deploy took %v, want < 60s", elapsed)
	}
}
