package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const statusClientClosedRequest = 499

const (
	gatewayQueueFullCode        = "gateway_queue_full"
	gatewayConcurrencyLimitCode = "gateway_concurrency_limit"
)

func concurrencyErrorResponse(err error, slotType string) (int, string, string, string) {
	var waitQueueFullErr *WaitQueueFullError
	if errors.As(err, &waitQueueFullErr) {
		return http.StatusTooManyRequests, "rate_limit_error", gatewayQueueFullCode,
			"Too many pending requests, please retry later"
	}

	var concurrencyErr *ConcurrencyError
	if errors.As(err, &concurrencyErr) {
		if concurrencyErr.SlotType != "" {
			slotType = concurrencyErr.SlotType
		}
		return http.StatusTooManyRequests, "rate_limit_error", gatewayConcurrencyLimitCode,
			fmt.Sprintf("Concurrency limit exceeded for %s, please retry later", slotType)
	}

	if errors.Is(err, context.Canceled) {
		return statusClientClosedRequest, "api_error", "", "context canceled"
	}

	return http.StatusServiceUnavailable, "api_error", "", "Service temporarily unavailable, please retry later"
}

func concurrencyErrorRetryAfterSeconds(err error) int {
	var waitQueueFullErr *WaitQueueFullError
	if errors.As(err, &waitQueueFullErr) {
		return defaultConcurrencyRetryAfterSeconds
	}

	var concurrencyErr *ConcurrencyError
	if errors.As(err, &concurrencyErr) {
		return defaultConcurrencyRetryAfterSeconds
	}

	return 0
}

func writeConcurrencyRetryAfter(c *gin.Context, err error, streamStarted bool) {
	if c == nil || c.Writer == nil || streamStarted || c.Writer.Written() || service.IsResponseCommitted(c) {
		return
	}
	if retryAfter := concurrencyErrorRetryAfterSeconds(err); retryAfter > 0 {
		c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
	}
}
