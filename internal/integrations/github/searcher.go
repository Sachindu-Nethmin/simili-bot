// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-05-09
// Last Modified: 2026-05-09

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ghlib "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// SearchHit represents a single result from the GitHub hybrid search API.
type SearchHit struct {
	Number int
	Title  string
	Body   string
	URL    string
	State  string
	Type   string // "issue" or "pr"
}

// Searcher wraps the GitHub /search/issues endpoint with search_type=hybrid support.
type Searcher struct {
	client *ghlib.Client
}

// NewSearcher creates a Searcher authenticated with the given token.
func NewSearcher(ctx context.Context, token string) *Searcher {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &Searcher{client: ghlib.NewClient(tc)}
}

// searchIssuesResponse mirrors the subset of GitHub's /search/issues response we use.
type searchIssuesResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		HTMLURL     string `json:"html_url"`
		State       string `json:"state"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request,omitempty"`
	} `json:"items"`
}

// SearchIssues queries /search/issues?search_type=hybrid for the given repo.
// itemType filters results: "issue" for issues only, "pr" for pull requests only,
// or "" to search both. Returns hits, rateLimited=true when the API rate-limit is
// hit (not a hard error), and a non-nil err only for genuine failures.
func (s *Searcher) SearchIssues(ctx context.Context, org, repo, query, itemType string, limit int) (hits []SearchHit, rateLimited bool, err error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	filter := "is:issue"
	if itemType == "pr" {
		filter = "is:pr"
	} else if itemType == "" {
		filter = "" // search both issues and PRs
	}
	q := fmt.Sprintf("repo:%s/%s %s %s", org, repo, filter, query)
	path := fmt.Sprintf("search/issues?q=%s&search_type=hybrid&per_page=%d",
		url.QueryEscape(q), limit)

	req, err := s.client.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, false, fmt.Errorf("searcher: build request: %w", err)
	}

	var raw searchIssuesResponse
	resp, err := s.client.Do(ctx, req, &raw)
	if err != nil {
		// go-github wraps non-2xx as ErrorResponse; check for rate-limit before surfacing.
		if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) {
			if resp.Header.Get("X-Ratelimit-Remaining") == "0" {
				return nil, true, nil
			}
		}
		return nil, false, fmt.Errorf("searcher: request failed: %w", err)
	}

	hits = make([]SearchHit, 0, len(raw.Items))
	for _, item := range raw.Items {
		t := "issue"
		if item.PullRequest != nil {
			t = "pr"
		}
		hits = append(hits, SearchHit{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			URL:    item.HTMLURL,
			State:  item.State,
			Type:   t,
		})
	}
	return hits, false, nil
}

// searchIssuesRaw is used in tests to decode JSON from a test server without oauth2.
// It is unexported and only called via the test helper.
func searchIssuesRaw(data []byte) ([]SearchHit, error) {
	var raw searchIssuesResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, len(raw.Items))
	for _, item := range raw.Items {
		t := "issue"
		if item.PullRequest != nil && strings.TrimSpace(item.PullRequest.URL) != "" {
			t = "pr"
		}
		hits = append(hits, SearchHit{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			URL:    item.HTMLURL,
			State:  item.State,
			Type:   t,
		})
	}
	return hits, nil
}
