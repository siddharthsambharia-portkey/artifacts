package storage

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"google.golang.org/api/iterator"
)

type GCSStore struct {
	client *storage.Client
	bucket string
}

func NewGCS(cfg *config.Config) (*GCSStore, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create GCS client: %w — ensure GOOGLE_APPLICATION_CREDENTIALS is set", err)
	}
	return &GCSStore{client: client, bucket: cfg.Storage.Bucket}, nil
}

func (g *GCSStore) Put(ctx context.Context, path string, r io.Reader, size int64, contentType string) error {
	w := g.client.Bucket(g.bucket).Object(path).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (g *GCSStore) Get(ctx context.Context, path string) (io.ReadCloser, *ObjectInfo, error) {
	obj := g.client.Bucket(g.bucket).Object(path)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, nil, err
	}
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, nil, err
	}
	info := &ObjectInfo{
		Path: path, Size: attrs.Size, ContentType: attrs.ContentType,
		ETag: attrs.Etag, LastModified: attrs.Updated,
	}
	return r, info, nil
}

func (g *GCSStore) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	it := g.client.Bucket(g.bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append(out, ObjectInfo{
			Path: attrs.Name, Size: attrs.Size, ContentType: attrs.ContentType,
			ETag: attrs.Etag, LastModified: attrs.Updated,
		})
	}
	return out, nil
}

func (g *GCSStore) Delete(ctx context.Context, path string) error {
	return g.client.Bucket(g.bucket).Object(path).Delete(ctx)
}

func (g *GCSStore) DeletePrefix(ctx context.Context, prefix string) error {
	it := g.client.Bucket(g.bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return err
		}
		if err := g.client.Bucket(g.bucket).Object(attrs.Name).Delete(ctx); err != nil {
			return err
		}
	}
}

func (g *GCSStore) Exists(ctx context.Context, path string) (bool, error) {
	_, err := g.client.Bucket(g.bucket).Object(path).Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	return err == nil, err
}
