package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

type ObjectInfo struct {
	Path         string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

type Store interface {
	Put(ctx context.Context, path string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, path string) (io.ReadCloser, *ObjectInfo, error)
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	Delete(ctx context.Context, path string) error
	DeletePrefix(ctx context.Context, prefix string) error
	Exists(ctx context.Context, path string) (bool, error)
}

func New(cfg *config.Config) (Store, error) {
	switch cfg.Storage.Driver {
	case "local":
		path := cfg.Storage.Path
		if path == "" {
			path = cfg.DataDir + "/sites"
		}
		return NewLocal(path)
	case "s3":
		return NewS3(cfg)
	case "gcs":
		return NewGCS(cfg)
	default:
		return nil, fmt.Errorf("unknown storage driver %q: use local, s3, or gcs", cfg.Storage.Driver)
	}
}

func SitePrefix(site string) string {
	return "sites/" + site + "/"
}

func SiteManifestPath(site string) string {
	return "sites/" + site + "/.artifact-manifest.json"
}

func SiteCurrentPath(site string) string {
	return "sites/" + site + "/.artifact-current"
}
