package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestInsightsPersonalUsageRequiresJWTSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInsightsHandler(nil, &config.Config{Timezone: "Asia/Shanghai"}, nil, nil)
	r := gin.New()
	r.GET("/usage", h.PersonalUsage)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/usage", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMapTodaySubscriptionsUsesAccountingProgress(t *testing.T) {
	reset := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	items := mapTodaySubscriptions([]service.UserSubscription{{ID: 1}, {ID: 2}, {ID: 3, DailyUsageUSD: 4}}, []service.SubscriptionProgress{{ID: 1, GroupName: "daily", Daily: &service.UsageWindowProgress{UsedUSD: 12, LimitUSD: 10, RemainingUSD: 0, ResetsAt: reset}}, {ID: 2, GroupName: "weekly", Weekly: &service.UsageWindowProgress{UsedUSD: 2, LimitUSD: 20, RemainingUSD: 18, ResetsAt: reset}}})
	if len(items) != 3 || items[0].Window != "daily" || !items[0].OverLimit || items[0].Currency != "USD" || items[1].Window != "daily" || !items[1].Unlimited || items[1].LimitAmount != nil || items[1].UsedAmount != 0 || !items[2].Unlimited || items[2].UsedAmount != 4 || items[2].Currency != "USD" {
		t.Fatalf("items=%+v", items)
	}
}

func TestInsightsRejectsInvalidRangeBeforeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInsightsHandler(nil, &config.Config{Timezone: "Asia/Shanghai"}, nil, nil)
	r := gin.New()
	r.GET("/usage", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		h.PersonalUsage(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/usage?from=2026-09-23&to=2026-09-22", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInsightsRejectsFutureHeatmapYear(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInsightsHandler(nil, &config.Config{Timezone: "Asia/Shanghai"}, nil, nil)
	r := gin.New()
	r.GET("/heatmap", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		h.Heatmap(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/heatmap?year=2999", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func authenticatedInsightsRouter(path string, handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.GET(path, func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		handler(c)
	})
	return r
}

func TestInsightsRejectsInvalidGranularityBeforeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInsightsHandler(nil, &config.Config{Timezone: "Asia/Shanghai"}, nil, nil)
	r := authenticatedInsightsRouter("/usage", h.PersonalUsage)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/usage?from=2026-09-20&to=2026-09-21&granularity=hour", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInsightsRejectsMalformedCursorBeforeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInsightsHandler(nil, &config.Config{Timezone: "Asia/Shanghai"}, nil, nil)
	r := authenticatedInsightsRouter("/logs", h.UsageLogs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?from=2026-09-20&to=2026-09-21&cursor=not-base64!", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInsightsReturns422ForExpiredDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewInsightsHandler(nil, &config.Config{Timezone: "Asia/Shanghai"}, nil, nil)
	r := authenticatedInsightsRouter("/logs", h.UsageLogs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?from=2020-01-01&to=2020-01-02", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
