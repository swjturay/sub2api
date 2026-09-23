package handler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/tidwall/gjson"
)

type grokRealtimeInsightsTurn struct {
	call       *insights.Call
	responseID string
}

type grokRealtimeInsightsTracker struct {
	mu              sync.Mutex
	identity        insights.Identity
	clock           insights.Clock
	turns           []*grokRealtimeInsightsTurn
	emit            func(insights.CallFact)
	reportGap       func(string)
	automaticTiming bool
}

func newGrokRealtimeInsightsTracker(ctx context.Context, userID, apiKeyID int64, model string) *grokRealtimeInsightsTracker {
	clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string)
	return &grokRealtimeInsightsTracker{
		identity: insights.Identity{
			ClientRequestID: strings.TrimSpace(clientRequestID),
			Platform:        service.PlatformGrok,
			Model:           strings.TrimSpace(model),
			UserID:          &userID,
			APIKeyID:        &apiKeyID,
			Transport:       insights.TransportWebSocketTurn,
		},
		clock:     time.Now,
		emit:      func(f insights.CallFact) { insights.RecordBestEffort(f) },
		reportGap: insights.ReportGapBestEffort,
	}
}

func (t *grokRealtimeInsightsTracker) beforeUpstreamWrite(message []byte) {
	if t == nil || strings.TrimSpace(gjson.GetBytes(message, "type").String()) != "response.create" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.clock()
	call := insights.NewCall(t.identity, now, t.clock)
	call.MarkUpstreamSend(now)
	t.turns = append(t.turns, &grokRealtimeInsightsTurn{call: call})
}

func (t *grokRealtimeInsightsTracker) afterUpstreamRead(message []byte) {
	if t == nil {
		return
	}
	switch strings.TrimSpace(gjson.GetBytes(message, "type").String()) {
	case "response.created":
		t.bindResponse(strings.TrimSpace(gjson.GetBytes(message, "response.id").String()))
	case "response.output_audio.delta", "response.audio.delta", "response.output_text.delta", "response.text.delta", "response.audio_transcript.delta":
		t.markFirstOutput(strings.TrimSpace(gjson.GetBytes(message, "response_id").String()))
	case "response.done":
		t.finishResponse(message)
	case "error":
		t.finishError(message)
	}
}

func (t *grokRealtimeInsightsTracker) bindResponse(responseID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, turn := range t.turns {
		if turn.responseID == "" {
			turn.responseID = responseID
			return
		}
	}
	// Server VAD may create a response without a client response.create. The
	// gateway observes it only after upstream starts, so timing stays partial.
	now := t.clock()
	call := insights.NewCall(t.identity, now, t.clock)
	call.MarkUpstreamSend(now)
	t.turns = append(t.turns, &grokRealtimeInsightsTurn{call: call, responseID: responseID})
	if !t.automaticTiming {
		t.automaticTiming = true
		t.reportGap("grok realtime server-VAD turn send time is not observable")
	}
}

func (t *grokRealtimeInsightsTracker) markFirstOutput(responseID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, turn := range t.turns {
		if responseID != "" && turn.responseID != responseID {
			continue
		}
		turn.call.MarkFirstToken(t.clock())
		return
	}
}

func (t *grokRealtimeInsightsTracker) finishResponse(message []byte) {
	turn := t.take(strings.TrimSpace(gjson.GetBytes(message, "response.id").String()))
	if turn == nil {
		t.reportGap("grok realtime response.done has no observed turn start")
		return
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(message, "response.status").String()))
	if status == "completed" {
		output := gjson.GetBytes(message, "response.usage.output_tokens").Int()
		t.emit(turn.call.FinishSuccess(&output))
		return
	}
	errorType := strings.TrimSpace(gjson.GetBytes(message, "response.status_details.error.type").String())
	if errorType == "" {
		errorType = "realtime_" + firstNonEmpty(status, "failed")
	}
	outcome := insights.OutcomeError
	if status == "cancelled" {
		outcome = insights.OutcomeCancelled
	}
	t.emit(turn.call.FinishFailure(outcome, errorType, ""))
}

func (t *grokRealtimeInsightsTracker) finishError(message []byte) {
	turn := t.take(strings.TrimSpace(gjson.GetBytes(message, "error.response_id").String()))
	if turn == nil {
		return
	}
	errorType := strings.TrimSpace(gjson.GetBytes(message, "error.type").String())
	if errorType == "" {
		errorType = "realtime_error"
	}
	t.emit(turn.call.FinishFailure(insights.OutcomeError, errorType, ""))
}

func (t *grokRealtimeInsightsTracker) take(responseID string) *grokRealtimeInsightsTurn {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, turn := range t.turns {
		if responseID != "" && turn.responseID != responseID {
			continue
		}
		t.turns = append(t.turns[:i], t.turns[i+1:]...)
		return turn
	}
	return nil
}

func (t *grokRealtimeInsightsTracker) disconnect(err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	turns := t.turns
	t.turns = nil
	t.mu.Unlock()
	outcome, errorType := insights.OutcomeStreamInterrupted, "realtime_disconnected"
	if errors.Is(err, context.Canceled) {
		outcome, errorType = insights.OutcomeCancelled, "client_cancelled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		outcome, errorType = insights.OutcomeTimeout, "timeout"
	}
	for _, turn := range turns {
		t.emit(turn.call.FinishFailure(outcome, errorType, ""))
	}
}
