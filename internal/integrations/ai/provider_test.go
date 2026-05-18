package ai

import "testing"

func TestResolveProviderPrefersGeminiWhenBothEnvKeysSet(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gemini-env-key")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	provider, key, err := ResolveProvider("config-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected provider %q, got %q", ProviderGemini, provider)
	}
	if key != "gemini-env-key" {
		t.Fatalf("expected Gemini env key, got %q", key)
	}
}

func TestResolveProviderUsesOpenAIEnvWhenGeminiMissing(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	provider, key, err := ResolveProvider("config-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderOpenAI {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, provider)
	}
	if key != "sk-openai-env-key" {
		t.Fatalf("expected OpenAI env key, got %q", key)
	}
}

func TestResolveProviderFallsBackToConfigKeyInference(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	provider, key, err := ResolveProvider("sk-config-openai-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderOpenAI {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, provider)
	}
	if key != "sk-config-openai-key" {
		t.Fatalf("expected config key passthrough, got %q", key)
	}

	provider, key, err = ResolveProvider("gemini-config-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected provider %q, got %q", ProviderGemini, provider)
	}
	if key != "gemini-config-key" {
		t.Fatalf("expected config key passthrough, got %q", key)
	}
}

func TestResolveProviderErrorsWhenNoKeyAvailable(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "") // ensure zero-config fallback is also absent

	_, _, err := ResolveProvider("")
	if err == nil {
		t.Fatal("expected error when no provider key is set")
	}
}

func TestResolveProviderGitHubModelsExplicitHint(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_test_token")

	provider, key, err := ResolveProvider("", string(ProviderGitHubModels))
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGitHubModels {
		t.Fatalf("expected provider %q, got %q", ProviderGitHubModels, provider)
	}
	if key != "ghs_test_token" {
		t.Fatalf("expected GITHUB_TOKEN value, got %q", key)
	}
}

func TestResolveProviderGitHubModelsFallbackWhenNoAIKeys(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_fallback_token")

	provider, key, err := ResolveProvider("")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGitHubModels {
		t.Fatalf("expected zero-config fallback to %q, got %q", ProviderGitHubModels, provider)
	}
	if key != "ghs_fallback_token" {
		t.Fatalf("expected GITHUB_TOKEN value, got %q", key)
	}
}

func TestResolveProviderGitHubModelsExplicitRequiresToken(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")

	_, _, err := ResolveProvider("", string(ProviderGitHubModels))
	if err == nil {
		t.Fatal("expected error when github_models is selected but GITHUB_TOKEN is unset")
	}
}

func TestResolveProviderGitHubModelsDoesNotOverrideExplicitAIKeys(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_token")

	// No explicit hint — GEMINI_API_KEY should still win.
	provider, _, err := ResolveProvider("")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected %q when GEMINI_API_KEY is set, got %q", ProviderGemini, provider)
	}
}

func TestNewLLMClientOpenAIDefaultModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	client, err := NewLLMClient("", "")
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if client.Provider() != string(ProviderOpenAI) {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, client.Provider())
	}
	if client.Model() != "gpt-5.2" {
		t.Fatalf("expected default OpenAI model %q, got %q", "gpt-5.2", client.Model())
	}
}

func TestNewLLMClientOpenAIExplicitModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	client, err := NewLLMClient("", "", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if client.Provider() != string(ProviderOpenAI) {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, client.Provider())
	}
	if client.Model() != "gpt-4o-mini" {
		t.Fatalf("expected model %q, got %q", "gpt-4o-mini", client.Model())
	}
}

func TestNewLLMClientGitHubModelsDefaultModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_test_token")

	client, err := NewLLMClient("", string(ProviderGitHubModels))
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if client.Provider() != string(ProviderGitHubModels) {
		t.Fatalf("expected provider %q, got %q", ProviderGitHubModels, client.Provider())
	}
	if client.Model() != "gpt-4o-mini" {
		t.Fatalf("expected default GitHub Models model %q, got %q", "gpt-4o-mini", client.Model())
	}
}

func TestNewLLMClientGitHubModelsExplicitModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_test_token")

	client, err := NewLLMClient("", string(ProviderGitHubModels), "gpt-4o")
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if client.Provider() != string(ProviderGitHubModels) {
		t.Fatalf("expected provider %q, got %q", ProviderGitHubModels, client.Provider())
	}
	if client.Model() != "gpt-4o" {
		t.Fatalf("expected model %q, got %q", "gpt-4o", client.Model())
	}
}

func TestNewLLMClientGitHubModelsFallback(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_fallback_token")

	// No provider hint — should auto-select github_models via zero-config fallback.
	client, err := NewLLMClient("", "")
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if client.Provider() != string(ProviderGitHubModels) {
		t.Fatalf("expected zero-config fallback to %q, got %q", ProviderGitHubModels, client.Provider())
	}
}

func TestNewEmbedderGitHubModelsDefaultModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_test_token")

	embedder, err := NewEmbedder("", "", string(ProviderGitHubModels))
	if err != nil {
		t.Fatalf("NewEmbedder returned error: %v", err)
	}
	t.Cleanup(func() { _ = embedder.Close() })

	if embedder.Provider() != string(ProviderGitHubModels) {
		t.Fatalf("expected provider %q, got %q", ProviderGitHubModels, embedder.Provider())
	}
	if embedder.Model() != "text-embedding-3-small" {
		t.Fatalf("expected default model %q, got %q", "text-embedding-3-small", embedder.Model())
	}
	if embedder.Dimensions() != 1536 {
		t.Fatalf("expected 1536 dimensions, got %d", embedder.Dimensions())
	}
}

func TestNewEmbedderGitHubModelsFallback(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "ghs_fallback_token")

	// No provider hint — auto-selects github_models via zero-config fallback.
	embedder, err := NewEmbedder("", "")
	if err != nil {
		t.Fatalf("NewEmbedder returned error: %v", err)
	}
	t.Cleanup(func() { _ = embedder.Close() })

	if embedder.Provider() != string(ProviderGitHubModels) {
		t.Fatalf("expected zero-config fallback to %q, got %q", ProviderGitHubModels, embedder.Provider())
	}
}

func TestResolveProviderExplicitGeminiHintWinsOverOpenAI(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("OPENAI_API_KEY", "sk-openai-key")

	// Explicit gemini hint should select Gemini even though both keys are set.
	provider, key, err := ResolveProvider("", string(ProviderGemini))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected %q, got %q", ProviderGemini, provider)
	}
	if key != "gemini-key" {
		t.Fatalf("expected GEMINI_API_KEY value, got %q", key)
	}
}

func TestResolveProviderExplicitOpenAIHintWinsOverGemini(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("OPENAI_API_KEY", "sk-openai-key")

	// Explicit openai hint should select OpenAI even though GEMINI_API_KEY is set.
	provider, key, err := ResolveProvider("", string(ProviderOpenAI))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != ProviderOpenAI {
		t.Fatalf("expected %q, got %q", ProviderOpenAI, provider)
	}
	if key != "sk-openai-key" {
		t.Fatalf("expected OPENAI_API_KEY value, got %q", key)
	}
}

func TestResolveProviderExplicitGeminiHintFallsBackToConfigKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	provider, key, err := ResolveProvider("config-gemini-key", string(ProviderGemini))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected %q, got %q", ProviderGemini, provider)
	}
	if key != "config-gemini-key" {
		t.Fatalf("expected config api_key, got %q", key)
	}
}

func TestResolveProviderExplicitGeminiHintErrorsWhenNoKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, _, err := ResolveProvider("", string(ProviderGemini))
	if err == nil {
		t.Fatal("expected error when gemini is explicitly requested but no key is available")
	}
}

func TestResolveProviderExplicitOpenAIHintErrorsWhenNoKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, _, err := ResolveProvider("", string(ProviderOpenAI))
	if err == nil {
		t.Fatal("expected error when openai is explicitly requested but no key is available")
	}
}
