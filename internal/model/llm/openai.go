package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

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
	Model       string              `json:"model"`
	Messages    []model.ChatMessage `json:"messages"`
	Stream      bool                `json:"stream"`
	Temperature *float64            `json:"temperature,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
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
	payload := chatCompletionRequest{
		Model:       c.cfg.Model,
		Messages:    req.Messages,
		Stream:      false,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
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
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Error != nil {
		return nil, &UpstreamError{StatusCode: resp.StatusCode, Message: parsed.Error.Message}
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	return &model.ChatResponse{Content: content}, nil
}

func (c *openaiClient) ChatStream(ctx context.Context, req model.ChatRequest, onDelta model.StreamCallback) error {
	payload := chatCompletionRequest{
		Model:       c.cfg.Model,
		Messages:    req.Messages,
		Stream:      true,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
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
