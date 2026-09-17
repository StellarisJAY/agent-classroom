package extractor

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	mineru "github.com/opendatalab/MinerU-Ecosystem/sdk/go"
)

// MineruConfig minerU 服务接入配置（官方 API 由官方 Go SDK 实现）。
type MineruConfig struct {
	// BaseURL 官方 API 或自托管服务的 v4 兼容端点（空 → SDK 默认 https://mineru.net/api/v4）
	BaseURL string
	// AdminToken 官方 API 的管理令牌；为空或无效时回退免登录 Flash 提取
	AdminToken string
	// Timeout 单篇文档提取（上传+轮询，SDK 内部托管）总超时
	Timeout time.Duration
}

// Mineru minerU 文档提取器：薄适配官方 Go SDK，bytes 经临时文件提交，统一为同步 Extract。
type Mineru struct {
	cfg    MineruConfig
	client *mineru.Client
	authed bool
}

var _ Extractor = (*Mineru)(nil)

// NewMineru 创建 minerU 提取器。
func NewMineru(cfg MineruConfig) *Mineru {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	var opts []mineru.ClientOption
	if cfg.BaseURL != "" {
		opts = append(opts, mineru.WithBaseURL(cfg.BaseURL), mineru.WithFlashBaseURL(cfg.BaseURL))
	}

	client, err := mineru.New(cfg.AdminToken, opts...)
	authed := true
	if err != nil {
		// 令牌无效或缺失 → 免登录 Flash 提取（受 10MB/20 页限制，超出部分由 chain 回退本地兜底）
		slog.Info("mineru token unavailable, fallback to flash extraction", "error", err)
		client = mineru.NewFlash(opts...)
		authed = false
	}
	cfg.Timeout = timeout
	return &Mineru{cfg: cfg, client: client, authed: authed}
}

// Extract 按部署形态分派提取。
func (m *Mineru) Extract(ctx context.Context, filename string, data []byte) (Result, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".txt") ||
		strings.HasSuffix(strings.ToLower(filename), ".md") ||
		strings.HasSuffix(strings.ToLower(filename), ".markdown") {
		return Result{}, errors.New("mineru: plain text file, use local extractor")
	}

	path, cleanup, err := stagedFile(filename, data)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	var res *mineru.ExtractResult
	if m.authed {
		res, err = m.client.Extract(ctx, path,
			mineru.WithFormula(true),
			mineru.WithTable(true),
			mineru.WithPollTimeout(m.cfg.Timeout))
	} else {
		res, err = m.client.FlashExtract(ctx, path, mineru.WithFlashTimeout(m.cfg.Timeout))
	}
	if err != nil {
		return Result{}, err
	}
	if rerr := res.Err(); rerr != nil {
		return Result{}, rerr
	}
	return Result{Text: res.Markdown, Source: "mineru"}, nil
}

// stagedFile 把文档 bytes 写入带源扩展名的临时文件，供 SDK 按格式识别；cleanup 负责删除。
func stagedFile(filename string, data []byte) (path string, cleanup func(), err error) {
	f, cerr := os.CreateTemp("", "mineru-*"+filepath.Ext(filename))
	if cerr != nil {
		return "", nil, cerr
	}
	if _, werr := f.Write(data); werr != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, werr
	}
	if cerr := f.Close(); cerr != nil {
		os.Remove(f.Name())
		return "", nil, cerr
	}
	name := f.Name()
	return name, func() { os.Remove(name) }, nil
}
