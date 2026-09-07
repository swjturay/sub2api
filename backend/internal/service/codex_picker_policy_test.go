package service

import (
	"encoding/json"
	"testing"
)

func TestCodexPickerPolicy(t *testing.T) {
	cases := []struct {
		name, slug, preferred string
		hide                  bool
	}{
		{"astra alias", "gpt-6", "gpt-6-astra", true},
		{"sol alias", "gpt-5.6", "gpt-5.6-sol", true},
		{"high duplicate", "gemini-3.1-pro-high", "gemini-3.1-pro", true},
		{"preview duplicate", "gemini-3.1-pro-preview", "gemini-3.1-pro", true},
		{"standalone alias", "gpt-6", "", false},
		{"standalone variant", "gemini-3.1-pro-high", "", false},
		{"independent low", "gemini-3.1-pro-low", "gemini-3.1-pro", false},
		{"independent tier", "gemini-3.6-flash-tiered", "gemini-3.6-flash", false},
		{"independent high", "gemini-3.6-flash-high", "gemini-3.6-flash", false},
		{"unknown chat", "chat_custom", "", false},
	}
	for slug := range codexPickerDedicatedModels {
		cases = append(cases, struct {
			name, slug, preferred string
			hide                  bool
		}{slug, slug, "", true})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models := []map[string]any{{"slug": tc.slug, "visibility": "list", "supported_in_api": true, "future_field": 42}}
			if tc.preferred != "" {
				models = append(models, map[string]any{"slug": tc.preferred, "visibility": "list", "supported_in_api": true})
			}
			body, _ := json.Marshal(map[string]any{"models": models, "extra": "keep"})
			got, err := FilterCodexPickerManifest(body)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Models []map[string]any `json:"models"`
				Extra  string           `json:"extra"`
			}
			if err := json.Unmarshal(got, &result); err != nil {
				t.Fatal(err)
			}
			want := "list"
			if tc.hide {
				want = "hide"
			}
			if result.Models[0]["visibility"] != want || result.Models[0]["slug"] != tc.slug || result.Models[0]["future_field"] != float64(42) || result.Extra != "keep" || len(result.Models) != len(models) {
				t.Fatalf("unexpected result: %s", got)
			}
			if len(models) > 1 && result.Models[1]["visibility"] != "list" {
				t.Fatal("preferred model hidden")
			}
			again, err := FilterCodexPickerManifest(got)
			if err != nil || string(again) != string(got) {
				t.Fatal("policy not idempotent")
			}
		})
	}
}

func TestCodexPickerUnavailablePreferredModel(t *testing.T) {
	for _, preferred := range []string{`{"slug":"gpt-6-astra","visibility":"hide","supported_in_api":true}`, `{"slug":"gpt-6-astra","visibility":"list","supported_in_api":false}`} {
		body := []byte(`{"models":[{"slug":"gpt-6","visibility":"list","supported_in_api":true},` + preferred + `]}`)
		got, err := FilterCodexPickerManifest(body)
		if err != nil || string(got) != string(body) {
			t.Fatalf("must preserve lone usable alias: %s, %v", got, err)
		}
	}
}

func TestCodexPickerMalformedManifest(t *testing.T) {
	for _, body := range []string{`{`, `{}`, `{"models":{}}`} {
		if _, err := FilterCodexPickerManifest([]byte(body)); err == nil {
			t.Fatalf("expected error for %s", body)
		}
	}
}
