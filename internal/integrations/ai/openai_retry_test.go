// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fastRetry is used in all tests to keep them quick.
var fastRetry = RetryConfig{
	MaxRetries:  2,
	BaseDelay:   1 * time.Millisecond,
	MaxDelay:    10 * time.Millisecond,
	JitterRatio: 0,
}

// newEmbedder builds a test Embedder pointed at the given httptest server URL.
func newTestEmbedder(srvURL string) *Embedder {
	e := &Embedder{
		provider:    ProviderOpenAI,
		openAI:      &http.Client{},
		apiKey:      "test-key",
		model:       "text-embedding-3-small",
		baseURL:     srvURL,
		retryConfig: fastRetry,
	}
	e.dimensions.Store(1536)
	return e
}

// newTestLLMClient builds a test LLMClient pointed at the given httptest server URL.
func newTestLLMClient(srvURL string) *LLMClient {
	return &LLMClient{
		provider:    ProviderOpenAI,
		openAI:      &http.Client{},
		apiKey:      "test-key",
		model:       "gpt-4o-mini",
		baseURL:     srvURL,
		retryConfig: fastRetry,
	}
}

// embeddingOKBody returns a minimal valid OpenAI embeddings response.
func embeddingOKBody() []byte {
	type embItem struct {
		Embedding []float64 `json:"embedding"`
	}
	type embResp struct {
		Data []embItem `json:"data"`
	}
	b, _ := json.Marshal(embResp{Data: []embItem{{Embedding: []float64{0.1, 0.2, 0.3}}}})
	return b
}

// chatOKBody returns a minimal valid OpenAI chat completion response.
func chatOKBody(content string) []byte {
	type msg struct {
		Content string `json:"content"`
	}
	type choice struct {
		Message msg `json:"message"`
	}
	type resp struct {
		Choices []choice `json:"choices"`
	}
	b, _ := json.Marshal(resp{Choices: []choice{{Message: msg{Content: content}}}})
	return b
}

// statusServer returns a handler that serves status codes from the given slice
// in order, then always uses the last entry.
func statusServer(codes []int, body func(code int) []byte) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1)) - 1
		if n >= len(codes) {
			n = len(codes) - 1
		}
		code := codes[n]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if body != nil {
			_, _ = w.Write(body(code))
		}
	}))
	return srv, &calls
}

// ── embedOpenAI tests ──────────────────────────────────────────────────────

func TestEmbedOpenAI_RetryOn429(t *testing.T) {
	srv, calls := statusServer(
		[]int{429, 429, 200},
		func(code int) []byte {
			if code == 200 {
				return embeddingOKBody()
			}
			return []byte(`{"error":{"message":"rate limited"}}`)
		},
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	emb, err := e.embedOpenAI(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emb) == 0 {
		t.Fatal("expected non-empty embedding")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 server calls, got %d", got)
	}
}

func TestEmbedOpenAI_RetryOn500(t *testing.T) {
	srv, calls := statusServer(
		[]int{500, 200},
		func(code int) []byte {
			if code == 200 {
				return embeddingOKBody()
			}
			return []byte(`{"error":{"message":"internal"}}`)
		},
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.embedOpenAI(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
}

func TestEmbedOpenAI_NoRetryOn400(t *testing.T) {
	srv, calls := statusServer(
		[]int{400},
		func(_ int) []byte { return []byte(`{"error":{"message":"bad request"}}`) },
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.embedOpenAI(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 server call, got %d", got)
	}
}

func TestEmbedOpenAI_ExhaustsRetries(t *testing.T) {
	srv, calls := statusServer(
		[]int{429},
		func(_ int) []byte { return []byte(`{"error":{"message":"still rate limited"}}`) },
	)
	defer srv.Close()

	e := newTestEmbedder(srv.URL)
	_, err := e.embedOpenAI(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	// MaxRetries=2 → initial attempt + 2 retries = 3 calls
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 server calls (1 + 2 retries), got %d", got)
	}
}

// ── generateOpenAIText tests ───────────────────────────────────────────────

func TestGenerateOpenAIText_RetryOn429(t *testing.T) {
	srv, calls := statusServer(
		[]int{429, 200},
		func(code int) []byte {
			if code == 200 {
				return chatOKBody("pong")
			}
			return []byte(`{"error":{"message":"rate limited"}}`)
		},
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	text, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "pong" {
		t.Fatalf("expected 'pong', got %q", text)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
}

func TestGenerateOpenAIText_RetryOn500(t *testing.T) {
	srv, calls := statusServer(
		[]int{500, 200},
		func(code int) []byte {
			if code == 200 {
				return chatOKBody("ok")
			}
			return []byte(`{"error":{"message":"internal"}}`)
		},
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	_, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 server calls, got %d", got)
	}
}

func TestGenerateOpenAIText_NoRetryOn400(t *testing.T) {
	srv, calls := statusServer(
		[]int{400},
		func(_ int) []byte { return []byte(`{"error":{"message":"bad request"}}`) },
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	_, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 server call, got %d", got)
	}
}

func TestGenerateOpenAIText_ExhaustsRetries(t *testing.T) {
	srv, calls := statusServer(
		[]int{429},
		func(_ int) []byte { return []byte(`{"error":{"message":"still rate limited"}}`) },
	)
	defer srv.Close()

	l := newTestLLMClient(srv.URL)
	_, err := l.generateOpenAIText(context.Background(), "ping", 0.0, false)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 server calls (1 + 2 retries), got %d", got)
	}
}

// ── github_models client tests ─────────────────────────────────────────────

func TestGenerateText_GitHubModels(t *testing.T) {
	srv, _ := statusServer(
		[]int{200},
		func(_ int) []byte { return chatOKBody("response from github models") },
	)
	defer srv.Close()

	client := &LLMClient{
		provider:    ProviderGitHubModels,
		openAI:      &http.Client{},
		apiKey:      "ghs_test_token",
		model:       "gpt-4o-mini",
		baseURL:     srv.URL,
		retryConfig: fastRetry,
	}
	text, err := client.generateText(context.Background(), "ping", 0.3, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestCallOpenAIJSON_EmptyKeyError(t *testing.T) {
	err := callOpenAIJSON(context.Background(), nil, "", "", "/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("expected error mentioning API key, got: %v", err)
	}
}

// capturePathServer returns an httptest.Server whose handler records the
// request path and responds with the given body function.
func capturePathServer(t *testing.T, body func() []byte) (*httptest.Server, *string) {
	t.Helper()
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if body != nil {
			_, _ = w.Write(body())
		}
	}))
	return srv, &captured
}

// TestGitHubModels_ChatPathStripsV1 verifies that callOpenAIJSON strips the /v1
// prefix for the GitHub Models endpoint so chat calls reach /chat/completions,
// not /v1/chat/completions (which returns 404 on that endpoint).
func TestGitHubModels_ChatPathStripsV1(t *testing.T) {
	srv, capturedPath := capturePathServer(t, func() []byte { return chatOKBody("ok") })
	defer srv.Close()

	transport := &rewriteTransport{target: srv.URL}
	err := callOpenAIJSON(
		context.Background(),
		&http.Client{Transport: transport},
		"ghs_token",
		gitHubModelsBaseURL,
		"/v1/chat/completions",
		map[string]string{"model": "gpt-4o-mini"},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *capturedPath != "/chat/completions" {
		t.Errorf("expected path /chat/completions, got %q", *capturedPath)
	}
}

// TestGitHubModels_EmbeddingsPathStripsV1 verifies that callOpenAIJSON strips the
// /v1 prefix for the GitHub Models endpoint so embedding calls reach /embeddings,
// not /v1/embeddings (which returns 404 on that endpoint).
func TestGitHubModels_EmbeddingsPathStripsV1(t *testing.T) {
	srv, capturedPath := capturePathServer(t, func() []byte { return embeddingOKBody() })
	defer srv.Close()

	transport := &rewriteTransport{target: srv.URL}
	err := callOpenAIJSON(
		context.Background(),
		&http.Client{Transport: transport},
		"ghs_token",
		gitHubModelsBaseURL,
		"/v1/embeddings",
		map[string]string{"model": "text-embedding-3-small"},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *capturedPath != "/embeddings" {
		t.Errorf("expected path /embeddings, got %q", *capturedPath)
	}
}

// TestOpenAI_PathKeepsV1 verifies that the /v1 prefix is NOT stripped for the
// standard OpenAI base URL (only GitHub Models needs the strip).
func TestOpenAI_PathKeepsV1(t *testing.T) {
	srv, capturedPath := capturePathServer(t, func() []byte { return chatOKBody("ok") })
	defer srv.Close()

	err := callOpenAIJSON(
		context.Background(),
		&http.Client{},
		"sk-test",
		srv.URL, // not gitHubModelsBaseURL — no strip should happen
		"/v1/chat/completions",
		map[string]string{"model": "gpt-4o-mini"},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *capturedPath != "/v1/chat/completions" {
		t.Errorf("expected path /v1/chat/completions to be preserved for OpenAI, got %q", *capturedPath)
	}
}

// rewriteTransport redirects all requests to a fixed target URL (scheme+host),
// preserving path and query. Used in tests to intercept calls made to a real
// host constant (e.g. gitHubModelsBaseURL) without a real network connection.
type rewriteTransport struct {
	target string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := *req.URL
	newURL.Scheme = "http"
	newURL.Host = strings.TrimPrefix(strings.TrimPrefix(rt.target, "https://"), "http://")
	req2 := req.Clone(req.Context())
	req2.URL = &newURL
	req2.Host = newURL.Host
	return http.DefaultTransport.RoundTrip(req2)
}
