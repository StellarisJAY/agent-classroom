package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// openaiClient 基于标准库实现的 OpenAI 兼容客户端。
type openaiClient struct {
	cfg        model.ProviderConfig
	httpClient *http.Client
}

var _ model.LLMClient = (*openaiClient)(nil)

// chatCompletionRequest 对齐 OpenAI Chat Completions 请求体。
type chatCompletionRequest struct {
	Model           string              `json:"model"`
	Messages        []model.ChatMessage `json:"messages"`
	Stream          bool                `json:"stream"`
	Temperature     *float64            `json:"temperature,omitempty"`
	MaxTokens       *int                `json:"max_tokens,omitempty"`
	ReasoningEffort *string             `json:"reasoning_effort,omitempty"`
	Tools           []model.Tool        `json:"tools,omitempty"`
}

// reasoningEffort 将 model.Thinking 映射为 OpenAI reasoning_effort。
// max → high；off / default / 空 不传（交由模型默认或非推理路径）。
func reasoningEffort(thinking string) *string {
	if thinking == model.ThinkingMax {
		e := "high"
		return &e
	}
	return nil
}

// chatCompletionResponse 非流式响应。
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content   string           `json:"content"`
			ToolCalls []model.ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// chatCompletionChunk 流式增量。
type chatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content   string          `json:"content"`
			ToolCalls []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// toolCallDelta 流式工具调用增量。ID/名称通常只在首个分片出现，
// arguments 为 JSON 字符串的增量片段，需按下标累加拼装。
type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// toolCallAcc 单个（按 index 分组）工具调用的增量拼装缓冲。
type toolCallAcc struct {
	id   string
	typ  string
	name strings.Builder
	args strings.Builder
}

// toolCallAccumulator 聚合一轮输出中的全部工具调用增量；按 index 建组，finalize 时按 index 排序输出。
type toolCallAccumulator struct {
	calls map[int]*toolCallAcc
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{calls: make(map[int]*toolCallAcc)}
}

// add 累加一组增量字段。
func (a *toolCallAccumulator) add(d toolCallDelta) {
	c := a.calls[d.Index]
	if c == nil {
		c = &toolCallAcc{typ: d.Type}
		a.calls[d.Index] = c
	}
	if d.ID != "" {
		c.id = d.ID
	}
	if d.Type != "" {
		c.typ = d.Type
	}
	c.name.WriteString(d.Function.Name)
	c.args.WriteString(d.Function.Arguments)
}

// finalize 输出拼装完成的调用并清空；无任何增量时返回 nil。
func (a *toolCallAccumulator) finalize() []model.ToolCall {
	if len(a.calls) == 0 {
		return nil
	}
	idx := make([]int, 0, len(a.calls))
	for i := range a.calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	out := make([]model.ToolCall, 0, len(idx))
	for _, i := range idx {
		c := a.calls[i]
		typ := c.typ
		if typ == "" {
			typ = "function"
		}
		out = append(out, model.ToolCall{
			ID:   c.id,
			Type: typ,
			Function: model.ToolCallFunc{
				Name:      c.name.String(),
				Arguments: c.args.String(),
			},
		})
	}
	a.calls = make(map[int]*toolCallAcc)
	return out
}

func (c *openaiClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	ctx, cancel := c.requestContext(ctx, c.cfg.Timeout, req.Thinking)
	defer cancel()

	payload := chatCompletionRequest{
		Model:           c.cfg.Model,
		Messages:        req.Messages,
		Stream:          false,
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxTokens,
		ReasoningEffort: reasoningEffort(req.Thinking),
		Tools:           req.Tools,
	}
	resp, err := c.do(ctx, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readUpstreamError(resp)
	}
	var parsed chatCompletionResponse
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	slog.Debug("openai raw resp", "raw", string(raw))

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	if parsed.Error != nil {
		return nil, &UpstreamError{StatusCode: resp.StatusCode, Message: parsed.Error.Message}
	}

	slog.Debug("parsed resp", "data", parsed)
	content := ""
	var toolCalls []model.ToolCall
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
		toolCalls = parsed.Choices[0].Message.ToolCalls
	}
	return &model.ChatResponse{Content: content, ToolCalls: toolCalls}, nil
}

func (c *openaiClient) ChatStream(ctx context.Context, req model.ChatRequest, onDelta model.StreamCallback) error {
	// 纯文本流：工具调用增量忽略（无分发口）。
	return c.ChatToolStream(ctx, req, model.StreamHandler{OnText: onDelta})
}

// ChatToolStream 实现 model.ToolStreamClient：
// 文本增量逐片回调 OnText；工具调用增量按 index 累加拼装，
// 在 finish_reason=tool_calls 或上游 [DONE] 时整体回调 OnToolCall（纯文本轮不触发）。
func (c *openaiClient) ChatToolStream(ctx context.Context, req model.ChatRequest, handler model.StreamHandler) error {
	// 流式输出持续时长不定，仅按 StreamTimeout 设限（不做思考模式放大），通常依赖上层 context 取消。
	ctx, cancel := c.requestContext(ctx, c.cfg.StreamTimeout, "")
	defer cancel()

	payload := chatCompletionRequest{
		Model:           c.cfg.Model,
		Messages:        req.Messages,
		Stream:          true,
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxTokens,
		ReasoningEffort: reasoningEffort(req.Thinking),
		Tools:           req.Tools,
	}
	resp, err := c.do(ctx, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readUpstreamError(resp)
	}

	acc := newToolCallAccumulator()
	flushTools := func() error {
		if handler.OnToolCall == nil {
			return nil
		}
		calls := acc.finalize()
		if len(calls) == 0 {
			return nil
		}
		return handler.OnToolCall(calls)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			if err := flushTools(); err != nil {
				return err
			}
			return nil
		}
		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if delta := choice.Delta.Content; delta != "" && handler.OnText != nil {
			if err := handler.OnText(delta); err != nil {
				return err
			}
		}
		for _, d := range choice.Delta.ToolCalls {
			acc.add(d)
		}
		if choice.FinishReason == "tool_calls" {
			if err := flushTools(); err != nil {
				return err
			}
		}
	}
	// 上游未发 [DONE] 即断流：有未 flush 的工具调用时补发一次，尽力而为。
	return errors.Join(scanner.Err(), flushTools())
}

func (c *openaiClient) do(ctx context.Context, payload any) (*http.Response, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	return c.httpClient.Do(req)
}

func (c *openaiClient) endpoint() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
}

// requestContext 为一次请求派生带超时的 context。
// base <= 0 时不额外设限，原样返回调用方 ctx。
// 非流式在 thinking=max（深度思考）时放大 1.5 倍，覆盖长推理耗时。
func (c *openaiClient) requestContext(ctx context.Context, base time.Duration, thinking string) (context.Context, context.CancelFunc) {
	if base <= 0 {
		return ctx, func() {}
	}
	if thinking == model.ThinkingMax {
		base = base * 3 / 2
	}
	return context.WithTimeout(ctx, base)
}

// readUpstreamError 读取非 2xx 响应中的 error.message 构造 UpstreamError。
func readUpstreamError(resp *http.Response) error {
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	msg := resp.Status
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err == nil && parsed.Error != nil && parsed.Error.Message != "" {
		msg = parsed.Error.Message
	}
	return &UpstreamError{StatusCode: resp.StatusCode, Message: msg}
}
