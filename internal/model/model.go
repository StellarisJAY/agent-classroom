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
	// RoleTool 工具执行结果消息，ToolCallID 标识其对应的 assistant 工具调用。
	RoleTool Role = "tool"
)

// ToolFunction 工具函数定义。
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Parameters JSON Schema 对象（map / 自定义结构均可，原样透传）。
	Parameters any `json:"parameters"`
}

// Tool 工具定义（请求侧 Tools 数组元素）。
type Tool struct {
	Type     string       `json:"type"` // 当前固定 "function"
	Function ToolFunction `json:"function"`
}

// ToolCallTypeFunction 工具调用 type 字段的固定取值（OpenAI 协议）。
const ToolCallTypeFunction = "function"

// ToolCall 一条工具调用记录。Arguments 为 JSON 原始字符串（协议约定，不解析透传）。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type,omitempty"` // "function"
	Function ToolCallFunc `json:"function"`
}

// ToolCallFunc 工具调用的函数名与参数（JSON 字符串，与协议对齐）。
type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatMessage 一条对话消息
type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content,omitempty"`
	// ToolCalls assistant 消息发起的工具调用（tool 角色消息不携带）。
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID tool 角色消息对应的调用 ID。
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Name tool 角色消息标识执行者（可选）。
	Name string `json:"name,omitempty"`
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
	// Tools 可注入的工具定义（OpenAI tools 格式）；为空不随请求下发。
	Tools []Tool
}

// ChatResponse 非流式对话结果
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall // 非流式响应中的工具调用（可能为空）
}

// StreamCallback 流式增量回调；返回 error 可中断生成。
type StreamCallback func(delta string) error

// StreamHandler 流式事件分发器（工具调用流使用）。
// OnText 处理文本增量；OnToolCall 在本轮输出的全部工具调用增量
// 拼装完成后回调一次（纯文本轮不触发）。二者均可为 nil，返回 error 中断生成。
type StreamHandler struct {
	OnText     StreamCallback
	OnToolCall func(toolCalls []ToolCall) error
}

// LLMClient 大模型客户端接口
type LLMClient interface {
	// Chat 非流式对话，返回完整内容。
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatStream 流式对话，逐片回调 onDelta；onDelta 返回 error 时提前中断。
	ChatStream(ctx context.Context, req ChatRequest, onDelta StreamCallback) error
}

// ToolStreamClient 支持工具调用流式输出的客户端扩展接口。
// 独立于 LLMClient 定义，避免既有实现（测试 fake 等）被迫实现。
// 含 Tools 的请求经此方法发起：文本增量交 OnText，拼装完成的
// 工具调用集合整体交 OnToolCall（一次对话轮最多回调一次）。
type ToolStreamClient interface {
	ChatToolStream(ctx context.Context, req ChatRequest, handler StreamHandler) error
}

// LLMFactory 根据配置构造 LLMClient 的工厂。
type LLMFactory func(cfg ProviderConfig) LLMClient

// ImageRequest 一次文生图请求。
type ImageRequest struct {
	// Prompt 图像描述提示词。
	Prompt string
	// Size 期望尺寸（如 "1024x1024"）；空串由实现取默认值。
	Size string
}

// ImageResponse 文生图结果，Data 为解码后的图片字节。
type ImageResponse struct {
	Data []byte
}

// ImageClient 文生图客户端接口。
type ImageClient interface {
	// GenerateImage 生成一张图片，返回图片字节。
	GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error)
}

// ImageFactory 根据配置构造 ImageClient 的工厂。
type ImageFactory func(cfg ProviderConfig) ImageClient

// TTSRequest 一次语音合成请求。
// Voice 为平台统一定义的音色 ID（见 TTSVoiceCatalog），由各适配器映射到供应商实际参数；
// Speed<=0 表示使用实现默认语速（1.0）；Format 为空由实现取默认格式。
type TTSRequest struct {
	Text   string
	Voice  string
	Speed  float64
	Format string
}

// TTSResponse 语音合成结果，Audio 为解码后的音频字节。
type TTSResponse struct {
	Audio       []byte
	ContentType string
}

// TTSClient 语音合成客户端接口。
type TTSClient interface {
	// Synthesize 合成一段语音，返回音频字节。
	Synthesize(ctx context.Context, req TTSRequest) (*TTSResponse, error)
}

// TTSFactory 根据配置构造 TTSClient 的工厂。
type TTSFactory func(cfg ProviderConfig) TTSClient

// Registry provider 路由表。命中注册项则用对应工厂，否则回退默认工厂。
// 默认工厂由上层（bootstrap）注入，避免 model 包反向依赖具体实现（如 llm 包）。
type Registry struct {
	llm          map[string]LLMFactory
	defaultLLM   LLMFactory
	image        map[string]ImageFactory
	defaultImage ImageFactory
	tts          map[string]TTSFactory
	defaultTTS   TTSFactory
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{
		llm:   make(map[string]LLMFactory),
		image: make(map[string]ImageFactory),
		tts:   make(map[string]TTSFactory),
	}
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

// RegisterImage 为指定 provider 注册文生图工厂。
func (r *Registry) RegisterImage(provider string, f ImageFactory) {
	r.image[provider] = f
}

// SetDefaultImageFactory 设置文生图未命中注册表时的回退工厂。
func (r *Registry) SetDefaultImageFactory(f ImageFactory) {
	r.defaultImage = f
}

// NewImage 按 provider 路由构造文生图客户端；未命中且无默认工厂时返回 nil。
func (r *Registry) NewImage(cfg ProviderConfig) ImageClient {
	if f, ok := r.image[cfg.Provider]; ok {
		return f(cfg)
	}
	if r.defaultImage != nil {
		return r.defaultImage(cfg)
	}
	return nil
}

// RegisterTTS 为指定 provider 注册语音合成工厂。
func (r *Registry) RegisterTTS(provider string, f TTSFactory) {
	r.tts[provider] = f
}

// SetDefaultTTSFactory 设置语音合成未命中注册表时的回退工厂。
func (r *Registry) SetDefaultTTSFactory(f TTSFactory) {
	r.defaultTTS = f
}

// NewTTS 按 provider 路由构造语音合成客户端；未命中且无默认工厂时返回 nil。
func (r *Registry) NewTTS(cfg ProviderConfig) TTSClient {
	if f, ok := r.tts[cfg.Provider]; ok {
		return f(cfg)
	}
	if r.defaultTTS != nil {
		return r.defaultTTS(cfg)
	}
	return nil
}
