package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalStore struct {
	root string
}

func NewLocal(root string) (*LocalStore, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create storage directory %s: %w", root, err)
	}
	return &LocalStore{root: root}, nil
}

func (l *LocalStore) fullPath(path string) string {
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "..") {
		return ""
	}
	return filepath.Join(l.root, clean)
}

func (l *LocalStore) Put(ctx context.Context, path string, r io.Reader, size int64, contentType string) error {
	full := l.fullPath(path)
	if full == "" {
		return fmt.Errorf("invalid path %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (l *LocalStore) Get(ctx context.Context, path string) (io.ReadCloser, *ObjectInfo, error) {
	full := l.fullPath(path)
	if full == "" {
		return nil, nil, fmt.Errorf("invalid path %q", path)
	}
	fi, err := os.Stat(full)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, nil, err
	}
	ct := mime.TypeByExtension(filepath.Ext(full))
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := md5.New()
	io.Copy(h, f)
	f.Seek(0, 0)
	info := &ObjectInfo{
		Path: path, Size: fi.Size(), ContentType: ct,
		ETag: hex.EncodeToString(h.Sum(nil)), LastModified: fi.ModTime(),
	}
	return f, info, nil
}

func (l *LocalStore) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	base := l.fullPath(prefix)
	var out []ObjectInfo
	err := filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(l.root, p)
		rel = filepath.ToSlash(rel)
		ct := mime.TypeByExtension(filepath.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		out = append(out, ObjectInfo{Path: rel, Size: fi.Size(), ContentType: ct, LastModified: fi.ModTime()})
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

func (l *LocalStore) Delete(ctx context.Context, path string) error {
	full := l.fullPath(path)
	if full == "" {
		return fmt.Errorf("invalid path %q", path)
	}
	return os.Remove(full)
}

func (l *LocalStore) DeletePrefix(ctx context.Context, prefix string) error {
	base := l.fullPath(prefix)
	return os.RemoveAll(base)
}

func (l *LocalStore) Exists(ctx context.Context, path string) (bool, error) {
	full := l.fullPath(path)
	if full == "" {
		return false, nil
	}
	_, err := os.Stat(full)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// LocalFilePath returns the absolute filesystem path for local storage reads.
func (l *LocalStore) LocalFilePath(path string) string {
	return l.fullPath(path)
}

// WalkSiteFiles walks files under a site deploy prefix.
func (l *LocalStore) WalkSiteFiles(site, deployID string, fn func(relPath string, modTime time.Time) error) error {
	prefix := fmt.Sprintf("sites/%s/deploys/%s/", site, deployID)
	base := l.fullPath(prefix)
	return filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		return fn(filepath.ToSlash(rel), fi.ModTime())
	})
}
