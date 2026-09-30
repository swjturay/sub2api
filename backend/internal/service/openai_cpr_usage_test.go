//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestCPRUsageEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, usage, status string
		input               int
	}{
		{"absent", "null", "unknown", 0}, {"empty", "{}", "unknown", 0},
		{"invalid", `{"input_tokens":"9","output_tokens":null}`, "unknown", 0},
		{"partial", `{"input_tokens":9}`, "partial", 9},
		{"mixed", `{"input_tokens":9,"output_tokens":"7"}`, "partial", 9},
		{"zero", `{"input_tokens":0,"output_tokens":0}`, "complete", 0},
		{"final", `{"input_tokens":9,"output_tokens":2}`, "complete", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &cprUsageCapture{streaming: true}
			capture.observe([]byte(`{"type":"response.completed","response":{"usage":`+tc.usage+`}}`), "")
			usage, observation := capture.snapshot()
			require.Equal(t, tc.status, observation.Status)
			require.Equal(t, tc.input, usage.InputTokens)
			if tc.name == "mixed" {
				require.Zero(t, usage.OutputTokens)
			}
		})
	}
	// Explicit final zero supersedes a progressive observation.
	capture := &cprUsageCapture{streaming: true}
	capture.observe([]byte(`{"type":"response.created","response":{"usage":{"input_tokens":9}}}`), "")
	capture.observe([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":0,"output_tokens":0}}}`), "")
	usage, observation := capture.snapshot()
	require.Zero(t, usage.InputTokens)
	require.Equal(t, "complete", observation.Status)
}

func TestCPRReceivedUsageSurvivesClientWriteFailure(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(rawRelaySSE))}}
	svc := &OpenAIGatewayService{cfg: cprTestConfig(), httpUpstream: upstream}
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":"hi"}`)
	c, _, _ := rawRelayTestContext(t, context.Background(), "/v1/responses", body, "", nil)
	c.Writer = &openAIChatFailingWriter{ResponseWriter: c.Writer}
	result, err := svc.Forward(c.Request.Context(), c, newCPRTestAccount(), body)
	require.Error(t, err)
	require.True(t, result.ClientDisconnect)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, "complete", result.CodexObservation.Usage.Status)
}

func TestCPRStreamEOFIsUnknownFailure(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.created\",\"response\":{\"id\":\"unfinished\"}}\n\n"))}}
	svc := &OpenAIGatewayService{cfg: cprTestConfig(), httpUpstream: upstream}
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":"hi"}`)
	c, _, _ := rawRelayTestContext(t, context.Background(), "/v1/responses", body, "", nil)
	result, err := svc.Forward(c.Request.Context(), c, newCPRTestAccount(), body)
	require.ErrorContains(t, err, "before a terminal event")
	require.Equal(t, "unknown", result.CodexObservation.Usage.Status)
}

func TestCPRObservationBoundsWholeSSEFrame(t *testing.T) {
	capture := &cprUsageCapture{streaming: true}
	oversized := strings.Repeat("data: "+strings.Repeat("x", 1024)+"\n", 17000) + "\n"
	valid := "event: response.completed\ndata: {\"response\":{\n" + "data: \"usage\":{\"input_tokens\":3,\"output_tokens\":2}}}\n\n"
	body := &cprObservedBody{ReadCloser: io.NopCloser(strings.NewReader(oversized + valid)), capture: capture, streaming: true}
	n, err := io.Copy(io.Discard, body)
	require.NoError(t, err)
	require.Equal(t, int64(len(oversized+valid)), n)
	usage, observation := capture.snapshot()
	require.Equal(t, 3, usage.InputTokens)
	require.Equal(t, "complete", observation.Status)
}

func TestCPRHTTPAttemptsUseSeparateBillingKeys(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	billing := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "same-client-call")
	var keys []string
	for _, tokens := range []int{0, 7} {
		c, _, body := rawRelayTestContext(t, ctx, "/v1/responses", []byte(`{"model":"gpt-5.4","input":"hi"}`), "", nil)
		// Drive the public transport seam, including its generated per-attempt key.
		upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"upstream","usage":{"input_tokens":` + strconv.Itoa(tokens) + `,"output_tokens":0}}`))}}
		forward := &OpenAIGatewayService{cfg: cprTestConfig(), httpUpstream: upstream}
		if tokens == 0 {
			upstream.resp.StatusCode = http.StatusTooManyRequests
		}
		result, err := forward.Forward(ctx, c, newCPRTestAccount(), body)
		if tokens == 0 {
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
		} else {
			require.NoError(t, err)
		}
		require.NotEmpty(t, result.BillingRequestID)
		require.NoError(t, svc.RecordUsage(ctx, &OpenAIRecordUsageInput{Result: result, StatisticalAt: time.Now(), APIKey: &APIKey{ID: 1, Quota: 100, Group: &Group{RateMultiplier: 1}}, User: &User{ID: 2}, Account: newCPRTestAccount(), APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}}))
		keys = append(keys, repo.lastLog.RequestID)
		require.Equal(t, result.BillingRequestID, billing.lastCmd.RequestID)
		require.Equal(t, result.CodexObservation, repo.lastLog.CodexObservation)
	}
	require.NotEqual(t, keys[0], keys[1])
}
