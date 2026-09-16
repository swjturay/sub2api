package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexPickerResponseFinalETag(t *testing.T) {
	body := []byte(`{"models":[{"slug":"gpt-6","visibility":"list","supported_in_api":true},{"slug":"gpt-6-astra","visibility":"list","supported_in_api":true}]}`)
	sourceTag := service.CodexModelsManifestETag(body)
	source := &service.OpenAIModelsResponse{Body: body, ETag: sourceTag}
	request := func(tag string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.153.4", nil)
		c.Request.Header.Set("If-None-Match", tag)
		writeCodexModelsManifestResponse(c, source)
		return w
	}
	first := request(sourceTag)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"hide"`) {
		t.Fatalf("expected fresh filtered body: %s", first.Body.String())
	}
	tag := first.Header().Get("ETag")
	if tag == sourceTag || tag != service.CodexModelsManifestETag(first.Body.Bytes()) {
		t.Fatal("ETag must describe final body")
	}
	second := request(tag)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatal("filtered ETag must return empty 304")
	}
	if source.ETag != sourceTag || string(source.Body) != string(body) || source.NotModified {
		t.Fatal("source cache mutated")
	}
}

func TestCodexPickerDiscoveryPathsRespectAllowlistAndFinalETag(t *testing.T) {
	for _, mode := range []string{"configured", "pinned", "scheduler", "fallback"} {
		for _, aliasOnly := range []bool{false, true} {
			name := mode + "/all-models"
			if aliasOnly {
				name = mode + "/alias-only"
			}
			t.Run(name, func(t *testing.T) {
				account := newPinnedCodexAccount(2, service.StatusActive, true, false)
				if mode != "scheduler" {
					account.Credentials["model_mapping"] = map[string]any{
						"gpt-6": "source-alias", "gpt-6-astra": "source-preferred", "chat_20706": "source-dedicated",
					}
				}
				body := `{"object":"list","data":[{"id":"source-alias"},{"id":"source-preferred"},{"id":"source-dedicated"}]}`
				if mode == "scheduler" {
					body = `{"object":"list","data":[{"id":"gpt-6"},{"id":"gpt-6-astra"},{"id":"chat_20706"}]}`
				}
				upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{2: body}}
				accounts := []service.Account{account}
				group := &service.Group{ID: 93, Platform: service.PlatformOpenAI}
				if mode == "pinned" {
					other := newPinnedCodexAccount(1, service.StatusActive, true, false)
					other.Credentials["model_mapping"] = map[string]any{"local-only": "local-only"}
					accounts = append(accounts, other)
					group.CodexModelsManifestConfig = service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{2}}
				}
				if mode == "fallback" {
					group.CodexModelsManifestConfig = service.GroupCodexModelsManifestConfig{
						Enabled: true, AccountIDs: []int64{99}, FallbackToScheduler: true,
					}
				}
				if aliasOnly {
					group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6"}}
				}
				handler := newPinnedCodexTestHandler(accounts, upstream, 3)
				first := performCodexModelsRequestForGroup(t, handler, group, "")
				require.Equal(t, http.StatusOK, first.Code, first.Body.String())
				var manifest struct {
					Models []struct {
						Slug       string `json:"slug"`
						Visibility string `json:"visibility"`
					} `json:"models"`
				}
				require.NoError(t, json.Unmarshal(first.Body.Bytes(), &manifest))
				visibility := make(map[string]string)
				for _, model := range manifest.Models {
					visibility[model.Slug] = model.Visibility
				}
				if aliasOnly {
					require.Equal(t, map[string]string{"gpt-6": "list"}, visibility, "an excluded preferred model must not hide the remaining alias")
				} else {
					require.Equal(t, "hide", visibility["gpt-6"])
					require.Equal(t, "list", visibility["gpt-6-astra"])
					require.Equal(t, "hide", visibility["chat_20706"])
				}
				require.NotContains(t, visibility, "local-only", "pinned discovery must take precedence over local mappings")
				require.NotContains(t, visibility, "source-alias", "upstream mapping must precede picker policy")
				etag := first.Header().Get("ETag")
				require.Equal(t, service.CodexModelsManifestETag(first.Body.Bytes()), etag)
				second := performCodexModelsRequestForGroup(t, handler, group, etag)
				require.Equal(t, http.StatusNotModified, second.Code, second.Body.String())
				require.Empty(t, second.Body.Bytes())
				if mode == "configured" {
					require.Empty(t, upstream.accountIDs())
				} else {
					require.Equal(t, []int64{2}, upstream.accountIDs(), "discovery should reuse the upstream cache")
				}
			})
		}
	}
}
