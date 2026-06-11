package sites

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
	lru "github.com/hashicorp/golang-lru/v2"
)

type DeployCache struct {
	store storage.Store
	lru   *lru.Cache[string, string]
	mu    sync.Mutex
}

func NewDeployCache(store storage.Store, size int) *DeployCache {
	c, _ := lru.New[string, string](size)
	return &DeployCache{store: store, lru: c}
}

func (d *DeployCache) CurrentDeployID(ctx context.Context, site string) (string, error) {
	d.mu.Lock()
	if id, ok := d.lru.Get(site); ok {
		d.mu.Unlock()
		return id, nil
	}
	d.mu.Unlock()

	rc, _, err := d.store.Get(ctx, storage.SiteCurrentPath(site))
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	d.mu.Lock()
	d.lru.Add(site, id)
	d.mu.Unlock()
	return id, nil
}

func (d *DeployCache) Invalidate(site string) {
	d.mu.Lock()
	d.lru.Remove(site)
	d.mu.Unlock()
}

// warmCache preloads hot sites on startup (optional).
func (d *DeployCache) Warm(ctx context.Context, sites []string) {
	for _, s := range sites {
		d.CurrentDeployID(ctx, s)
		time.Sleep(time.Millisecond)
	}
}
