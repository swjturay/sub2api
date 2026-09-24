package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type InsightsHandler struct {
	query               *insights.Query
	loc                 *time.Location
	subscriptionService *service.SubscriptionService
	catalog             *service.ModelPlazaService
	usageRetentionDays  int
	errorRetentionDays  int
}

func NewInsightsHandler(db *sql.DB, cfg *config.Config, subscriptionService *service.SubscriptionService, catalog *service.ModelPlazaService) *InsightsHandler {
	tz := cfg.Timezone
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
		tz = "UTC"
	}
	var statisticsStart *time.Time
	if value := strings.TrimSpace(cfg.Insights.StatisticsStartDate); value != "" {
		if parsed, parseErr := time.ParseInLocation("2006-01-02", value, loc); parseErr == nil {
			statisticsStart = &parsed
		}
	}
	insights.ConfigureRuntimeWithOptions(db, tz, 4096, insights.RuntimeOptions{
		TrustedCollection: cfg.Insights.TrustedCollection,
		StatisticsStart:   statisticsStart,
	})
	usageDays := cfg.DashboardAgg.Retention.UsageLogsDays
	if usageDays <= 0 {
		usageDays = 365
	}
	errorDays := cfg.Ops.Cleanup.ErrorLogRetentionDays
	if errorDays <= 0 {
		errorDays = 30
	}
	return &InsightsHandler{query: insights.NewQuery(db, tz, statisticsStart), loc: loc, subscriptionService: subscriptionService, catalog: catalog, usageRetentionDays: usageDays, errorRetentionDays: errorDays}
}

func (h *InsightsHandler) Dimensions(c *gin.Context) {
	department, err := h.query.DepartmentDimension(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to resolve insights dimensions")
		return
	}
	models := make([]insights.ModelIdentity, 0)
	catalogStatus := "not_configured"
	if h.catalog != nil {
		catalog, err := h.catalog.ListInsightsCatalog(c.Request.Context())
		if err != nil {
			response.InternalError(c, "failed to load insights model catalog")
			return
		}
		catalogStatus = "complete"
		models = make([]insights.ModelIdentity, 0, len(catalog))
		for _, model := range catalog {
			models = append(models, insights.ModelIdentity{Platform: model.Platform, Name: model.Name, DisplayName: model.DisplayName})
		}
	}
	now := time.Now().In(h.loc)
	retainedFrom := time.Date(now.Year()-2, time.January, 1, 0, 0, 0, 0, h.loc)
	retentionEnd := time.Date(now.Year()+1, time.January, 1, 0, 0, 0, 0, h.loc)
	retentionDays := int(retentionEnd.Sub(retainedFrom).Hours() / 24)
	c.JSON(http.StatusOK, h.query.Envelope(map[string]any{
		"department": department,
		"models":     models,
		"retention":  map[string]any{"usage_detail_days": h.usageRetentionDays, "error_detail_days": h.errorRetentionDays, "daily_days": retentionDays, "daily_years": 3, "daily_from": retainedFrom.Format("2006-01-02")},
	}, []insights.CoverageInfo{{Dataset: "department_attribute", Status: department.Status}, {Dataset: "model_catalog", Status: catalogStatus}}))
}

type todaySubscription struct {
	ID              int64                             `json:"id"`
	Name            string                            `json:"name"`
	Window          string                            `json:"window,omitempty"`
	UsedAmount      float64                           `json:"used_amount"`
	LimitAmount     *float64                          `json:"limit_amount"`
	RemainingAmount *float64                          `json:"remaining_amount"`
	ResetAt         *time.Time                        `json:"reset_at"`
	Unlimited       bool                              `json:"unlimited"`
	OverLimit       bool                              `json:"over_limit"`
	Currency        string                            `json:"currency"`
	RecentUsage     []insights.SubscriptionUsagePoint `json:"recent_usage"`
}

func (h *InsightsHandler) Today(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	subs, err := h.subscriptionService.ListActiveUserSubscriptions(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "failed to load subscription quota")
		return
	}
	for i := range subs {
		if err := h.subscriptionService.CheckAndResetWindows(c.Request.Context(), &subs[i]); err != nil {
			response.InternalError(c, "failed to refresh subscription quota window")
			return
		}
	}
	progresses, err := h.subscriptionService.GetUserSubscriptionsWithProgress(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "failed to load subscription quota")
		return
	}
	items := mapTodaySubscriptions(subs, progresses)
	subscriptionIDs := make([]int64, 0, len(items))
	for _, item := range items {
		subscriptionIDs = append(subscriptionIDs, item.ID)
	}
	recentUsage, err := h.query.SubscriptionRecentUsage(c.Request.Context(), subject.UserID, subscriptionIDs)
	if err != nil {
		response.InternalError(c, "failed to query recent subscription usage")
		return
	}
	for i := range items {
		items[i].RecentUsage = recentUsage[items[i].ID]
	}
	now := time.Now().In(h.loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, h.loc)
	todayUsage, coverage, err := h.query.PersonalToday(c.Request.Context(), subject.UserID, from, from.AddDate(0, 0, 1))
	if err != nil {
		response.InternalError(c, "failed to query today usage")
		return
	}
	c.JSON(http.StatusOK, h.query.Envelope(map[string]any{
		"date":          from.Format("2006-01-02"),
		"subscriptions": items,
		"tokens":        todayUsage.Tokens,
		"actual_cost":   todayUsage.ActualCost,
	}, []insights.CoverageInfo{coverage}))
}

func mapTodaySubscriptions(subs []service.UserSubscription, progresses []service.SubscriptionProgress) []todaySubscription {
	byID := make(map[int64]service.SubscriptionProgress, len(progresses))
	for _, p := range progresses {
		byID[p.ID] = p
	}
	items := make([]todaySubscription, 0, len(subs))
	for _, sub := range subs {
		progress := byID[sub.ID]
		name := ""
		if sub.Group != nil {
			name = sub.Group.Name
		}
		if progress.GroupName != "" {
			name = progress.GroupName
		}
		value := progress.Daily
		if value == nil {
			items = append(items, todaySubscription{ID: sub.ID, Name: name, Window: "daily", UsedAmount: sub.DailyUsageUSD, Unlimited: true, Currency: "USD"})
			continue
		}
		limit, remaining, reset := value.LimitUSD, value.RemainingUSD, value.ResetsAt
		items = append(items, todaySubscription{ID: sub.ID, Name: name, Window: "daily", UsedAmount: value.UsedUSD, LimitAmount: &limit, RemainingAmount: &remaining, ResetAt: &reset, OverLimit: value.UsedUSD > value.LimitUSD, Currency: "USD"})
	}
	return items
}

func (h *InsightsHandler) usageDetailCutoff() time.Time {
	now := time.Now().In(h.loc).AddDate(0, 0, -h.usageRetentionDays)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, h.loc)
}

func (h *InsightsHandler) rangeFromQuery(c *gin.Context, defaultDays int) (time.Time, time.Time, bool) {
	today := time.Now().In(h.loc)
	toDate := today
	fromDate := today.AddDate(0, 0, -(defaultDays - 1))
	var err error
	if raw := c.Query("from"); raw != "" {
		fromDate, err = time.ParseInLocation("2006-01-02", raw, h.loc)
		if err != nil {
			response.BadRequest(c, "invalid from date")
			return time.Time{}, time.Time{}, false
		}
	}
	if raw := c.Query("to"); raw != "" {
		toDate, err = time.ParseInLocation("2006-01-02", raw, h.loc)
		if err != nil {
			response.BadRequest(c, "invalid to date")
			return time.Time{}, time.Time{}, false
		}
	}
	if toDate.After(today) {
		toDate = today
	}
	from := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, h.loc)
	to := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, h.loc).AddDate(0, 0, 1)
	if !from.Before(to) {
		response.BadRequest(c, "from must not be after to")
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func (h *InsightsHandler) PersonalUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "day")
	cutoff := h.usageDetailCutoff()
	var out insights.Envelope
	var err error
	if from.Before(cutoff) && to.After(cutoff) {
		out, err = h.query.PersonalUsageCombined(c.Request.Context(), subject.UserID, from, to, cutoff, granularity, c.QueryArray("model"))
	} else if from.Before(cutoff) {
		out, err = h.query.PersonalUsageDaily(c.Request.Context(), subject.UserID, from, to, granularity, c.QueryArray("model"))
	} else {
		out, err = h.query.PersonalUsage(c.Request.Context(), subject.UserID, from, to, granularity, c.QueryArray("model"))
	}
	if err != nil {
		if errors.Is(err, insights.ErrInvalidFilter) {
			response.BadRequest(c, "invalid usage filter")
			return
		}
		response.InternalError(c, "failed to query insights usage")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) Heatmap(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	year := time.Now().In(h.loc).Year()
	if raw := c.Query("year"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 2000 || parsed > time.Now().In(h.loc).Year() {
			response.BadRequest(c, "invalid year")
			return
		}
		year = parsed
	}
	out, err := h.query.Heatmap(c.Request.Context(), subject.UserID, year, h.usageDetailCutoff())
	if err != nil {
		response.InternalError(c, "failed to query insights heatmap")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) Departments(c *gin.Context) {
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "day")
	departments, models, performanceModels := c.QueryArray("department"), c.QueryArray("model"), c.QueryArray("performance_model")
	cutoff := h.usageDetailCutoff()
	var out insights.Envelope
	var err error
	if from.Before(cutoff) && to.After(cutoff) {
		out, err = h.query.DepartmentsCombined(c.Request.Context(), from, to, cutoff, granularity, departments, models, performanceModels)
	} else if from.Before(cutoff) {
		out, err = h.query.DepartmentsDaily(c.Request.Context(), from, to, granularity, departments, models, performanceModels)
	} else {
		out, err = h.query.Departments(c.Request.Context(), from, to, granularity, departments, models, performanceModels)
	}
	if err != nil {
		if errors.Is(err, insights.ErrInvalidFilter) {
			response.BadRequest(c, "invalid department filter")
			return
		}
		response.InternalError(c, "failed to query department insights")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) GatewayQuality(c *gin.Context) {
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	out, err := h.query.GatewayQualityLongTerm(c.Request.Context(), from, to, h.usageDetailCutoff(), c.DefaultQuery("granularity", "day"), c.QueryArray("department"))
	if err != nil {
		if errors.Is(err, insights.ErrOutsideRetention) {
			response.Error(c, http.StatusUnprocessableEntity, "DETAIL_OUTSIDE_RETENTION")
			return
		}
		if errors.Is(err, insights.ErrInvalidFilter) {
			response.BadRequest(c, "invalid gateway quality filter")
			return
		}
		response.InternalError(c, "failed to query gateway quality")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) UsageLogs(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	if !to.After(h.usageDetailCutoff()) {
		response.Error(c, http.StatusUnprocessableEntity, "DETAIL_OUTSIDE_RETENTION")
		return
	}
	pageSize := 20
	if raw := c.Query("page_size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.BadRequest(c, "invalid page_size")
			return
		}
		pageSize = parsed
	}
	out, err := h.query.UsageLogs(c.Request.Context(), subject.UserID, from, to, pageSize, c.Query("cursor"), c.QueryArray("model"))
	if err != nil {
		if errors.Is(err, insights.ErrInvalidFilter) {
			response.BadRequest(c, "invalid log query")
			return
		}
		response.InternalError(c, "failed to query usage logs")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) ErrorLogs(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	if !to.After(time.Now().In(h.loc).AddDate(0, 0, -h.errorRetentionDays)) {
		response.Error(c, http.StatusUnprocessableEntity, "DETAIL_OUTSIDE_RETENTION")
		return
	}
	pageSize := 20
	if raw := c.Query("page_size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.BadRequest(c, "invalid page_size")
			return
		}
		pageSize = parsed
	}
	out, err := h.query.ErrorLogs(c.Request.Context(), subject.UserID, from, to, pageSize, c.Query("cursor"), c.QueryArray("model"))
	if err != nil {
		if errors.Is(err, insights.ErrInvalidFilter) {
			response.BadRequest(c, "invalid error log query")
			return
		}
		response.InternalError(c, "failed to query error logs")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) GatewayModelPreferences(c *gin.Context) {
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	out, err := h.query.GatewayModelPreferencesLongTerm(c.Request.Context(), from, to, h.usageDetailCutoff(), c.QueryArray("department"))
	if err != nil {
		response.InternalError(c, "failed to query gateway model preferences")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) GatewayUsers(c *gin.Context) {
	from, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	out, err := h.query.GatewayUsersLongTerm(c.Request.Context(), from, to, h.usageDetailCutoff(), c.DefaultQuery("granularity", "day"), c.QueryArray("department"))
	if err != nil {
		if errors.Is(err, insights.ErrOutsideRetention) {
			response.Error(c, http.StatusUnprocessableEntity, "DETAIL_OUTSIDE_RETENTION")
			return
		}
		if errors.Is(err, insights.ErrInvalidFilter) {
			response.BadRequest(c, "invalid gateway users filter")
			return
		}
		response.InternalError(c, "failed to query gateway users")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *InsightsHandler) GatewayRetention(c *gin.Context) {
	_, to, ok := h.rangeFromQuery(c, 7)
	if !ok {
		return
	}
	out, err := h.query.GatewayRetentionFiltered(c.Request.Context(), to, c.QueryArray("department"))
	if err != nil {
		response.InternalError(c, "failed to query gateway retention")
		return
	}
	c.JSON(http.StatusOK, out)
}
