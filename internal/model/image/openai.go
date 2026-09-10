package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// defaultSize 未指定尺寸时的默认值。
const defaultSize = "1024x1024"

// openaiClient 基于标准库实现的 OpenAI 兼容文生图客户端。
type openaiClient struct {
	cfg model.ProviderConfig
}

var _ model.ImageClient = (*openaiClient)(nil)

// imageRequest 对齐 OpenAI Images API 请求体。
type imageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size,omitempty"`
	N              int    `json:"n"`
	ResponseFormat string `json:"response_format,omitempty"`
}

// imageResponse 对齐 OpenAI Images API 响应体（b64_json）。
type imageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openaiClient) GenerateImage(ctx context.Context, req model.ImageRequest) (*model.ImageResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("image: empty prompt")
	}
	if c.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	size := req.Size
	if size == "" {
		size = defaultSize
	}
	payload := imageRequest{
		Model:          c.cfg.Model,
		Prompt:         req.Prompt,
		Size:           size,
		N:              1,
		ResponseFormat: "b64_json",
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image upstream error (status %d): %s", resp.StatusCode, extractError(raw))
	}

	var parsed imageResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse image response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("image upstream error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return nil, fmt.Errorf("image: empty image data")
	}
	data, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode image base64: %w", err)
	}
	return &model.ImageResponse{Data: data}, nil
}

func (c *openaiClient) endpoint() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + "/images/generations"
}

// extractError 尽力从非 2xx 响应中提取 error.message，失败则返回原始片段。
func extractError(raw []byte) string {
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Error != nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
