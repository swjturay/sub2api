package handler

import (
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/stretchr/testify/require"
)

func newTestGrokRealtimeTracker(now *time.Time, facts *[]insights.CallFact, gaps *[]string) *grokRealtimeInsightsTracker {
	userID, apiKeyID := int64(7), int64(9)
	return &grokRealtimeInsightsTracker{
		identity:  insights.Identity{Platform: "grok", Model: "grok-voice-latest", UserID: &userID, APIKeyID: &apiKeyID, Transport: insights.TransportWebSocketTurn},
		clock:     func() time.Time { return *now },
		emit:      func(f insights.CallFact) { *facts = append(*facts, f) },
		reportGap: func(reason string) { *gaps = append(*gaps, reason) },
	}
}

func TestGrokRealtimeInsightsTracksEachProtocolTurn(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	var facts []insights.CallFact
	var gaps []string
	tracker := newTestGrokRealtimeTracker(&now, &facts, &gaps)

	tracker.beforeUpstreamWrite([]byte(`{"type":"response.create"}`))
	now = now.Add(20 * time.Millisecond)
	tracker.afterUpstreamRead([]byte(`{"type":"response.created","response":{"id":"resp_1"}}`))
	now = now.Add(30 * time.Millisecond)
	tracker.afterUpstreamRead([]byte(`{"type":"response.output_audio.delta","response_id":"resp_1","delta":"abc"}`))
	now = now.Add(50 * time.Millisecond)
	tracker.afterUpstreamRead([]byte(`{"type":"response.done","response":{"id":"resp_1","status":"completed","usage":{"output_tokens":12}}}`))

	tracker.beforeUpstreamWrite([]byte(`{"type":"response.create"}`))
	now = now.Add(50 * time.Millisecond)
	tracker.disconnect(errors.New("socket closed"))

	require.Len(t, facts, 2)
	require.Empty(t, gaps)
	require.Equal(t, insights.OutcomeSuccess, facts[0].Outcome)
	require.Equal(t, int64(12), *facts[0].OutputTokens)
	require.Equal(t, 100*time.Millisecond, *facts[0].ModelDuration)
	require.Equal(t, 50*time.Millisecond, *facts[0].FirstToken)
	require.Equal(t, now.Add(-50*time.Millisecond), facts[0].StatisticalAt)
	require.Equal(t, insights.OutcomeStreamInterrupted, facts[1].Outcome)
	require.Equal(t, "realtime_disconnected", facts[1].ErrorType)
	require.Equal(t, 1, facts[1].AttemptCount)
}

func TestGrokRealtimeInsightsClassifiesCancelledTerminal(t *testing.T) {
	now := time.Date(2026, 9, 23, 11, 0, 0, 0, time.UTC)
	var facts []insights.CallFact
	var gaps []string
	tracker := newTestGrokRealtimeTracker(&now, &facts, &gaps)
	tracker.beforeUpstreamWrite([]byte(`{"type":"response.create"}`))
	tracker.afterUpstreamRead([]byte(`{"type":"response.created","response":{"id":"resp_cancel"}}`))
	now = now.Add(time.Second)
	tracker.afterUpstreamRead([]byte(`{"type":"response.done","response":{"id":"resp_cancel","status":"cancelled"}}`))

	require.Len(t, facts, 1)
	require.Equal(t, insights.OutcomeCancelled, facts[0].Outcome)
	require.Nil(t, facts[0].ModelDuration)
}

func TestGrokRealtimeInsightsServerVADKeepsFactAndReportsTimingGap(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	var facts []insights.CallFact
	var gaps []string
	tracker := newTestGrokRealtimeTracker(&now, &facts, &gaps)
	tracker.afterUpstreamRead([]byte(`{"type":"response.created","response":{"id":"resp_vad"}}`))
	now = now.Add(2 * time.Second)
	tracker.afterUpstreamRead([]byte(`{"type":"response.done","response":{"id":"resp_vad","status":"completed"}}`))

	require.Len(t, facts, 1)
	require.Equal(t, insights.OutcomeSuccess, facts[0].Outcome)
	require.Equal(t, 2*time.Second, *facts[0].ModelDuration)
	require.Equal(t, []string{"grok realtime server-VAD turn send time is not observable"}, gaps)
}
