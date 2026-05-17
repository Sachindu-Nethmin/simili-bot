package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider identifies the active AI provider.
type Provider string

const (
	ProviderGemini       Provider = "gemini"
	ProviderOpenAI       Provider = "openai"
	ProviderGitHubModels Provider = "github_models"
)

const (
	openAIBaseURL       = "https://api.openai.com"
	gitHubModelsBaseURL = "https://models.inference.ai.azure.com"
)

// ResolveProvider selects provider/key using the optional hint, environment
// variables, and the config api_key.
//
// Selection order:
// 1. Explicit hint ("github_models", "gemini", "openai") — short-circuits env priority.
// 2. GEMINI_API_KEY env var (wins over OPENAI_API_KEY when both are set).
// 3. OPENAI_API_KEY env var.
// 4. Config api_key (provider inferred from key prefix).
// 5. Zero-config fallback — GITHUB_TOKEN present, no other AI key configured.
func ResolveProvider(apiKey string, providerHint ...string) (Provider, string, error) {
	hint := ""
	if len(providerHint) > 0 {
		hint = strings.TrimSpace(strings.ToLower(providerHint[0]))
	}

	geminiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	configKey := strings.TrimSpace(apiKey)

	// When the caller explicitly names a provider, honour it and skip env priority.
	switch Provider(hint) {
	case ProviderGitHubModels:
		token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
		if token == "" {
			return "", "", fmt.Errorf("provider %q requires GITHUB_TOKEN to be set", ProviderGitHubModels)
		}
		return ProviderGitHubModels, token, nil
	case ProviderGemini:
		if geminiKey != "" {
			return ProviderGemini, geminiKey, nil
		}
		if configKey != "" {
			return ProviderGemini, configKey, nil
		}
		return "", "", fmt.Errorf("provider %q requires GEMINI_API_KEY or api_key to be set", ProviderGemini)
	case ProviderOpenAI:
		if openAIKey != "" {
			return ProviderOpenAI, openAIKey, nil
		}
		if configKey != "" {
			return ProviderOpenAI, configKey, nil
		}
		return "", "", fmt.Errorf("provider %q requires OPENAI_API_KEY or api_key to be set", ProviderOpenAI)
	}

	// No explicit hint — fall back to env-key priority.
	switch {
	case geminiKey != "":
		return ProviderGemini, geminiKey, nil
	case openAIKey != "":
		return ProviderOpenAI, openAIKey, nil
	case configKey != "":
		return inferProviderFromKey(configKey), configKey, nil
	default:
		// Zero-config fallback: GITHUB_TOKEN is always available in Actions runners.
		if ghToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); ghToken != "" {
			return ProviderGitHubModels, ghToken, nil
		}
		return "", "", fmt.Errorf("no AI API key found: set GEMINI_API_KEY, OPENAI_API_KEY, or configure provider: github_models")
	}
}

func inferProviderFromKey(apiKey string) Provider {
	// OpenAI keys commonly use sk-* prefixes. Fall back to Gemini for compatibility.
	if strings.HasPrefix(strings.TrimSpace(apiKey), "sk-") {
		return ProviderOpenAI
	}
	return ProviderGemini
}

// callOpenAIJSON sends a POST request to the OpenAI-compatible API endpoint and
// decodes the JSON response into out. Pass a non-empty baseURL to override the
// default production URL (useful in tests with httptest servers).
func callOpenAIJSON(ctx context.Context, httpClient *http.Client, apiKey, baseURL, endpoint string, in, out interface{}) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("an API key or token is required for OpenAI-compatible requests")
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	if strings.TrimSpace(baseURL) == "" {
		baseURL = openAIBaseURL
	}

	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read OpenAI response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &openAIStatusError{Code: resp.StatusCode, Message: extractOpenAIErrorMessage(respBody)}
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	return nil
}

func extractOpenAIErrorMessage(body []byte) string {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		msg := strings.TrimSpace(errResp.Error.Message)
		if msg != "" {
			return msg
		}
	}
	return strings.TrimSpace(string(body))
}
