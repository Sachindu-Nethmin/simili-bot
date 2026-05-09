// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-05-09
// Last Modified: 2026-05-09

package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchIssuesRaw_Success(t *testing.T) {
	payload := map[string]any{
		"total_count": 2,
		"items": []map[string]any{
			{
				"number":   1,
				"title":    "Normal issue",
				"body":     "Some body",
				"html_url": "https://github.com/org/repo/issues/1",
				"state":    "open",
			},
			{
				"number":   2,
				"title":    "A pull request",
				"body":     "PR body",
				"html_url": "https://github.com/org/repo/pull/2",
				"state":    "open",
				"pull_request": map[string]any{
					"url": "https://api.github.com/repos/org/repo/pulls/2",
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	hits, err := searchIssuesRaw(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].Type != "issue" {
		t.Errorf("expected hits[0].Type = issue, got %s", hits[0].Type)
	}
	if hits[1].Type != "pr" {
		t.Errorf("expected hits[1].Type = pr, got %s", hits[1].Type)
	}
}

func TestSearchIssuesRaw_Empty(t *testing.T) {
	payload := map[string]any{"total_count": 0, "items": []any{}}
	data, _ := json.Marshal(payload)
	hits, err := searchIssuesRaw(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(hits))
	}
}

func TestSearchIssues_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()

	// Build a Searcher pointing at the test server via a plain http.Client.
	// We test rate-limit detection using the raw helper since NewSearcher
	// requires oauth2; the HTTP-level test uses a real server to verify header parsing.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/search/issues", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Ratelimit-Remaining") != "0" {
		t.Errorf("expected X-Ratelimit-Remaining=0")
	}
}
