package tts

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

// defaultFormat 未指定格式时的默认输出格式。
const defaultFormat = "mp3"

// defaultVoice OpenAI 兼容未命中平台音色映射时的兜底音色。
const defaultVoice = "alloy"

// openaiVoiceMap 平台统一音色 ID → OpenAI 音色参数的映射。
// 未命中的 ID 原样透传，以兼容使用自定义音色名的第三方兼容厂商。
var openaiVoiceMap = map[string]string{
	"female_warm":      "nova",
	"female_bright":    "shimmer",
	"female_narration": "fable",
	"male_deep":        "onyx",
	"male_steady":      "echo",
	"neutral_clear":    "alloy",
}

// openaiClient 基于标准库实现的 OpenAI 兼容语音合成客户端。
type openaiClient struct {
	cfg model.ProviderConfig
}

var _ model.TTSClient = (*openaiClient)(nil)

// speechRequest 对齐 OpenAI Audio Speech API 请求体。
type speechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
}

func (c *openaiClient) Synthesize(ctx context.Context, req model.TTSRequest) (*model.TTSResponse, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("tts: empty text")
	}
	if c.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	format := req.Format
	if format == "" {
		format = defaultFormat
	}
	payload := speechRequest{
		Model:          c.cfg.Model,
		Input:          text,
		Voice:          c.resolveVoice(req.Voice),
		Speed:          req.Speed,
		ResponseFormat: format,
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

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("tts upstream error (status %d): %s", resp.StatusCode, extractError(raw))
	}
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tts response: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("tts: empty audio data")
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	return &model.TTSResponse{Audio: audio, ContentType: contentType}, nil
}

// resolveVoice 将平台音色 ID 映射为 OpenAI 音色参数；空值取默认音色，
// 未命中映射的 ID 原样透传。
func (c *openaiClient) resolveVoice(voice string) string {
	if voice == "" {
		return defaultVoice
	}
	if mapped, ok := openaiVoiceMap[voice]; ok {
		return mapped
	}
	return voice
}

func (c *openaiClient) endpoint() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + "/audio/speech"
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
