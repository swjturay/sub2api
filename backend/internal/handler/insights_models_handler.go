package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// InsightsModelsHandler owns the read-only official model catalog and comparison slice.
type InsightsModelsHandler struct {
	store    *insights.ModelStore
	catalog  *service.ModelPlazaService
	timezone string
}

func NewInsightsModelsHandler(db *sql.DB, cfg *config.Config, catalog *service.ModelPlazaService) *InsightsModelsHandler {
	tz := cfg.Timezone
	if tz == "" {
		tz = "UTC"
	}
	return &InsightsModelsHandler{store: insights.NewModelStore(db, tz), catalog: catalog, timezone: tz}
}

// RegisterRoutes exposes the stable v1 paths on already-authenticated groups.
func (h *InsightsModelsHandler) RegisterRoutes(user *gin.RouterGroup) {
	user.GET("/models", h.List)
	user.GET("/models/detail", h.Detail)
	user.GET("/models/compare", h.Compare)
}
func modelWindow(raw string) (time.Time, time.Time, string, bool) {
	now := time.Now()
	switch raw {
	case "", "24h":
		return now.Add(-24 * time.Hour), now, "hour", true
	case "7d":
		return now.Add(-7 * 24 * time.Hour), now, "day", true
	default:
		return time.Time{}, time.Time{}, "", false
	}
}

func pricingProjection(p *service.PlazaOfficialPricing) insights.ReferencePricing {
	out := insights.ReferencePricing{Status: "unknown", Items: []insights.ReferencePriceItem{}}
	if p == nil {
		return out
	}
	out.Status, out.Source = "available", "global_reference_pricing"
	add := func(kind string, price *float64, condition string) {
		if price != nil {
			out.Items = append(out.Items, insights.ReferencePriceItem{Kind: kind, Price: *price, Currency: "USD", Unit: "token", Condition: condition})
		}
	}
	add("input", p.InputPrice, "")
	add("output", p.OutputPrice, "")
	add("cache_write_5m", p.CacheWritePrice, "")
	add("cache_write_1h", p.CacheWrite1hPrice, "")
	add("cache_read", p.CacheReadPrice, "")
	for _, tier := range p.Intervals {
		condition := "min_tokens=" + strconv.Itoa(tier.MinTokens)
		if tier.MaxTokens != nil {
			condition += ",max_tokens=" + strconv.Itoa(*tier.MaxTokens)
		}
		if tier.TierLabel != "" {
			condition += ",tier=" + tier.TierLabel
		}
		add("input", tier.InputPrice, condition)
		add("output", tier.OutputPrice, condition)
		add("cache_write_5m", tier.CacheWritePrice, condition)
		add("cache_write_1h", tier.CacheWrite1hPrice, condition)
		add("cache_read", tier.CacheReadPrice, condition)
	}
	if len(out.Items) == 0 {
		out.Status = "unknown"
	}
	return out
}

func (h *InsightsModelsHandler) catalogViews(c *gin.Context, ids []insights.ModelIdentity, from, to time.Time, step string) ([]insights.ModelView, error) {
	models, err := h.catalog.ListInsightsCatalog(c.Request.Context())
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]service.InsightsCatalogModel, len(models))
	for _, model := range models {
		byKey[insights.NormalizeModelKey(insights.ModelIdentity{Platform: model.Platform, Name: model.Name})] = model
	}
	selected := make([]service.InsightsCatalogModel, 0, len(models))
	if len(ids) == 0 {
		selected = append(selected, models...)
	} else {
		for _, id := range ids {
			if model, ok := byKey[insights.NormalizeModelKey(id)]; ok {
				selected = append(selected, model)
			}
		}
	}
	perfIDs := make([]insights.ModelIdentity, 0, len(selected))
	for _, m := range selected {
		perfIDs = append(perfIDs, insights.ModelIdentity{Platform: m.Platform, Name: m.Name})
	}
	performance, err := h.store.Performance(c.Request.Context(), perfIDs, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]insights.ModelView, 0, len(selected))
	for _, m := range selected {
		id := insights.ModelIdentity{Platform: m.Platform, Name: m.Name, DisplayName: m.DisplayName}
		trend, e := h.store.Trend(c.Request.Context(), id, from, to, step)
		if e != nil {
			return nil, e
		}
		perf, ok := performance[insights.NormalizeModelKey(id)]
		if !ok {
			zero := 0.0
			perf.AverageTPM = &zero
			perf.AverageRPM = &zero
		}
		out = append(out, insights.ModelView{Identity: id, Configured: true, Profile: m.Profile, ReferencePricing: pricingProjection(m.OfficialPricing), Performance: perf, Trend: trend})
	}
	return out, nil
}
func (h *InsightsModelsHandler) List(c *gin.Context) {
	from, to, step, ok := modelWindow(c.DefaultQuery("window", "24h"))
	if !ok {
		response.BadRequest(c, "invalid window")
		return
	}
	views, err := h.catalogViews(c, nil, from, to, step)
	if err != nil {
		response.InternalError(c, "failed to query model catalog")
		return
	}
	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	platforms := c.QueryArray("platform")
	allowed := map[string]struct{}{}
	for _, p := range platforms {
		allowed[strings.ToLower(p)] = struct{}{}
	}
	filtered := views[:0]
	for _, v := range views {
		if q != "" && !strings.Contains(strings.ToLower(v.Identity.Name+" "+v.Identity.Platform), q) {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[strings.ToLower(v.Identity.Platform)]; !ok {
				continue
			}
		}
		filtered = append(filtered, v)
	}
	c.JSON(http.StatusOK, insights.Envelope{Data: map[string]any{"items": filtered}, Meta: insights.Meta{Timezone: h.timezone, GeneratedAt: time.Now(), Coverage: []insights.CoverageInfo{{Dataset: "model_catalog", Status: "complete"}, {Dataset: "model_profile", Status: "complete"}, {Dataset: "usage_detail", Status: "partial"}}}})
}

func (h *InsightsModelsHandler) Detail(c *gin.Context) {
	id, err := insights.ParseModelIdentity(c.Query("model"))
	if err != nil {
		response.BadRequest(c, "invalid model")
		return
	}
	from, to, step, ok := modelWindow(c.DefaultQuery("window", "24h"))
	if !ok {
		response.BadRequest(c, "invalid window")
		return
	}
	views, err := h.catalogViews(c, []insights.ModelIdentity{id}, from, to, step)
	if err != nil {
		response.InternalError(c, "failed to query model")
		return
	}
	if len(views) == 0 {
		response.NotFound(c, "model not found")
		return
	}
	c.JSON(http.StatusOK, insights.Envelope{Data: views[0], Meta: insights.Meta{Timezone: h.timezone, GeneratedAt: time.Now()}})
}

func (h *InsightsModelsHandler) Compare(c *gin.Context) {
	raw := c.QueryArray("model")
	if len(raw) < 2 || len(raw) > 4 {
		response.BadRequest(c, "compare requires 2 to 4 models")
		return
	}
	ids := make([]insights.ModelIdentity, 0, len(raw))
	seen := map[string]struct{}{}
	for _, v := range raw {
		id, err := insights.ParseModelIdentity(v)
		if err != nil {
			response.BadRequest(c, "invalid model")
			return
		}
		k := insights.NormalizeModelKey(id)
		if _, ok := seen[k]; ok {
			response.BadRequest(c, "duplicate model")
			return
		}
		seen[k] = struct{}{}
		ids = append(ids, id)
	}
	from, to, step, ok := modelWindow(c.DefaultQuery("window", "24h"))
	if !ok {
		response.BadRequest(c, "invalid window")
		return
	}
	views, err := h.catalogViews(c, ids, from, to, step)
	if err != nil {
		response.InternalError(c, "failed to compare models")
		return
	}
	if len(views) != len(ids) {
		response.NotFound(c, "model not found")
		return
	}
	type trendValue struct {
		Model       string                    `json:"model"`
		Performance insights.ModelPerformance `json:"performance"`
	}
	type trendPoint struct {
		At       time.Time    `json:"at"`
		Complete bool         `json:"complete"`
		Values   []trendValue `json:"values"`
	}
	byAt := map[time.Time]*trendPoint{}
	order := []time.Time{}
	for _, v := range views {
		key := v.Identity.Platform + ":" + v.Identity.Name
		for _, p := range v.Trend {
			point := byAt[p.At]
			if point == nil {
				point = &trendPoint{At: p.At, Complete: p.Complete, Values: []trendValue{}}
				byAt[p.At] = point
				order = append(order, p.At)
			}
			point.Values = append(point.Values, trendValue{key, p.Performance})
		}
	}
	trends := make([]trendPoint, 0, len(order))
	for _, at := range order {
		trends = append(trends, *byAt[at])
	}
	c.JSON(http.StatusOK, insights.Envelope{Data: map[string]any{"models": views, "trends": trends}, Meta: insights.Meta{Timezone: h.timezone, GeneratedAt: time.Now()}})
}
