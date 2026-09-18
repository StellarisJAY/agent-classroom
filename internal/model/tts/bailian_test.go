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

// newBailianTestClient 起一个同时处理生成与音频下载的测试服务：
// POST /multimodal-generation/generation 由 genHandler 处理；GET /audio.mp3 返回固定音频。
func newBailianTestClient(t *testing.T, genHandler http.HandlerFunc) model.TTSClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/audio.mp3" {
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("fake-mp3-bytes"))
			return
		}
		genHandler(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewBailian(model.ProviderConfig{
		Provider: "bailian", Model: "qwen3-tts-flash", BaseURL: srv.URL, APIKey: "sk-test",
	})
}

// writeBailianAudioURL 以请求 Host 拼出音频 URL，避免测试服务的先有鸡先有蛋。
func writeBailianAudioURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"output":{"audio":{"url":"http://` + r.Host + `/audio.mp3"}}}`))
}

func TestBailianSynthesizeSuccess(t *testing.T) {
	var got bailianSpeechRequest
	client := newBailianTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/multimodal-generation/generation", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		writeBailianAudioURL(w, r)
	})
	resp, err := client.Synthesize(context.Background(), model.TTSRequest{Text: " 你好 ", Voice: "male_deep"})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-mp3-bytes"), resp.Audio)
	require.Equal(t, "audio/mpeg", resp.ContentType)
	require.Equal(t, "qwen3-tts-flash", got.Model)
	require.Equal(t, "你好", got.Input.Text)
	require.Equal(t, "Andre", got.Input.Voice, "平台音色应映射为百炼参数")
}

func TestBailianSynthesizeVoiceFallback(t *testing.T) {
	for _, voice := range []string{"", "unknown-voice"} {
		var got bailianSpeechRequest
		client := newBailianTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			require.NoError(t, json.Unmarshal(body, &got))
			writeBailianAudioURL(w, r)
		})
		_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi", Voice: voice})
		require.NoError(t, err)
		require.Equal(t, bailianDefaultVoice, got.Input.Voice)
	}
}

func TestBailianSynthesizeEmptyText(t *testing.T) {
	client := newBailianTestClient(t, func(http.ResponseWriter, *http.Request) {})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "  "})
	require.Error(t, err)
}

func TestBailianSynthesizeUpstreamCode(t *testing.T) {
	client := newBailianTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":"InvalidParameter","message":"voice not found"}`))
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi"})
	require.ErrorContains(t, err, "voice not found")
}

func TestBailianSynthesizeHTTPError(t *testing.T) {
	client := newBailianTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"InvalidApiKey","message":"invalid key"}`))
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi"})
	require.ErrorContains(t, err, "invalid key")
}

func TestBailianSynthesizeEmptyURL(t *testing.T) {
	client := newBailianTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"output":{"audio":{}}}`))
	})
	_, err := client.Synthesize(context.Background(), model.TTSRequest{Text: "hi"})
	require.ErrorContains(t, err, "empty audio url")
}
