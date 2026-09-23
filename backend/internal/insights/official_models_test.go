package insights

import (
	"net/url"
	"strings"
	"testing"
)

func TestOfficialModelCatalogIsCuratedAndComplete(t *testing.T) {
	models := OfficialModelCatalog()
	if len(models) < 50 {
		t.Fatalf("catalog unexpectedly small: %d", len(models))
	}

	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		key := NormalizeModelKey(ModelIdentity{Platform: model.Platform, Name: model.Name})
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate model identity: %s", key)
		}
		seen[key] = struct{}{}
		if model.DisplayName == "" || model.Profile.Description == nil || len(model.Profile.UseCases) == 0 {
			t.Fatalf("incomplete profile for %s", key)
		}
		if len(model.Profile.Sources) == 0 {
			t.Fatalf("missing official source for %s", key)
		}
		for _, source := range model.Profile.Sources {
			parsed, err := url.Parse(source.URL)
			if err != nil || parsed.Scheme != "https" {
				t.Fatalf("invalid official source for %s: %q", key, source.URL)
			}
			switch strings.ToLower(parsed.Hostname()) {
			case "developers.openai.com", "platform.claude.com", "ai.google.dev", "api-docs.deepseek.com", "docs.z.ai":
			default:
				t.Fatalf("non-official source host for %s: %s", key, parsed.Hostname())
			}
		}
	}

	for _, required := range []string{
		"openai:gpt-5.2",
		"openai:gpt-5.3-codex-spark",
		"openai:gpt-6-astra",
		"antigravity:claude-fable-5-1",
		"antigravity:gemini-3.8-flash-high",
		"deepseek:deepseek-flash",
		"deepseek:deepseek-v4.1-flash",
		"opencode_go:deepseek-v4-pro",
		"zhipu:glm-5.3-flash",
		"zhipu:glm-5.3-flashx",
	} {
		if _, ok := seen[required]; !ok {
			t.Fatalf("required production model missing: %s", required)
		}
	}

	for _, forbidden := range []string{
		"openai:codex-auto-review",
		"openai:chat_5f4f5f0d",
		"openai:tab_test_model",
	} {
		if _, ok := seen[forbidden]; ok {
			t.Fatalf("internal model leaked into catalog: %s", forbidden)
		}
	}
}
