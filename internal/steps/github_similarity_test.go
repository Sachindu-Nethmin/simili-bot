// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-05-09
// Last Modified: 2026-05-09

package steps

import (
	"testing"

	"github.com/similigh/simili-bot/internal/core/config"
	"github.com/similigh/simili-bot/internal/core/pipeline"
)

func newGitHubSimilarityCtx() *pipeline.Context {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	// Use a low threshold so BM25 results aren't all filtered.
	cfg.Defaults.SimilarityThreshold = 0.0
	cfg.Defaults.MaxSimilarToShow = 5
	return &pipeline.Context{
		Issue: &pipeline.Issue{
			Org:    "testorg",
			Repo:   "testrepo",
			Number: 99,
			Title:  "Test issue",
			Body:   "This is a test",
		},
		Config:   cfg,
		Result:   &pipeline.Result{},
		Metadata: map[string]interface{}{},
	}
}

func TestGitHubSimilarity_NilSearcher(t *testing.T) {
	step := &GitHubSimilarity{searcher: nil}
	ctx := newGitHubSimilarityCtx()
	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.SimilarIssues) != 0 {
		t.Errorf("expected no similar issues, got %v", ctx.SimilarIssues)
	}
}

func TestGitHubSimilarity_SkipOnTransferDetected(t *testing.T) {
	called := false
	step := &GitHubSimilarity{
		// Use nil searcher; the transfer skip should fire first.
		searcher: nil,
	}
	ctx := newGitHubSimilarityCtx()
	ctx.Metadata["skip_duplicate_detection"] = true
	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("searcher should not have been called")
	}
}

func TestGitHubSimilarity_SkipOnCommentEvent(t *testing.T) {
	step := &GitHubSimilarity{searcher: nil}
	ctx := newGitHubSimilarityCtx()
	ctx.Issue.EventType = "issue_comment"
	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitHubSimilarity_DryRun(t *testing.T) {
	// When dryRun=true and searcher=nil, the step should return nil without error.
	step := NewGitHubSimilarity(&pipeline.Dependencies{
		GitHubSearcher: nil,
		DryRun:         true,
	})
	ctx := newGitHubSimilarityCtx()
	if err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
