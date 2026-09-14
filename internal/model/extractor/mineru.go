package extractor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// MineruConfig minerU 服务接入配置。
type MineruConfig struct {
	// Mode 部署形态：official 官方 API | selfhosted 自托管端点
	Mode string
	// BaseURL 官方 API（https://mineru.net）或自托管服务地址
	BaseURL string
	// AdminToken 官方 API 的管理令牌；自托管模式下作为 Bearer 鉴权
	AdminToken string
	// Timeout 单篇文档提取（上传+轮询）总超时
	Timeout time.Duration
	// PollInterval 官方 API 轮询结果间隔
	PollInterval time.Duration
}

// Mineru minerU 文档提取器：official / selfhosted 两种部署形态统一为同步 Extract。
type Mineru struct {
	cfg    MineruConfig
	client *http.Client
	poll   time.Duration
}

var _ Extractor = (*Mineru)(nil)

// NewMineru 创建 minerU 提取器。
func NewMineru(cfg MineruConfig) *Mineru {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = 3 * time.Second
	}
	return &Mineru{
		cfg:    cfg,
		client: &http.Client{},
		poll:   poll,
	}
}

// Extract 按部署形态分派提取。
func (m *Mineru) Extract(ctx context.Context, filename string, data []byte) (Result, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".txt") ||
		strings.HasSuffix(strings.ToLower(filename), ".md") ||
		strings.HasSuffix(strings.ToLower(filename), ".markdown") {
		return Result{}, errors.New("mineru: plain text file, use local extractor")
	}
	switch m.cfg.Mode {
	case "selfhosted":
		return m.extractSelfhosted(ctx, filename, data)
	default:
		return m.extractOfficial(ctx, filename, data)
	}
}

// ---- selfhosted（POST {base}/file_parse，multipart 上传，JSON 直接返回 md）----

type fileParseResp struct {
	Markdown string `json:"md_content"`
}

func (m *Mineru) extractSelfhosted(ctx context.Context, filename string, data []byte) (Result, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if err := mw.WriteField("files", "1"); err != nil {
		return Result{}, fmt.Errorf("mineru: %w", err)
	}
	fw, err := mw.CreateFormFile("files", filename)
	if err != nil {
		return Result{}, err
	}
	if _, err := fw.Write(data); err != nil {
		return Result{}, err
	}
	if err := mw.Close(); err != nil {
		return Result{}, err
	}

	base := strings.TrimSuffix(m.cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/file_parse", body)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+m.cfg.AdminToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("mineru: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Result{}, fmt.Errorf("mineru: status %d: %s", resp.StatusCode, raw)
	}
	var parsed fileParseResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Result{}, fmt.Errorf("mineru: decode resp: %w", err)
	}
	return Result{Text: parsed.Markdown, Source: "mineru"}, nil
}

// ---- official（签名 token → 申请上传端点 → PUT 上传 → 轮询结果 zip 内的 md）----

const (
	mineruSignPath  = "/api/v4/common/token/sign"
	mineruApplyPath = "/api/v4/file-urls/batch"
	mineruStateFmt  = "/api/v4/extract-results/batch/%s"
)

type tokenSignResp struct {
	Code int    `json:"code"`
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

type applyUploadReq struct {
	EnableFormula bool               `json:"enable_formula"`
	EnableTable   bool               `json:"enable_table"`
	Files         []applyUploadFile  `json:"files"`
}

type applyUploadFile struct {
	IsOCR bool   `json:"is_ocr"`
	Name  string `json:"name"`
}

type applyUploadResp struct {
	Code int    `json:"code"`
	Data struct {
		BatchID  string   `json:"batch_id"`
		FileURLs []string `json:"file_urls"`
	} `json:"data"`
}

type batchResultResp struct {
	Code int    `json:"code"`
	Data struct {
		ExtractResults []struct {
			State      string `json:"state"`
			FullZipURL string `json:"full_zip_url"`
			ErrMsg     string `json:"err_msg"`
		} `json:"extract_result"`
	} `json:"data"`
}

func (m *Mineru) extractOfficial(ctx context.Context, filename string, data []byte) (Result, error) {
	if m.cfg.AdminToken == "" {
		return Result{}, errors.New("mineru: admin token not configured")
	}
	jwt, err := m.signToken(ctx)
	if err != nil {
		return Result{}, err
	}
	apply, err := m.applyUpload(ctx, jwt, filename)
	if err != nil {
		return Result{}, err
	}
	if len(apply.Data.FileURLs) == 0 {
		return Result{}, errors.New("mineru: no upload url granted")
	}
	if err := m.putFile(ctx, apply.Data.FileURLs[0], data); err != nil {
		return Result{}, err
	}
	return m.pollResults(ctx, jwt, apply.Data.BatchID)
}

// signToken 用管理令牌换取官方 API 的临时 JWT。
func (m *Mineru) signToken(ctx context.Context) (string, error) {
	base := strings.TrimSuffix(m.cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+mineruSignPath, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+m.cfg.AdminToken)
	var parsed tokenSignResp
	if err := doJSON(m.client, req, &parsed); err != nil {
		return "", fmt.Errorf("mineru: sign token: %w", err)
	}
	if parsed.Data.Token == "" {
		return "", fmt.Errorf("mineru: sign token empty (code %d)", parsed.Code)
	}
	return parsed.Data.Token, nil
}

func (m *Mineru) applyUpload(ctx context.Context, jwt, filename string) (*applyUploadResp, error) {
	payload := applyUploadReq{
		EnableFormula: true,
		EnableTable:   true,
		Files:         []applyUploadFile{{IsOCR: false, Name: filename}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(m.cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+mineruApplyPath, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	var parsed applyUploadResp
	if err := doJSON(m.client, req, &parsed); err != nil {
		return nil, fmt.Errorf("mineru: apply upload: %w", err)
	}
	return &parsed, nil
}

// putFile 按 presigned PUT 端点上传文件本体。
func (m *Mineru) putFile(ctx context.Context, url string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("mineru: upload: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("mineru: upload status %d", resp.StatusCode)
	}
	return nil
}

// pollResults 轮询批量任务直至完成，下载结果 zip 并解出 markdown 文本。
func (m *Mineru) pollResults(ctx context.Context, jwt, batchID string) (Result, error) {
	ticker := time.NewTicker(m.poll)
	defer ticker.Stop()
	for {
		state, zipURL, errMsg, err := m.stateOfBatch(ctx, jwt, batchID)
		if err != nil {
			return Result{}, err
		}
		switch {
		case strings.EqualFold(state, "done") && zipURL != "":
			text, zerr := mdFromZip(ctx, zipURL)
			if zerr != nil {
				return Result{}, fmt.Errorf("mineru: download result: %w", zerr)
			}
			return Result{Text: text, Source: "mineru"}, nil
		case strings.EqualFold(state, "done") && zipURL == "":
			return Result{}, fmt.Errorf("mineru: empty zip url: %s", errMsg)
		case strings.EqualFold(state, "failed"):
			return Result{}, fmt.Errorf("mineru: task failed: %s", errMsg)
		}
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return Result{}, ctx.Err()
			}
			return Result{}, errors.New("mineru: timeout while polling")
		case <-ticker.C:
		}
	}
}

// stateOfBatch 查询批次中首个文件（我们每次仅提交单文件）的状态。
func (m *Mineru) stateOfBatch(ctx context.Context, jwt, batchID string) (state, zipURL, errMsg string, err error) {
	base := strings.TrimSuffix(m.cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		base+fmt.Sprintf(mineruStateFmt, batchID), nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	var parsed batchResultResp
	if derr := doJSON(m.client, req, &parsed); derr != nil {
		return "", "", "", fmt.Errorf("mineru: poll: %w", derr)
	}
	if len(parsed.Data.ExtractResults) == 0 {
		return "", "", "", fmt.Errorf("mineru: no results for batch %s (code %d)", batchID, parsed.Code)
	}
	r := parsed.Data.ExtractResults[0]
	return r.State, r.FullZipURL, r.ErrMsg, nil
}

// mdFromZip 下载结果 zip，解出首个 markdown 文件内容。
func mdFromZip(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".md") && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}
	return "", errors.New("markdown file not found in result zip")
}

// doJSON 执行请求并把 JSON 响应解入 out；非 2xx 返回错误。
func doJSON(client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
