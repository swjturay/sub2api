package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/gin-gonic/gin"
)

func TestInsightsModelsCompareValidatesCountBeforeDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &InsightsModelsHandler{}
	for _, target := range []string{"/api/v1/insights/models/compare?model=OpenAI:a", "/api/v1/insights/models/compare?model=a:b&model=c:d&model=e:f&model=g:h&model=i:j"} {
		r := gin.New()
		r.GET("/api/v1/insights/models/compare", h.Compare)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d", target, w.Code)
		}
	}
}

func TestInsightsModelsIdentityPreservesSlashAndFurtherColon(t *testing.T) {
	id, err := insights.ParseModelIdentity("OpenAI:org/model:latest")
	if err != nil || id.Platform != "OpenAI" || id.Name != "org/model:latest" {
		t.Fatalf("identity=%+v err=%v", id, err)
	}
}

func TestInsightsModelsRoutesAreReadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &InsightsModelsHandler{}
	h.RegisterRoutes(r.Group("/api/v1/insights"))
	for _, route := range r.Routes() {
		if route.Method != http.MethodGet {
			t.Fatalf("unexpected writable model route: %s %s", route.Method, route.Path)
		}
		if strings.Contains(route.Path, "/profile") {
			t.Fatalf("editable profile route remains registered: %s", route.Path)
		}
	}
}
