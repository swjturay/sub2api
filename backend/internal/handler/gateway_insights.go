package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// InsightsCallMiddleware wraps inference routes only. Control endpoints, model
// discovery, token counting, usage reads and the WS connection itself are excluded.
func InsightsCallMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		transport, ok := insightsHTTPTransport(c.Request.Method, c.Request.URL.Path)
		if !ok && insightsKnownUninstrumentedGeneration(c.Request.Method, c.Request.URL.Path) {
			insights.ReportGapBestEffort("enabled generation route lacks terminal hook: " + c.Request.URL.Path)
		}
		if !ok {
			c.Next()
			return
		}
		startedAt := time.Now()
		clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		requestID := service.ResolveUsageFactRequestID(c.Request.Context(), "")
		call := insights.NewCall(insights.Identity{RequestID: requestID, ClientRequestID: strings.TrimSpace(clientRequestID), Transport: transport}, startedAt, nil)
		c.Request = c.Request.WithContext(insights.WithCall(c.Request.Context(), call))
		defer func() {
			insightsPopulateAuthIdentity(c, call)
			if recovered := recover(); recovered != nil {
				if _, done := call.Finalized(); !done {
					fact := call.FinishFailure(insights.OutcomeError, "handler_panic", "")
					insights.RecordBestEffort(fact)
				}
				panic(recovered)
			}
			if _, done := call.Finalized(); done {
				return
			}
			outcome, typ := insightsHTTPFailure(c, call)
			if typ == "terminal_unobserved" {
				insights.ReportGapBestEffort("protocol terminal was not observed")
			}
			fact := call.FinishFailure(outcome, typ, "")
			insights.RecordBestEffort(fact)
		}()
		c.Next()
	}
}

func insightsKnownUninstrumentedGeneration(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	return strings.HasSuffix(path, "/live") || strings.HasSuffix(path, "/realtime/calls")
}

func insightsHTTPTransport(method, path string) (insights.Transport, bool) {
	if method != http.MethodPost || strings.HasSuffix(path, "/count_tokens") || strings.HasSuffix(path, "/input_tokens") {
		return insights.TransportUnknown, false
	}
	generationSuffixes := []string{"/messages", "/responses", "/responses/compact", "/chat/completions", "/embeddings", "/alpha/search", "/web_search", "/x_search", "/images/generations", "/images/edits", "/videos", "/videos/generations", "/videos/edits", "/videos/extensions", "/tts", "/stt"}
	for _, suffix := range generationSuffixes {
		if strings.HasSuffix(path, suffix) {
			return insights.TransportHTTPSync, true
		}
	}
	if strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent") {
		return insights.TransportHTTPSync, true
	}
	return insights.TransportUnknown, false
}

func insightsPopulateAuthIdentity(c *gin.Context, call *insights.Call) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		return
	}
	update := insights.Identity{APIKeyID: &apiKey.ID}
	if apiKey.User != nil {
		update.UserID = &apiKey.User.ID
	}
	if apiKey.Group != nil {
		update.Platform = apiKey.Group.Platform
	}
	call.UpdateIdentity(update)
}

func insightsUpdateCall(c *gin.Context, model, platform string, stream bool) {
	call, ok := insights.CallFromContext(c.Request.Context())
	if !ok {
		return
	}
	transport := insights.TransportHTTPSync
	if stream {
		transport = insights.TransportHTTPStream
	}
	call.UpdateIdentity(insights.Identity{Model: strings.TrimSpace(model), Platform: strings.TrimSpace(platform), Transport: transport})
	insightsPopulateAuthIdentity(c, call)
}

func insightsFinishSuccess(c *gin.Context, model string, outputTokens int, firstTokenMS *int) time.Time {
	call, ok := insights.CallFromContext(c.Request.Context())
	if !ok {
		return time.Time{}
	}
	if firstTokenMS != nil && *firstTokenMS >= 0 {
		call.SetFirstTokenDuration(time.Duration(*firstTokenMS) * time.Millisecond)
	}
	output := int64(max(outputTokens, 0))
	fact := call.FinishSuccess(&output)
	insights.RecordBestEffort(fact)
	return fact.StatisticalAt
}

func insightsFinishFailure(c *gin.Context, outcome insights.Outcome, errorType string) time.Time {
	call, ok := insights.CallFromContext(c.Request.Context())
	if !ok {
		return time.Time{}
	}
	fact := call.FinishFailure(outcome, errorType, "")
	insights.RecordBestEffort(fact)
	return fact.StatisticalAt
}

func insightsHTTPFailure(c *gin.Context, call *insights.Call) (insights.Outcome, string) {
	err := c.Request.Context().Err()
	if errors.Is(err, context.Canceled) {
		return insights.OutcomeCancelled, "client_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return insights.OutcomeTimeout, "timeout"
	}
	if call.AttemptCount() == 0 {
		return insights.OutcomeRejected, "request_rejected"
	}
	if c.Writer.Size() > 0 && c.Writer.Status() < 400 {
		return insights.OutcomeStreamInterrupted, "stream_interrupted"
	}
	if c.Writer.Status() >= 400 {
		return insights.OutcomeError, "upstream_error"
	}
	return insights.OutcomeError, "terminal_unobserved"
}

func insightsFinishForwardSuccess(c *gin.Context, result *service.ForwardResult) time.Time {
	if result == nil {
		insights.ReportGapBestEffort("nil forward result without terminal")
		return time.Time{}
	}
	if call, ok := insights.CallFromContext(c.Request.Context()); ok {
		call.UpdateIdentity(insights.Identity{RequestID: service.ResolveUsageFactRequestID(c.Request.Context(), result.RequestID)})
	}
	return insightsFinishSuccess(c, result.Model, result.Usage.OutputTokens, result.FirstTokenMs)
}

func insightsFinishOpenAIForwardSuccess(c *gin.Context, result *service.OpenAIForwardResult) time.Time {
	if result == nil {
		insights.ReportGapBestEffort("nil openai forward result without terminal")
		return time.Time{}
	}
	if call, ok := insights.CallFromContext(c.Request.Context()); ok {
		call.UpdateIdentity(insights.Identity{RequestID: service.ResolveUsageFactRequestID(c.Request.Context(), result.RequestID)})
	}
	return insightsFinishSuccess(c, result.Model, result.Usage.OutputTokens, result.FirstTokenMs)
}
