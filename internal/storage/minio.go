package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// minioStorage 基于 Minio 对象存储的 Storage 实现。
// 对象 key 为斜杠分隔的相对路径，url 与本地实现一致，为 /uploads/<key>。
type minioStorage struct {
	client *minio.Client
	bucket string
}

var _ types.Storage = (*minioStorage)(nil)

// NewMinio 创建 Minio 存储实现，启动时确保 bucket 存在。
func NewMinio(cfg config.MinioConfig) (types.Storage, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("minio endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("minio bucket is required")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("create minio bucket %q: %w", cfg.Bucket, err)
		}
	}
	return &minioStorage{client: client, bucket: cfg.Bucket}, nil
}

func (s *minioStorage) Put(ctx context.Context, key string, r io.Reader) (string, error) {
	objectKey, err := cleanObjectKey(key)
	if err != nil {
		return "", err
	}
	if _, err := s.client.PutObject(ctx, s.bucket, objectKey, r, -1, minio.PutObjectOptions{}); err != nil {
		return "", fmt.Errorf("minio put object: %w", err)
	}
	return urlPrefix + objectKey, nil
}

func (s *minioStorage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	if !strings.HasPrefix(url, urlPrefix) {
		return nil, fmt.Errorf("invalid storage url %q", url)
	}
	objectKey, err := cleanObjectKey(strings.TrimPrefix(url, urlPrefix))
	if err != nil {
		return nil, fmt.Errorf("invalid storage url %q", url)
	}
	if _, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{}); err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio get object: %w", err)
	}
	return obj, nil
}

// cleanObjectKey 规范化对象 key：统一斜杠、去除前导 /，并拒绝目录穿越。
func cleanObjectKey(key string) (string, error) {
	rel := filepath.ToSlash(strings.TrimPrefix(filepath.Clean(filepath.FromSlash(key)), "/"))
	if rel == "." || rel == "" || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return rel, nil
}
