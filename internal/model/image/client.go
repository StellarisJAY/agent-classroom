package image

import (
	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// 本包提供 OpenAI 兼容协议的文生图实现（POST /images/generations），
// 覆盖 openai 及通过自定义 base_url 适配的兼容厂商。

// NewOpenAICompatible 构造 OpenAI 兼容文生图客户端。
// 超时由请求级 context 控制（时长来自 ProviderConfig.Timeout）。
func NewOpenAICompatible(cfg model.ProviderConfig) model.ImageClient {
	return &openaiClient{cfg: cfg}
}
