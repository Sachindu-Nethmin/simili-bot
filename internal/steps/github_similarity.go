// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-05-09
// Last Modified: 2026-05-09

package steps

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	ghlib "github.com/google/go-github/v60/github"
	"github.com/similigh/simili-bot/internal/core/pipeline"
	githubpkg "github.com/similigh/simili-bot/internal/integrations/github"
)

const bm25CorpusCap = 500 // max issues fetched for BM25 fallback

// GitHubSimilarity finds similar issues using GitHub hybrid search + BM25 re-ranking.
// It requires no external vector DB or embedding API key; only GITHUB_TOKEN is needed.
type GitHubSimilarity struct {
	searcher *githubpkg.Searcher
	gh       *githubpkg.Client
	dryRun   bool
}

// NewGitHubSimilarity creates a GitHubSimilarity step from pipeline dependencies.
func NewGitHubSimilarity(deps *pipeline.Dependencies) *GitHubSimilarity {
	return &GitHubSimilarity{
		searcher: deps.GitHubSearcher,
		gh:       deps.GitHub,
		dryRun:   deps.DryRun,
	}
}

// Name returns the step identifier.
func (s *GitHubSimilarity) Name() string { return "github_similarity" }

// Run executes the GitHub-native similarity search.
func (s *GitHubSimilarity) Run(ctx *pipeline.Context) error {
	if s.searcher == nil {
		log.Printf("[github_similarity] No GitHub Searcher configured, skipping")
		return nil
	}
	if skip, ok := ctx.Metadata["skip_duplicate_detection"].(bool); ok && skip {
		log.Printf("[github_similarity] Skipping (transfer detected)")
		return nil
	}
	// Skip comment events — only run on new issues.
	if ctx.Issue.EventType == "issue_comment" || ctx.Issue.EventType == "pr_comment" {
		return nil
	}

	threshold := ctx.Config.Defaults.SimilarityThreshold
	limit := ctx.Config.Defaults.MaxSimilarToShow
	if limit <= 0 {
		limit = 5
	}
	fetchLimit := limit * 5 // fetch more candidates to re-rank
	if fetchLimit < 25 {
		fetchLimit = 25
	}

	// Build keyword query from the issue title + body.
	queryText := ctx.Issue.Title + " " + ctx.Issue.Body
	tokens := tokenize(queryText)
	// Use up to 15 tokens (longest first as a simple relevance heuristic).
	sort.Slice(tokens, func(i, j int) bool { return len(tokens[i]) > len(tokens[j]) })
	if len(tokens) > 15 {
		tokens = tokens[:15]
	}
	query := strings.Join(tokens, " ")

	var candidates []githubpkg.SearchHit
	usedFallback := false

	if s.dryRun {
		log.Printf("[github_similarity] DRY RUN: Would query GitHub hybrid search: %q", query)
		return nil
	}

	// Determine item type for search filter: PRs search for similar PRs, issues for issues.
	itemType := "issue"
	if ctx.Issue.EventType == "pull_request" {
		itemType = "pr"
	}

	bm25Fallback := ctx.Config.Search.BM25Fallback == nil || *ctx.Config.Search.BM25Fallback
	backend := ctx.Config.Search.Backend

	// Tier 1: GitHub hybrid search (skipped when backend is explicitly "bm25").
	if backend != "bm25" {
		hits, rateLimited, err := s.searcher.SearchIssues(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, query, itemType, fetchLimit)
		if err != nil {
			log.Printf("[github_similarity] GitHub search error: %v", err)
		}
		if rateLimited {
			log.Printf("[github_similarity] WARN: GitHub search rate-limited")
		}
		if !rateLimited && err == nil && len(hits) > 0 {
			candidates = hits
		}
	}

	// Tier 2: BM25 over ListIssues — used when backend is "bm25", or as fallback
	// when tier 1 returned nothing and bm25_fallback is enabled.
	if len(candidates) == 0 && (backend == "bm25" || bm25Fallback) {
		usedFallback = true
		if s.gh == nil {
			log.Printf("[github_similarity] No GitHub client for BM25 fallback, skipping")
			return nil
		}
		log.Printf("[github_similarity] Falling back to BM25 over ListIssues")
		var fetchErr error
		candidates, fetchErr = s.fetchAllIssues(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, itemType)
		if fetchErr != nil {
			log.Printf("[github_similarity] BM25 fallback list error: %v", fetchErr)
			return nil
		}
	}

	if len(candidates) == 0 {
		log.Printf("[github_similarity] No candidates found for #%d", ctx.Issue.Number)
		return nil
	}

	if usedFallback {
		log.Printf("[github_similarity] BM25 fallback: scoring %d issues", len(candidates))
	} else {
		log.Printf("[github_similarity] BM25 re-ranking %d GitHub search hits", len(candidates))
	}

	// BM25 re-rank.
	queryTokens := tokenize(queryText)
	bodies := make([]string, len(candidates))
	for i, c := range candidates {
		bodies[i] = c.Title + " " + c.Body
	}
	scores := bm25Score(queryTokens, bodies)

	// Build results, filter self and apply threshold.
	type scored struct {
		hit   githubpkg.SearchHit
		score float64
	}
	var results []scored
	for i, hit := range candidates {
		if hit.Number == ctx.Issue.Number {
			continue
		}
		if scores[i] < threshold {
			continue
		}
		results = append(results, scored{hit: hit, score: scores[i]})
	}

	// Sort descending by BM25 score.
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > limit {
		results = results[:limit]
	}

	similar := make([]pipeline.SimilarIssue, len(results))
	for i, r := range results {
		similar[i] = pipeline.SimilarIssue{
			Number:     r.hit.Number,
			Title:      r.hit.Title,
			Body:       r.hit.Body,
			URL:        r.hit.URL,
			State:      r.hit.State,
			Type:       r.hit.Type,
			Similarity: r.score,
		}
	}

	ctx.SimilarIssues = similar
	ctx.Result.SimilarFound = similar
	log.Printf("[github_similarity] Found %d similar issues for #%d (threshold: %.2f)",
		len(similar), ctx.Issue.Number, threshold)
	return nil
}

// fetchAllIssues pages through the GitHub Issues list API up to bm25CorpusCap items.
// itemType filters the result: "issue" for issues, "pr" for pull requests, "" for both.
func (s *GitHubSimilarity) fetchAllIssues(ctx context.Context, org, repo, itemType string) ([]githubpkg.SearchHit, error) {
	var all []githubpkg.SearchHit
	opts := &ghlib.IssueListByRepoOptions{
		State: "open",
		ListOptions: ghlib.ListOptions{
			PerPage: 50,
		},
	}
	for {
		issues, resp, err := s.gh.ListIssues(ctx, org, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list issues: %w", err)
		}
		for _, iss := range issues {
			if len(all) >= bm25CorpusCap {
				log.Printf("[github_similarity] WARN: BM25 corpus capped at %d issues", bm25CorpusCap)
				return all, nil
			}
			if iss.GetNumber() == 0 {
				continue
			}
			isPR := iss.IsPullRequest()
			if itemType == "issue" && isPR {
				continue
			}
			if itemType == "pr" && !isPR {
				continue
			}
			t := "issue"
			if isPR {
				t = "pr"
			}
			body := ""
			if iss.Body != nil {
				body = *iss.Body
			}
			all = append(all, githubpkg.SearchHit{
				Number: iss.GetNumber(),
				Title:  iss.GetTitle(),
				Body:   body,
				URL:    iss.GetHTMLURL(),
				State:  iss.GetState(),
				Type:   t,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}
