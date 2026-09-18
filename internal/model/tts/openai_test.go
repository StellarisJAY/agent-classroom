package tts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) model.TTSClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewOpenAICompatible(model.ProviderConfig{
		Provider: "openai", Model: "gpt-4o-mini-tts", BaseURL: srv.URL, APIKey: "sk-test",
	})
}

func TestSynthesizeSuccess(t *testing.T) {
	var got speechRequest
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/audio/speech", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fake-mp3-bytes"))
	})
	resp, err := client.Synthesize(context.Background(), model.TTSRequest{Text: " hello ", Voice: "female_warm", Speed: 1.2})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-mp3-bytes"), resp.Audio)
	require.Equal(t, "audio/mpeg", resp.ContentType)
	require.Equal(t, "gpt-4o-mini-tts", got.Model)
	require.Equal(t, "hello", got.Input)
	require.Equal(t, "nova", got.Voice, "平台音色应映射为 OpenAI 参数")
	require.Equal(t, 1.2, got.Speed)
	require.Equal(t, "mp3", got.ResponseFormat)
}

func TestSynthesizeDefaultVoiceAndFormat(t *testing.T) {
	var got speechRequest
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = w.Write([]byte("audio"))
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi"})
	require.NoError(t, err)
	require.Equal(t, defaultVoice, got.Voice)
	require.Equal(t, "mp3", got.ResponseFormat)
	require.Zero(t, got.Speed)
}

func TestSynthesizeUnknownVoicePassthrough(t *testing.T) {
	var got speechRequest
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = w.Write([]byte("audio"))
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi", Voice: "vendor-custom"})
	require.NoError(t, err)
	require.Equal(t, "vendor-custom", got.Voice)
}

func TestSynthesizeEmptyText(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "  "})
	require.Error(t, err)
}

func TestSynthesizeUpstreamError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid voice"}}`))
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi"})
	require.ErrorContains(t, err, "invalid voice")
}

func TestSynthesizeEmptyAudio(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi"})
	require.Error(t, err)
}
