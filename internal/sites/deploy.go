package sites

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
)

type Manifest struct {
	Site       string            `json:"site"`
	DeployID   string            `json:"deploy_id"`
	Files      map[string]string `json:"files"` // path -> sha256
	DeployedBy string            `json:"deployed_by"`
	DeployedAt time.Time         `json:"deployed_at"`
	TotalBytes int64             `json:"total_bytes"`
}

type Deployer struct {
	cfg   *config.Config
	store storage.Store
	db    *db.DB
	gov   *governance.Governor
	cache *DeployCache
}

func NewDeployer(cfg *config.Config, store storage.Store, database *db.DB, gov *governance.Governor, cache *DeployCache) *Deployer {
	return &Deployer{cfg: cfg, store: store, db: database, gov: gov, cache: cache}
}

func (d *Deployer) Deploy(ctx context.Context, siteName, sourceDir string, user *auth.User) (string, error) {
	if siteName == "" {
		return "", fmt.Errorf("site name is required — run artifact deploy from a folder with artifact.json or pass --site")
	}
	if !validSiteName(siteName) {
		return "", fmt.Errorf("invalid site name %q: use lowercase letters, numbers, and hyphens only", siteName)
	}
	existing, _ := d.db.GetSite(ctx, siteName)
	if err := d.gov.CanDeploy(ctx, user, siteName, existing); err != nil {
		return "", err
	}
	deployID := fmt.Sprintf("%d", time.Now().UnixNano())
	manifest := &Manifest{
		Site: siteName, DeployID: deployID,
		Files:      make(map[string]string),
		DeployedBy: user.Email, DeployedAt: time.Now(),
	}
	prefix := fmt.Sprintf("sites/%s/deploys/%s/", siteName, deployID)
	var totalBytes int64
	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, _ := f.Stat()
		totalBytes += fi.Size()
		if totalBytes > int64(d.cfg.Governance.Quotas.SiteMaxMB)*1024*1024 {
			return fmt.Errorf("site exceeds %d MB quota — remove large files or ask an admin to raise site_max_mb", d.cfg.Governance.Quotas.SiteMaxMB)
		}
		h := sha256.New()
		tee := io.TeeReader(f, h)
		ct := mime.TypeByExtension(filepath.Ext(path))
		if ct == "" {
			ct = "application/octet-stream"
		}
		storagePath := prefix + rel
		if err := d.store.Put(ctx, storagePath, tee, fi.Size(), ct); err != nil {
			return fmt.Errorf("upload %s: %w", rel, err)
		}
		manifest.Files[rel] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	if err != nil {
		_ = d.store.DeletePrefix(ctx, prefix)
		return "", err
	}
	manifest.TotalBytes = totalBytes
	manifestData, _ := json.Marshal(manifest)
	manifestPath := fmt.Sprintf("sites/%s/deploys/%s/.artifact-manifest.json", siteName, deployID)
	if err := d.store.Put(ctx, manifestPath, strings.NewReader(string(manifestData)), int64(len(manifestData)), "application/json"); err != nil {
		return "", err
	}
	currentPath := storage.SiteCurrentPath(siteName)
	if err := d.store.Put(ctx, currentPath, strings.NewReader(deployID), int64(len(deployID)), "text/plain"); err != nil {
		return "", fmt.Errorf("flip deploy pointer: %w", err)
	}
	owner := ""
	if existing != nil {
		owner = existing.Owner
	}
	if owner == "" {
		owner = user.Email
	}
	rec := &db.SiteRecord{
		Name: siteName, Owner: owner, DeployID: deployID,
		DeployedBy: user.Email, DeployedAt: time.Now(),
		SizeBytes: totalBytes, Visibility: "public",
	}
	if err := d.db.UpsertSite(ctx, rec); err != nil {
		return "", err
	}
	if d.cache != nil {
		d.cache.Invalidate(siteName)
	}
	_ = d.db.InsertAudit(ctx, &db.AuditEntry{
		Timestamp: time.Now(), UserEmail: user.Email,
		Site: siteName, Action: "deploy", Detail: deployID,
	})
	url := d.siteURL(siteName)
	return url, nil
}

func (d *Deployer) CurrentDeployID(ctx context.Context, site string) (string, error) {
	rc, _, err := d.store.Get(ctx, storage.SiteCurrentPath(site))
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (d *Deployer) siteURL(site string) string {
	if d.cfg.Domain == "localhost" {
		return fmt.Sprintf("http://%s.localhost%s", site, d.cfg.Listen)
	}
	return fmt.Sprintf("https://%s.%s", site, d.cfg.Domain)
}

func validSiteName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func SiteNameFromDir(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "artifact.json"))
	if err != nil {
		return filepath.Base(dir)
	}
	var meta struct {
		Site string `json:"site"`
	}
	if json.Unmarshal(data, &meta) == nil && meta.Site != "" {
		return meta.Site
	}
	return filepath.Base(dir)
}
