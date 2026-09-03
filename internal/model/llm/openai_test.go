package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) model.LLMClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewOpenAICompatible(model.ProviderConfig{
		Provider: "openai",
		Model:    "test-model",
		BaseURL:  srv.URL,
		APIKey:   "sk-test",
	})
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
