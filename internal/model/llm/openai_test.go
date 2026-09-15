package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) model.LLMClient {
	t.Helper()
	return newTestClientCfg(t, model.ProviderConfig{}, handler)
}

// newTestClientCfg 用给定配置（含超时）构造 client。
func newTestClientCfg(t *testing.T, cfg model.ProviderConfig, handler http.HandlerFunc) model.LLMClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg.Provider = "openai"
	cfg.Model = "test-model"
	cfg.BaseURL = srv.URL
	cfg.APIKey = "sk-test"
	return NewOpenAICompatible(cfg)
}

func TestChat(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"你好"}}]}`))
	})
	resp, err := client.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	require.Equal(t, "你好", resp.Content)
}

// asToolStream 断言 client 支持 ChatToolStream 并返回该视图。
func asToolStream(t *testing.T, c model.LLMClient) model.ToolStreamClient {
	t.Helper()
	tc, ok := c.(model.ToolStreamClient)
	require.True(t, ok)
	return tc
}

func TestChatStream(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	})
	var got string
	err := client.ChatStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, func(delta string) error {
		got += delta
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, "你好", got)
}

func TestChatStreamInterrupt(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\n",
		))
	})
	count := 0
	err := client.ChatStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, func(delta string) error {
		count++
		return context.Canceled
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, count, "回调返回错误后应立即中断")
}

func ptr[T any](v T) *T { return &v }

func TestReasoningEffort(t *testing.T) {
	cases := []struct {
		thinking string
		want     *string
	}{
		{model.ThinkingMax, ptr("high")},
		{model.ThinkingOff, nil},
		{model.ThinkingDefault, nil},
		{"", nil},
	}
	for _, c := range cases {
		got := reasoningEffort(c.thinking)
		if c.want == nil {
			require.Nil(t, got)
		} else {
			require.NotNil(t, got)
			require.Equal(t, *c.want, *got)
		}
	}
}

func TestChatSendsReasoningEffort(t *testing.T) {
	var got struct {
		ReasoningEffort *string `json:"reasoning_effort"`
	}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	})
	_, err := client.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
		Thinking: model.ThinkingMax,
	})
	require.NoError(t, err)
	require.NotNil(t, got.ReasoningEffort)
	require.Equal(t, "high", *got.ReasoningEffort)
}

func TestChatUpstreamError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	})
	_, err := client.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	})
	require.Error(t, err)
	ue, ok := err.(*UpstreamError)
	require.True(t, ok)
	require.Equal(t, http.StatusUnauthorized, ue.StatusCode)
	require.Equal(t, "invalid api key", ue.Message)
}

// 非流式请求超过配置超时应返回 deadline 错误。
func TestChatTimeout(t *testing.T) {
	client := newTestClientCfg(t, model.ProviderConfig{Timeout: 50 * time.Millisecond}, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	})
	_, err := client.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// 思考模式 max 时超时放大：相同耗时下 max 不超时、非 max 超时。
func TestChatThinkingMaxExtendsTimeout(t *testing.T) {
	// base 200ms，max 放大到 300ms。服务端耗时 250ms：max 成功（300ms 内）。
	client := newTestClientCfg(t, model.ProviderConfig{Timeout: 200 * time.Millisecond}, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	})
	_, err := client.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
		Thinking: model.ThinkingMax,
	})
	require.NoError(t, err, "thinking=max 应放大超时而不超时")

	// 同一耗时，非 max（200ms）应超时。
	client2 := newTestClientCfg(t, model.ProviderConfig{Timeout: 200 * time.Millisecond}, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	})
	_, err2 := client2.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err2, context.DeadlineExceeded, "非 max 思考应命中基础超时")
}

// 流式不设限（StreamTimeout=0）时不应被超时截断。
func TestChatStreamNoTimeoutByDefault(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		flusher.Flush()
		time.Sleep(80 * time.Millisecond)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n"))
		flusher.Flush()
	})
	var got string
	err := client.ChatStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, func(delta string) error {
		got += delta
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, "好", got)
}

// 流式工具调用：arguments 跨分片增量，两个调用（index 0/1）交错输出，按 finish_reason 汇总。
func TestChatToolStreamToolCalls(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_a\",\"type\":\"function\",\"function\":{\"name\":\"highlight\",\"arguments\":\"{\\\"elem\\\"\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"id\":\"call_b\",\"type\":\"function\",\"function\":{\"name\":\"jump\",\"arguments\":\"{\\\"sec\\\"\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\":1}\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"function\":{\"arguments\":\":2}\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
				"data: [DONE]\n\n",
		))
	})
	var textGot []string
	var calls []model.ToolCall
	err := asToolStream(t, client).ChatToolStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, model.StreamHandler{
		OnText: func(delta string) error { textGot = append(textGot, delta); return nil },
		OnToolCall: func(tc []model.ToolCall) error {
			calls = append(calls, tc...)
			return nil
		},
	})
	require.NoError(t, err)
	require.Empty(t, textGot)
	require.Len(t, calls, 2)
	require.Equal(t, "call_a", calls[0].ID)
	require.Equal(t, "highlight", calls[0].Function.Name)
	require.Equal(t, `{"elem":1}`, calls[0].Function.Arguments)
	require.Equal(t, "call_b", calls[1].ID)
	require.Equal(t, `{"sec":2}`, calls[1].Function.Arguments)
}

// 无 finish_reason 时未 flush 的工具调用应在 [DONE] 汇总。
func TestChatToolStreamFlushAtDone(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"draw\",\"arguments\":\"{}\"}}]}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	})
	var calls []model.ToolCall
	err := asToolStream(t, client).ChatToolStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, model.StreamHandler{OnToolCall: func(tc []model.ToolCall) error {
		calls = append(calls, tc...)
		return nil
	}})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	require.Equal(t, "draw", calls[0].Function.Name)
	require.Equal(t, "function", calls[0].Type, "上游未带 type 时应回填默认值")
}

// 文本增量与工具调用混合输出：各走各的回调，互不干扰。
func TestChatToolStreamMixed(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"看这里\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"highlight\",\"arguments\":\"{}\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
				"data: [DONE]\n\n",
		))
	})
	var text strings.Builder
	var calls []model.ToolCall
	err := asToolStream(t, client).ChatToolStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, model.StreamHandler{
		OnText: func(delta string) error { text.WriteString(delta); return nil },
		OnToolCall: func(tc []model.ToolCall) error {
			calls = append(calls, tc...)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "看这里", text.String())
	require.Len(t, calls, 1)
}

// OnToolCall 返回 error 时中断流。
func TestChatToolStreamInterrupt(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"draw\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"after\"}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	})
	calls := 0
	err := asToolStream(t, client).ChatToolStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	}, model.StreamHandler{OnToolCall: func(tc []model.ToolCall) error {
		calls++
		return context.Canceled
	}})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

// 请求体应携带 Tools 与旋转工具消息序列（assistant tool_calls + tool 结果）。
func TestChatToolStreamPayload(t *testing.T) {
	var got struct {
		Tools []model.Tool `json:"tools"`
		Msgs  []struct {
			Role       string           `json:"role"`
			Content    *string          `json:"content"`
			ToolCalls  []model.ToolCall `json:"tool_calls"`
			ToolCallID *string          `json:"tool_call_id"`
		} `json:"messages"`
	}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	err := asToolStream(t, client).ChatToolStream(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{
			{Role: model.RoleUser, Content: "讲讲"},
			{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{{
				ID: "c1", Type: "function",
				Function: model.ToolCallFunc{Name: "highlight", Arguments: "{}"},
			}}},
			{Role: model.RoleTool, ToolCallID: "c1", Name: "highlight", Content: "success"},
		},
		Tools: []model.Tool{{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "highlight",
				Description: "高亮",
				Parameters:  map[string]any{"type": "object"},
			},
		}},
	}, model.StreamHandler{})
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Equal(t, "function", got.Tools[0].Type)
	require.Equal(t, "highlight", got.Tools[0].Function.Name)
	require.Len(t, got.Msgs, 3)
	require.Nil(t, got.Msgs[1].Content, "带 tool_calls 的 assistant 消息不应发送空 content")
	require.NotNil(t, got.Msgs[2].ToolCallID)
	require.Equal(t, "c1", *got.Msgs[2].ToolCallID)
	require.Equal(t, "success", *got.Msgs[2].Content)
}

// 非流式响应中的 tool_calls 应解析到 ChatResponse。
func TestChatNonStreamToolCalls(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"draw","arguments":"{}"}}]}}]}`))
	})
	resp, err := client.Chat(context.Background(), model.ChatRequest{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	require.Equal(t, "draw", resp.ToolCalls[0].Function.Name)
}
