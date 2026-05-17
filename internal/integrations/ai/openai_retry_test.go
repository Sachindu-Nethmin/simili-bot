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

// TestGitHubModels_PathStripsV1 verifies that callOpenAIJSON strips the /v1
// prefix when targeting the GitHub Models base URL so calls reach /chat/completions
// instead of /v1/chat/completions (which returns 404 on that endpoint).
func TestGitHubModels_PathStripsV1(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(chatOKBody("ok"))
	}))
	defer srv.Close()

	// Point gitHubModelsBaseURL at the test server by constructing a client
	// whose baseURL equals gitHubModelsBaseURL so the stripping logic fires.
	// We achieve this by temporarily aliasing: call callOpenAIJSON directly
	// with a baseURL that equals gitHubModelsBaseURL's value but redirect via
	// the test server instead.
	//
	// Because the strip is keyed on the constant string, we exercise it by
	// using an LLMClient whose baseURL is set to the test server URL and then
	// calling callOpenAIJSON with the constant directly as baseURL=srvURL and
	// checking the path the test server received when baseURL == srvURL and the
	// server URL replaces gitHubModelsBaseURL.
	//
	// Simpler: call through the LLMClient.generateOpenAIText path and confirm
	// the request path reaching the server has no /v1 prefix.
	client := &LLMClient{
		provider:    ProviderGitHubModels,
		openAI:      &http.Client{},
		apiKey:      "ghs_token",
		model:       "gpt-4o-mini",
		baseURL:     gitHubModelsBaseURL, // triggers stripping logic
		retryConfig: fastRetry,
	}
	// Swap in the test server URL so traffic actually hits our handler.
	client.baseURL = srv.URL
	// Now manually verify that the strip fires by calling callOpenAIJSON
	// with baseURL == gitHubModelsBaseURL constant but routing via srv.
	// We use a fresh http.Client that always hits srv regardless of host.
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
	if strings.HasPrefix(capturedPath, "/v1") {
		t.Errorf("expected /v1 prefix to be stripped for GitHub Models, got path %q", capturedPath)
	}
	if capturedPath != "/chat/completions" {
		t.Errorf("expected path /chat/completions, got %q", capturedPath)
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
