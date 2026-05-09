// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-05-09
// Last Modified: 2026-05-09

package steps

import (
	"math"
	"strings"
	"unicode"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// tokenize lowercases text, strips punctuation, and returns tokens with len ≥ 2.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	var tokens []string
	var buf strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
		} else {
			if buf.Len() >= 2 {
				tokens = append(tokens, buf.String())
			}
			buf.Reset()
		}
	}
	if buf.Len() >= 2 {
		tokens = append(tokens, buf.String())
	}
	return tokens
}

// bm25Score returns BM25 Okapi scores for query terms against each document,
// normalized to [0, 1] by dividing by the maximum score. Returns nil when
// docs is empty or all scores are zero.
func bm25Score(query []string, docs []string) []float64 {
	n := len(docs)
	if n == 0 || len(query) == 0 {
		return make([]float64, n)
	}

	// Tokenize all docs and compute lengths.
	tokenized := make([][]string, n)
	lengths := make([]int, n)
	totalLen := 0
	for i, d := range docs {
		toks := tokenize(d)
		tokenized[i] = toks
		lengths[i] = len(toks)
		totalLen += len(toks)
	}
	avgdl := float64(totalLen) / float64(n)

	// Build term frequency maps per document.
	tf := make([]map[string]int, n)
	for i, toks := range tokenized {
		m := make(map[string]int, len(toks))
		for _, t := range toks {
			m[t]++
		}
		tf[i] = m
	}

	// For each query term, compute IDF and accumulate scores.
	scores := make([]float64, n)
	for _, qt := range query {
		// n(qt): number of docs containing the term.
		nq := 0
		for i := range tf {
			if tf[i][qt] > 0 {
				nq++
			}
		}
		if nq == 0 {
			continue
		}
		idf := math.Log((float64(n)-float64(nq)+0.5)/(float64(nq)+0.5) + 1)
		for i := range scores {
			freq := float64(tf[i][qt])
			if freq == 0 {
				continue
			}
			dl := float64(lengths[i])
			denom := freq + bm25K1*(1-bm25B+bm25B*dl/avgdl)
			scores[i] += idf * (freq * (bm25K1 + 1)) / denom
		}
	}

	// Normalize to [0, 1].
	maxScore := 0.0
	for _, s := range scores {
		if s > maxScore {
			maxScore = s
		}
	}
	if maxScore == 0 {
		return scores
	}
	for i := range scores {
		scores[i] /= maxScore
	}
	return scores
}
