// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-18

package config

import (
	"strings"
	"testing"
)

// TestConfigDefaults verifies that default values are applied correctly.
func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.Defaults.SimilarityThreshold != 0.65 {
		t.Errorf("Expected SimilarityThreshold to be 0.65, got %f", cfg.Defaults.SimilarityThreshold)
	}

	if cfg.Defaults.MaxSimilarToShow != 5 {
		t.Errorf("Expected MaxSimilarToShow to be 5, got %d", cfg.Defaults.MaxSimilarToShow)
	}

	if cfg.Embedding.Provider != "gemini" {
		t.Errorf("Expected Embedding.Provider to be 'gemini', got %s", cfg.Embedding.Provider)
	}
}

func TestLLMConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.LLM.Provider != "gemini" {
		t.Errorf("Expected LLM.Provider to be 'gemini', got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("Expected LLM.Model to be 'gemini-2.5-flash', got %s", cfg.LLM.Model)
	}
}

func TestMergeConfigsLLM(t *testing.T) {
	parent := &Config{}
	parent.applyDefaults()

	child := &Config{
		LLM: LLMConfig{
			Model: "gemini-2.0-flash",
		},
	}

	merged := mergeConfigs(parent, child)
	if merged.LLM.Model != "gemini-2.0-flash" {
		t.Errorf("Expected merged LLM.Model to be 'gemini-2.0-flash', got %s", merged.LLM.Model)
	}
	if merged.LLM.Provider != "gemini" {
		t.Errorf("Expected merged LLM.Provider to be 'gemini', got %s", merged.LLM.Provider)
	}
}

func TestLoadConfigWithLLM(t *testing.T) {
	yamlContent := `
qdrant:
  url: "http://localhost:6334"
  api_key: "test-key"
  collection: "test"
embedding:
  provider: gemini
  api_key: "test-key"
llm:
  provider: gemini
  api_key: "test-key"
  model: gemini-2.5-flash
defaults:
  similarity_threshold: 0.7
`
	cfg, err := parseRaw([]byte(yamlContent))
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	// Manually apply defaults since parseRaw doesn't
	cfg.applyDefaults()

	// manually validate to ensure it passes
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("Expected LLM.Model 'gemini-2.5-flash', got '%s'", cfg.LLM.Model)
	}
	if cfg.LLM.Provider != "gemini" {
		t.Errorf("Expected LLM.Provider 'gemini', got '%s'", cfg.LLM.Provider)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Config",
			config: Config{
				Qdrant: QdrantConfig{
					URL:        "http://localhost",
					APIKey:     "key",
					Collection: "col",
				},
				Embedding: EmbeddingConfig{
					Provider: "gemini",
					APIKey:   "key",
				},
				LLM: LLMConfig{
					Provider: "gemini",
					APIKey:   "key",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing Qdrant URL",
			config: Config{
				Qdrant:    QdrantConfig{APIKey: "key", Collection: "col"},
				Embedding: EmbeddingConfig{Provider: "gemini", APIKey: "key"},
				LLM:       LLMConfig{Provider: "gemini", APIKey: "key"},
			},
			wantErr: true,
			errMsg:  "qdrant.url",
		},
		{
			name: "Missing Multiple Fields",
			config: Config{
				Qdrant:    QdrantConfig{Collection: "col"},
				Embedding: EmbeddingConfig{APIKey: "key"},
				LLM:       LLMConfig{Provider: "gemini"},
			},
			wantErr: true,
			errMsg:  "missing required configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestParseExtendsRef verifies extends reference parsing.
func TestParseExtendsRef(t *testing.T) {
	tests := []struct {
		name        string
		ref         string
		wantOrg     string
		wantRepo    string
		wantBranch  string
		wantPath    string
		expectError bool
	}{
		{
			name:       "valid ref with default path",
			ref:        "org/repo@main",
			wantOrg:    "org",
			wantRepo:   "repo",
			wantBranch: "main",
			wantPath:   ".github/simili.yaml",
		},
		{
			name:       "valid ref with custom path",
			ref:        "org/repo@main:custom/path.yaml",
			wantOrg:    "org",
			wantRepo:   "repo",
			wantBranch: "main",
			wantPath:   "custom/path.yaml",
		},
		{
			name:        "invalid ref missing branch",
			ref:         "org/repo",
			expectError: true,
		},
		{
			name:        "invalid ref missing repo",
			ref:         "org@main",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, branch, path, err := ParseExtendsRef(tt.ref)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for ref %s, got nil", tt.ref)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if org != tt.wantOrg {
				t.Errorf("Expected org %s, got %s", tt.wantOrg, org)
			}
			if repo != tt.wantRepo {
				t.Errorf("Expected repo %s, got %s", tt.wantRepo, repo)
			}
			if branch != tt.wantBranch {
				t.Errorf("Expected branch %s, got %s", tt.wantBranch, branch)
			}
			if path != tt.wantPath {
				t.Errorf("Expected path %s, got %s", tt.wantPath, path)
			}
		})
	}
}
