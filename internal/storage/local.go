package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// localStorage 基于本地磁盘的 Storage 实现（对象存储预留的落地形态）。
// 文件落在 baseDir 下，key 为斜杠分隔的相对路径，url 为 /uploads/<key>。
type localStorage struct {
	baseDir string
}

var _ types.Storage = (*localStorage)(nil)

// NewLocal 创建本地磁盘存储实现，目录不存在时自动创建。
func NewLocal(baseDir string) (types.Storage, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve storage dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &localStorage{baseDir: abs}, nil
}

func (s *localStorage) Put(ctx context.Context, key string, r io.Reader) (string, error) {
	rel := filepath.ToSlash(strings.TrimPrefix(filepath.Clean(filepath.FromSlash(key)), "/"))
	if rel == "." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	dst := filepath.Join(s.baseDir, filepath.FromSlash(rel))
	if !strings.HasPrefix(dst, s.baseDir) {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	_ = ctx
	return localURLPrefix + rel, nil
}

// localURLPrefix 本地存储对外暴露的静态访问前缀（由 router 挂载 /uploads 静态目录）。
const localURLPrefix = "/uploads/"

func (s *localStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	if !strings.HasPrefix(url, localURLPrefix) {
		return nil, fmt.Errorf("invalid storage url %q", url)
	}
	key := strings.TrimPrefix(url, localURLPrefix)
	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(key)))
	if rel == "." || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("invalid storage url %q", url)
	}
	f, err := os.Open(filepath.Join(s.baseDir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	return f, nil
}
