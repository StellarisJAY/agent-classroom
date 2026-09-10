package image

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

func newBailianClient(t *testing.T, gen http.HandlerFunc) model.ImageClient {
	t.Helper()
	genSrv := httptest.NewServer(gen)
	t.Cleanup(genSrv.Close)
	return NewBailian(model.ProviderConfig{
		Provider: "bailian", Model: "qwen-image-3.0", BaseURL: genSrv.URL, APIKey: "sk-test",
	})
}

func TestBailianGenerateImageSuccess(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fake-png-bytes"))
	}))
	t.Cleanup(imgSrv.Close)

	client := newBailianClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, generationPath, r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"image":"` + imgSrv.URL + `"}]}}]}}`))
	})

	resp, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a flower", Size: "1024x1024"})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-png-bytes"), resp.Data)
}

func TestBailianGenerateImageUsesStarSize(t *testing.T) {
	client := newBailianClient(t, func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		require.Contains(t, string(buf), `"size":"1024*1024"`)
		require.Contains(t, string(buf), `"prompt_extend":true`)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"results":[{"image":"https://x/1.png"}]}}`))
	})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat", Size: "1024x1024"})
	require.Error(t, err, "下载不存在的 URL 应失败")
}

func TestBailianGenerateImageResultsFallback(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok-bytes"))
	}))
	t.Cleanup(imgSrv.Close)

	client := newBailianClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"results":[{"image":"` + imgSrv.URL + `"}]}}`))
	})
	resp, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat"})
	require.NoError(t, err)
	require.Equal(t, []byte("ok-bytes"), resp.Data)
}

func TestBailianGenerateImageEmptyURL(t *testing.T) {
	client := newBailianClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[]}}`))
	})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat"})
	require.ErrorContains(t, err, "empty image url")
}

func TestBailianGenerateImageUpstreamError(t *testing.T) {
	client := newBailianClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"InvalidParameter","message":"size invalid"}`))
	})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat"})
	require.ErrorContains(t, err, "size invalid")
}

func TestBailianGenerateImageDownloadFails(t *testing.T) {
	client := newBailianClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"image":"http://127.0.0.1:1/nope.png"}]}}]}}`))
	})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat"})
	require.Error(t, err)
}

func TestBailianGenerateImageEmptyPrompt(t *testing.T) {
	client := newBailianClient(t, func(w http.ResponseWriter, _ *http.Request) {})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "  "})
	require.Error(t, err)
}
