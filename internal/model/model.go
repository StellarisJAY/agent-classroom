package model

import (
	"context"
	"time"
)

// 本包定义大模型（LLM/TTS）适配层的通用类型与接口。
// 该层不依赖 types / 服务层，仅依赖标准库，供上层业务按需装配。

// Role 消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ChatMessage 一条对话消息
type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ProviderConfig 一次模型调用的最小可用配置（APIKey 已解密）。
type ProviderConfig struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	// Timeout 非流式 Chat 请求超时；<=0 表示不额外设限，跟随调用方传入的 context。
	// 思考模式 max 时实现可在该基础上放大。
	Timeout time.Duration
	// StreamTimeout 流式 ChatStream 请求超时；<=0 表示不额外设限。
	StreamTimeout time.Duration
}

// 思考强度（reasoning effort）取值。空串等价于 default。
const (
	ThinkingOff     = "off"
	ThinkingDefault = "default"
	ThinkingMax     = "max"
)

// ChatRequest 一次对话请求
type ChatRequest struct {
	Messages    []ChatMessage
	Temperature *float64 // 可选
	MaxTokens   *int     // 可选
	// Thinking 思考限制：off / default / max（空串视为 default）。是否真正下发由各 provider 适配。
	Thinking string
}

// ChatResponse 非流式对话结果
type ChatResponse struct {
	Content string
}

// StreamCallback 流式增量回调；返回 error 可中断生成。
type StreamCallback func(delta string) error

// LLMClient 大模型客户端接口
type LLMClient interface {
	// Chat 非流式对话，返回完整内容。
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatStream 流式对话，逐片回调 onDelta；onDelta 返回 error 时提前中断。
	ChatStream(ctx context.Context, req ChatRequest, onDelta StreamCallback) error
}

// LLMFactory 根据配置构造 LLMClient 的工厂。
type LLMFactory func(cfg ProviderConfig) LLMClient

// Registry provider 路由表。命中注册项则用对应工厂，否则回退默认工厂。
// 默认工厂由上层（bootstrap）注入，避免 model 包反向依赖具体实现（如 llm 包）。
type Registry struct {
	llm        map[string]LLMFactory
	defaultLLM LLMFactory
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{llm: make(map[string]LLMFactory)}
}

// RegisterLLM 为指定 provider 注册工厂（未来非 OpenAI 协议厂商的扩展点）。
func (r *Registry) RegisterLLM(provider string, f LLMFactory) {
	r.llm[provider] = f
}

// SetDefaultLLMFactory 设置未命中注册表时的回退工厂。
func (r *Registry) SetDefaultLLMFactory(f LLMFactory) {
	r.defaultLLM = f
}

// NewLLM 按 provider 路由构造客户端；未命中且无默认工厂时返回 nil。
func (r *Registry) NewLLM(cfg ProviderConfig) LLMClient {
	if f, ok := r.llm[cfg.Provider]; ok {
		return f(cfg)
	}
	if r.defaultLLM != nil {
		return r.defaultLLM(cfg)
	}
	return nil
}
