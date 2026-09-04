package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
