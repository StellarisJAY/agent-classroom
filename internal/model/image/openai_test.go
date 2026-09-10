package image

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) model.ImageClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewOpenAICompatible(model.ProviderConfig{
		Provider: "openai", Model: "gpt-image-1", BaseURL: srv.URL, APIKey: "sk-test",
	})
}

func b64Response(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte("fake-png-bytes"))
}

func TestGenerateImageSuccess(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/images/generations", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + b64Response(t) + `"}]}`))
	})
	resp, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat", Size: "1024x1024"})
	require.NoError(t, err)
	require.Equal(t, []byte("fake-png-bytes"), resp.Data)
}

func TestGenerateImageEmptyPrompt(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "  "})
	require.Error(t, err)
}

func TestGenerateImageUpstreamError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat"})
	require.ErrorContains(t, err, "rate limited")
}

func TestGenerateImageInvalidBase64(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"not-valid-###"}]}`))
	})
	_, err := client.GenerateImage(context.Background(), model.ImageRequest{Prompt: "a cat"})
	require.Error(t, err)
}
