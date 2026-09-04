package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
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
			Content string `json:"content"`
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
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
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
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	return &model.ChatResponse{Content: content}, nil
}

func (c *openaiClient) ChatStream(ctx context.Context, req model.ChatRequest, onDelta model.StreamCallback) error {
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
	}
	resp, err := c.do(ctx, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readUpstreamError(resp)
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
			return nil
		}
		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		if err := onDelta(chunk.Choices[0].Delta.Content); err != nil {
			return err
		}
	}
	return scanner.Err()
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
