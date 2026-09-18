package tts

import (
	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// 本包提供语音合成（TTS）实现：
//   - OpenAI 兼容协议（POST /audio/speech），覆盖 openai 及自定义 base_url 适配的兼容厂商；
//   - 其他供应商（如阿里云百炼）按需新增专属适配器并注册。

// NewOpenAICompatible 构造 OpenAI 兼容语音合成客户端。
// 超时由请求级 context 控制（时长来自 ProviderConfig.Timeout）。
func NewOpenAICompatible(cfg model.ProviderConfig) model.TTSClient {
	return &openaiClient{cfg: cfg}
}

// NewBailian 构造阿里云百炼 Qwen-TTS 语音合成客户端。
// base_url 需填到接口前缀，如 https://dashscope.aliyuncs.com/api/v1/services/aigc。
func NewBailian(cfg model.ProviderConfig) model.TTSClient {
	return &bailianClient{cfg: cfg}
}
