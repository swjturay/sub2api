package service

import (
	"context"
	"testing"
)

func TestListInsightsCatalogUsesOfficialAllowlistWithoutRuntimeRepositories(t *testing.T) {
	catalog, err := (&ModelPlazaService{}).ListInsightsCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]struct{}, len(catalog))
	for _, model := range catalog {
		seen[model.Platform+":"+model.Name] = struct{}{}
		if model.Profile.Description == nil || len(model.Profile.Sources) == 0 {
			t.Fatalf("incomplete official profile: %s:%s", model.Platform, model.Name)
		}
	}
	for _, required := range []string{"openai:gpt-6-astra", "antigravity:gemini-3.8-flash-high", "deepseek:deepseek-v4.1-flash", "zhipu:glm-5.3-flash"} {
		if _, ok := seen[required]; !ok {
			t.Fatalf("official model missing: %s", required)
		}
	}
	if _, ok := seen["openai:codex-auto-review"]; ok {
		t.Fatal("internal usage probe model leaked into Insights catalog")
	}
}
