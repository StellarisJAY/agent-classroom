package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// 阿里云百炼（bailian）文生图适配器。
// 与 OpenAI Images API 协议不同：POST {base}/multimodal-generation/generation，
// 请求体为 input.messages[].content[{text}]，尺寸使用 "*" 分隔（DashScope 协议），
// 响应以图片 URL 形式返回，需再下载为字节。
// base_url 约定填到接口前缀（如 https://dashscope.aliyuncs.com/api/v1/services/aigc）。

// generationPath 百炼多模态生成接口路径（追加在接口前缀后）。
const generationPath = "/multimodal-generation/generation"

// bailianClient 基于标准库实现的阿里云百炼文生图客户端。
type bailianClient struct {
	cfg model.ProviderConfig
}

var _ model.ImageClient = (*bailianClient)(nil)

// bailianRequest 对齐百炼 multimodal-generation 请求体。
type bailianRequest struct {
	Model      string            `json:"model"`
	Input      bailianInput      `json:"input"`
	Parameters bailianParameters `json:"parameters"`
}

type bailianInput struct {
	Messages []bailianMessage `json:"messages"`
}

type bailianMessage struct {
	Role    string           `json:"role"`
	Content []bailianContent `json:"content"`
}

type bailianContent struct {
	Text string `json:"text"`
}

type bailianParameters struct {
	N            int    `json:"n"`
	Size         string `json:"size,omitempty"`
	PromptExtend bool   `json:"prompt_extend"`
	Watermark    bool   `json:"watermark"`
}

// bailianResponse 对齐百炼 multimodal-generation 响应体。
type bailianResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []map[string]string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Results []struct {
			Image string `json:"image"`
		} `json:"results"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// bailianImageURL 从响应中提取第一张图片 URL。
func (r *bailianResponse) imageURL() string {
	for _, ch := range r.Output.Choices {
		for _, item := range ch.Message.Content {
			if u := item["image"]; u != "" {
				return u
			}
		}
	}
	for _, res := range r.Output.Results {
		if res.Image != "" {
			return res.Image
		}
	}
	return ""
}

func (c *bailianClient) GenerateImage(ctx context.Context, req model.ImageRequest) (*model.ImageResponse, error) {
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
	payload := bailianRequest{
		Model: c.cfg.Model,
		Input: bailianInput{
			Messages: []bailianMessage{{
				Role:    "user",
				Content: []bailianContent{{Text: req.Prompt}},
			}},
		},
		Parameters: bailianParameters{
			N:            1,
			Size:         strings.ReplaceAll(size, "x", "*"),
			PromptExtend: true,
			Watermark:    false,
		},
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
		return nil, fmt.Errorf("image upstream error (status %d): %s", resp.StatusCode, extractBailianError(raw))
	}

	var parsed bailianResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse bailian response: %w", err)
	}
	if parsed.Code != "" && parsed.Message != "" {
		return nil, fmt.Errorf("image upstream error: %s", parsed.Message)
	}
	url := parsed.imageURL()
	if url == "" {
		return nil, fmt.Errorf("image: empty image url in bailian response")
	}

	data, err := downloadImage(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("download bailian image: %w", err)
	}
	return &model.ImageResponse{Data: data}, nil
}

func (c *bailianClient) endpoint() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + generationPath
}

// downloadImage 匿名下载图片 URL 为字节。
func downloadImage(ctx context.Context, url string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extractBailianError 尽力从非 2xx 响应中提取百炼错误信息，失败则返回原始片段。
func extractBailianError(raw []byte) string {
	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && (parsed.Code != "" || parsed.Message != "") {
		msg := strings.TrimSpace(parsed.Message)
		if parsed.Code != "" {
			msg = fmt.Sprintf("%s (code %s)", msg, parsed.Code)
		}
		return msg
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
