package llm

import (
	"fmt"
	"net/http"
	"time"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// 本包提供 OpenAI 兼容协议的 LLM 实现，覆盖 openai/deepseek/qwen 等
// 通过自定义 base_url 适配的主流厂商。

// 常见 provider 字符串（与前端下拉、user_model_config.provider 对齐）。
const (
	ProviderOpenAI   = "openai"
	ProviderDeepSeek = "deepseek"
	ProviderQwen     = "qwen"
)

// UpstreamError 上游模型服务错误，携带 HTTP 状态码与上游 message。
type UpstreamError struct {
	StatusCode int
	Message    string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("model upstream error (status %d): %s", e.StatusCode, e.Message)
}

// NewOpenAICompatible 构造 OpenAI 兼容客户端（默认 60s 超时）。
func NewOpenAICompatible(cfg model.ProviderConfig) model.LLMClient {
	return &openaiClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}
