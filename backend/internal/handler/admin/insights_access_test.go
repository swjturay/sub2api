package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingsRejectInsightsAccessSyncTargetsBeforeSaving(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, key := range []string{"dingtalk_connect_sync_corp_email_attr_key", "dingtalk_connect_sync_display_name_attr_key", "dingtalk_connect_sync_dept_attr_key"} {
		h := &SettingHandler{}
		r := gin.New()
		r.PUT("/settings", h.UpdateSettings)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/settings", strings.NewReader(`{"`+key+`":" insights_access "}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, 400, w.Code)
		require.Contains(t, w.Body.String(), "identity synchronization target")
	}
	// A legacy mapping cannot rename the system definition during settings repair.
	(&SettingHandler{}).ensureUserAttributeDefinition(t.Context(), "insights_access", "overwrite", "overwrite", "text")
}
