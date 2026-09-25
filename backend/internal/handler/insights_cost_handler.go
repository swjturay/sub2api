package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

var costDataStartMonth = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

func (h *InsightsHandler) CostData(c *gin.Context) {
	month, ok := h.parseCostMonth(c.Query("month"))
	if !ok {
		response.BadRequest(c, "invalid cost month")
		return
	}
	page, pageSize := 1, 20
	if value, err := strconv.Atoi(c.Query("page")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(c.Query("page_size")); err == nil && (value == 20 || value == 50 || value == 100) {
		pageSize = value
	}
	out, err := h.query.CostDashboard(c.Request.Context(), insights.CostFilter{
		Month: month, Department: c.Query("department"), Platform: c.Query("platform"), Search: c.Query("q"),
		Contributor: c.Query("contributor"), Payment: c.Query("payment_method"), Status: c.Query("status"),
		Completeness: c.Query("completeness"), Registration: c.Query("registration"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		response.InternalError(c, "failed to query cost data")
		return
	}
	c.JSON(http.StatusOK, out)
}

type saveCostMonthRequest struct {
	ContributorUserID int64   `json:"contributor_user_id"`
	PaymentMethod     string  `json:"payment_method"`
	ActualCost        *string `json:"actual_cost"`
	Notes             string  `json:"notes"`
}

func (h *InsightsHandler) SaveCostMonth(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	month, valid := h.parseCostMonth(c.Param("month"))
	if !valid {
		response.BadRequest(c, "invalid cost month")
		return
	}
	var request saveCostMonthRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid cost month payload")
		return
	}
	if request.ContributorUserID <= 0 {
		response.BadRequest(c, "contributor_user_id is required")
		return
	}
	if !insights.ValidateCostPaymentMethod(request.PaymentMethod) {
		response.BadRequest(c, "invalid payment method")
		return
	}
	if utf8.RuneCountInString(request.Notes) > 1000 {
		response.BadRequest(c, "notes must not exceed 1000 characters")
		return
	}
	var actualCost *string
	if request.ActualCost != nil && strings.TrimSpace(*request.ActualCost) != "" {
		normalized, err := insights.NormalizeCostAmount(*request.ActualCost)
		if err != nil {
			response.BadRequest(c, "actual cost must be a non-negative USD amount with at most two decimals")
			return
		}
		actualCost = &normalized
	}
	err = h.query.SaveCostMonth(c.Request.Context(), insights.SaveCostMonthInput{
		AccountID: accountID, Month: month, ContributorID: request.ContributorUserID,
		PaymentMethod: request.PaymentMethod, ActualCost: actualCost, Notes: strings.TrimSpace(request.Notes), UpdatedBy: subject.UserID,
	})
	if h.writeCostMutationError(c, err) {
		return
	}
	response.Success(c, map[string]any{"saved": true})
}

type stopCostAccountRequest struct {
	AfterMonth string `json:"after_month"`
}

func (h *InsightsHandler) StopCostAccount(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	var request stopCostAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid stop payload")
		return
	}
	month, valid := h.parseCostMonth(request.AfterMonth)
	if !valid {
		response.BadRequest(c, "invalid cost month")
		return
	}
	err = h.query.StopCostAccountAfter(c.Request.Context(), accountID, month, subject.UserID)
	if h.writeCostMutationError(c, err) {
		return
	}
	response.Success(c, map[string]any{"stopped_after": month.Format("2006-01")})
}

func (h *InsightsHandler) parseCostMonth(raw string) (time.Time, bool) {
	now := time.Now().In(h.loc)
	if strings.TrimSpace(raw) == "" {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, h.loc), true
	}
	month, err := time.ParseInLocation("2006-01", raw, h.loc)
	if err != nil {
		return time.Time{}, false
	}
	minimum := time.Date(costDataStartMonth.Year(), costDataStartMonth.Month(), 1, 0, 0, 0, 0, h.loc)
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, h.loc)
	return month, !month.Before(minimum) && !month.After(current)
}

func (h *InsightsHandler) writeCostMutationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, insights.ErrCostAccountNotFound), errors.Is(err, insights.ErrCostContributorNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, insights.ErrCostStopConflict):
		response.Error(c, http.StatusConflict, "next month already has actual cost data")
	case errors.Is(err, insights.ErrCostShadowAccount), errors.Is(err, insights.ErrCostDeletedAccount), errors.Is(err, insights.ErrInvalidFilter):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "failed to update cost data")
	}
	return true
}
