package image

import (
	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// 本包提供文生图实现：
//   - OpenAI 兼容协议（POST /images/generations），覆盖 openai 及自定义 base_url 适配的兼容厂商；
//   - 阿里云百炼（bailian）专属多模态协议（见 bailian.go）。

// NewOpenAICompatible 构造 OpenAI 兼容文生图客户端。
// 超时由请求级 context 控制（时长来自 ProviderConfig.Timeout）。
func NewOpenAICompatible(cfg model.ProviderConfig) model.ImageClient {
	return &openaiClient{cfg: cfg}
}

// NewBailian 构造阿里云百炼文生图客户端。
// base_url 需填到接口前缀，如 https://dashscope.aliyuncs.com/api/v1/services/aigc。
func NewBailian(cfg model.ProviderConfig) model.ImageClient {
	return &bailianClient{cfg: cfg}
}
