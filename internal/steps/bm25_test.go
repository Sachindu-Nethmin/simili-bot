// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-05-09
// Last Modified: 2026-05-09

package steps

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! foo-bar baz 42 a")
	want := map[string]bool{
		"hello": true,
		"world": true,
		"foo":   true,
		"bar":   true,
		"baz":   true,
		"42":    true,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens %v, want %d", len(tokens), tokens, len(want))
	}
	for _, tok := range tokens {
		if !want[tok] {
			t.Errorf("unexpected token %q", tok)
		}
	}
}

func TestTokenize_SingleCharFiltered(t *testing.T) {
	tokens := tokenize("a b c de")
	if len(tokens) != 1 || tokens[0] != "de" {
		t.Fatalf("expected only [de], got %v", tokens)
	}
}

func TestBM25Score_EmptyInputs(t *testing.T) {
	if s := bm25Score(nil, nil); len(s) != 0 {
		t.Fatalf("expected empty slice, got %v", s)
	}
	s := bm25Score([]string{"x"}, nil)
	if len(s) != 0 {
		t.Fatalf("expected empty slice for nil docs, got %v", s)
	}
	s = bm25Score(nil, []string{"some doc"})
	if len(s) != 1 {
		t.Fatalf("expected len 1, got %v", s)
	}
}

func TestBM25Score_Ranking(t *testing.T) {
	query := []string{"duplicate", "issue"}
	docs := []string{
		"duplicate issue duplicate issue duplicate issue", // high freq
		"this is a duplicate issue",                       // medium freq
		"unrelated topic about something else",            // no match
	}
	scores := bm25Score(query, docs)
	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %v", scores)
	}
	if !(scores[0] > scores[1]) {
		t.Errorf("expected scores[0] > scores[1], got %.4f vs %.4f", scores[0], scores[1])
	}
	if !(scores[1] > scores[2]) {
		t.Errorf("expected scores[1] > scores[2], got %.4f vs %.4f", scores[1], scores[2])
	}
	if scores[2] != 0 {
		t.Errorf("expected scores[2] == 0, got %.4f", scores[2])
	}
}

func TestBM25Score_Normalization(t *testing.T) {
	query := []string{"bug", "fix"}
	docs := []string{
		"bug fix needed here",
		"this is a fix for the bug reported",
		"nothing relevant",
	}
	scores := bm25Score(query, docs)
	for i, s := range scores {
		if s < 0 || s > 1.0001 {
			t.Errorf("score[%d] = %.4f out of [0,1]", i, s)
		}
	}
}

func TestBM25Score_ExactMatch(t *testing.T) {
	query := []string{"unique"}
	docs := []string{"unique"}
	scores := bm25Score(query, docs)
	if len(scores) != 1 || scores[0] != 1.0 {
		t.Fatalf("expected [1.0], got %v", scores)
	}
}
