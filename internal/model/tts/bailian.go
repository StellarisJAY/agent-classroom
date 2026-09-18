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

// 阿里云百炼（bailian）Qwen-TTS 适配器。
// 与 OpenAI Audio Speech 协议不同：POST {base}/services/aigc/multimodal-generation/generation，
// 请求体为 input.text / input.voice，非流式响应返回音频文件 URL（24h 有效），需再下载为字节。
// 百炼不支持语速参数，且对未知音色会直接报错，故未命中映射时回退默认音色。
// base_url 约定填到接口前缀（如 https://dashscope.aliyuncs.com/api/v1/services/aigc）。

// bailianGenerationPath 百炼多模态生成接口路径（追加在接口前缀后）。
const bailianGenerationPath = "/multimodal-generation/generation"

// bailianDefaultVoice 未指定或未命中平台音色映射时的百炼默认音色。
const bailianDefaultVoice = "Serena"

// bailianVoiceMap 平台统一音色 ID → 百炼 Qwen-TTS 系统音色映射。
var bailianVoiceMap = map[string]string{
	"female_warm":      "Serena",
	"female_bright":    "Cherry",
	"female_narration": "Elias",
	"male_deep":        "Andre",
	"male_steady":      "Neil",
	"neutral_clear":    "Kai",
}

// bailianClient 基于标准库实现的百炼 Qwen-TTS 语音合成客户端。
type bailianClient struct {
	cfg model.ProviderConfig
}

var _ model.TTSClient = (*bailianClient)(nil)

// bailianSpeechRequest 对齐百炼 Qwen-TTS 非实时语音合成请求体。
type bailianSpeechRequest struct {
	Model string          `json:"model"`
	Input bailianTTSInput `json:"input"`
}

type bailianTTSInput struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
}

// bailianSpeechResponse 对齐百炼 Qwen-TTS 非流式响应体（音频以 URL 形式返回）。
type bailianSpeechResponse struct {
	Output struct {
		Audio struct {
			URL string `json:"url"`
		} `json:"audio"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *bailianClient) Synthesize(ctx context.Context, req model.TTSRequest) (*model.TTSResponse, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("tts: empty text")
	}
	if c.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	payload := bailianSpeechRequest{
		Model: c.cfg.Model,
		Input: bailianTTSInput{
			Text:  text,
			Voice: resolveBailianVoice(req.Voice),
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
		return nil, fmt.Errorf("tts upstream error (status %d): %s", resp.StatusCode, extractBailianError(raw))
	}

	var parsed bailianSpeechResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse bailian tts response: %w", err)
	}
	if parsed.Code != "" && parsed.Message != "" {
		return nil, fmt.Errorf("tts upstream error: %s", parsed.Message)
	}
	url := strings.TrimSpace(parsed.Output.Audio.URL)
	if url == "" {
		return nil, fmt.Errorf("tts: empty audio url in bailian response")
	}

	audio, contentType, err := downloadAudio(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("download bailian audio: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("tts: empty audio data")
	}
	return &model.TTSResponse{Audio: audio, ContentType: contentType}, nil
}

func (c *bailianClient) endpoint() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + bailianGenerationPath
}

// resolveBailianVoice 将平台音色 ID 映射为百炼音色参数；空值或未命中回退默认音色。
func resolveBailianVoice(voice string) string {
	if voice == "" {
		return bailianDefaultVoice
	}
	if mapped, ok := bailianVoiceMap[voice]; ok {
		return mapped
	}
	return bailianDefaultVoice
}

// downloadAudio 匿名下载音频 URL，返回字节与 Content-Type（缺省 audio/mpeg）。
func downloadAudio(ctx context.Context, url string) ([]byte, string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	return data, contentType, nil
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
