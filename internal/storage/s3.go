package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
)

type S3Store struct {
	client *minio.Client
	bucket string
}

func NewS3(cfg *config.Config) (*S3Store, error) {
	endpoint := cfg.Storage.Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	accessKey := os.Getenv("ARTIFACT_S3_ACCESS_KEY")
	secretKey := os.Getenv("ARTIFACT_S3_SECRET_KEY")
	useSSL := os.Getenv("ARTIFACT_S3_USE_SSL") != "false"
	if endpoint == "minio:9000" || endpoint == "localhost:9000" || endpoint == "127.0.0.1:9000" {
		useSSL = false
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to S3 endpoint %s: %w", endpoint, err)
	}
	return &S3Store{client: client, bucket: cfg.Storage.Bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, path string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, path, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3Store) Get(ctx context.Context, path string) (io.ReadCloser, *ObjectInfo, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, err
	}
	info := &ObjectInfo{
		Path: path, Size: stat.Size, ContentType: stat.ContentType,
		ETag: stat.ETag, LastModified: stat.LastModified,
	}
	return obj, info, nil
}

func (s *S3Store) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		out = append(out, ObjectInfo{
			Path: obj.Key, Size: obj.Size, ETag: obj.ETag, LastModified: obj.LastModified,
		})
	}
	return out, nil
}

func (s *S3Store) Delete(ctx context.Context, path string) error {
	return s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{})
}

func (s *S3Store) DeletePrefix(ctx context.Context, prefix string) error {
	objects := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	for obj := range objects {
		if obj.Err != nil {
			return obj.Err
		}
		if err := s.client.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3Store) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
