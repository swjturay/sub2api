package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestCodexPickerResponseFinalETag(t *testing.T) {
	body := []byte(`{"models":[{"slug":"gpt-6","visibility":"list","supported_in_api":true},{"slug":"gpt-6-astra","visibility":"list","supported_in_api":true}]}`)
	sourceTag := service.CodexModelsManifestETag(body)
	source := &service.CodexModelsManifest{Body: body, ETag: sourceTag}
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
