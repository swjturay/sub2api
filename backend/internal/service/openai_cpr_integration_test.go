//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCPRAllHTTPProtocolsCancelUpstream(t *testing.T) {
	for _, protocol := range []string{"responses", "messages", "chat/completions"} {
		t.Run(protocol, func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			svc, account := rawRelayTestSetup(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"pending\"}}\n\n")
				w.(http.Flusher).Flush()
				close(started)
				<-r.Context().Done()
				close(stopped)
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body := []byte(`{"model":"gpt-5.4","stream":true,"input":"hi","messages":[{"role":"user","content":"hi"}],"max_tokens":64}`)
			c, _, _ := rawRelayTestContext(t, ctx, "/v1/"+protocol, body, "", nil)
			done := make(chan struct{})
			var result *OpenAIForwardResult
			var err error
			go func() {
				defer close(done)
				switch protocol {
				case "responses":
					result, err = svc.Forward(ctx, c, account, body)
				case "messages":
					result, err = svc.ForwardAsAnthropic(ctx, c, account, body, "", "")
				default:
					result, err = svc.ForwardAsChatCompletions(ctx, c, account, body, "", "")
				}
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("upstream not started")
			}
			cancel()
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("CPR kept executing after cancel")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("forwarding did not finish")
			}
			require.Error(t, err)
			require.NotNil(t, result)
			require.True(t, result.ClientDisconnect)
			require.Equal(t, "unknown", result.CodexObservation.Usage.Status)
		})
	}
}

// Regressions for CPR compatibility conversion and failure accounting.
func TestCPRIntegrationNonResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"messages", "chat/completions"} {
		t.Run(protocol, func(t *testing.T) {
			messages := make([]string, 0, 15)
			for i := 0; i < 15; i++ {
				messages = append(messages, fmt.Sprintf(`{"role":"user","content":"message-%02d"}`, i))
			}
			body := []byte(`{"model":"gpt-5.4","max_tokens":128,"messages":[` + strings.Join(messages, ",") + `],"stream":false}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			upstream := &httpUpstreamRecorder{resp: openAICompatSSECompletedResponse("resp_research", "gpt-5.4")}
			svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: cprTestConfig()}
			account := newCPRTestAccount()
			var result *OpenAIForwardResult
			var err error
			if protocol == "messages" {
				result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
			} else {
				result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
			require.Equal(t, int64(15), gjson.GetBytes(upstream.lastBody, "input.#").Int())
			require.Contains(t, string(upstream.lastBody), "message-00")
			require.NotContains(t, string(upstream.lastBody), openAICompatClaudeCodeTodoGuardMarker)
			t.Logf("protocol=%s model=%s input_items=%d first_message_preserved=%t todo_guard=%t upstream_stream=%t downstream_status=%d downstream_type=%s downstream_object=%s usage=%+v", protocol, gjson.GetBytes(upstream.lastBody, "model").String(), gjson.GetBytes(upstream.lastBody, "input.#").Int(), bytes.Contains(upstream.lastBody, []byte("message-00")), strings.Contains(gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String(), openAICompatClaudeCodeTodoGuardMarker), gjson.GetBytes(upstream.lastBody, "stream").Bool(), rec.Code, gjson.Get(rec.Body.String(), "type").String(), gjson.Get(rec.Body.String(), "object").String(), result.Usage)
		})
	}
}

func TestCPRIntegrationFailedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"messages", "chat/completions"} {
		t.Run(protocol, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.4","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, bytes.NewReader(body))
			wire := "data: " + `{"type":"response.failed","response":{"id":"resp_failed_research","model":"gpt-5.4","status":"failed","output":[],"error":{"type":"invalid_request_error","code":"invalid_request","message":"synthetic rejection"},"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}` + "\n\n"
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(wire))}}
			svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: cprTestConfig()}
			account := newCPRTestAccount()
			var result *OpenAIForwardResult
			var err error
			if protocol == "messages" {
				result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
			} else {
				result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			}
			require.Error(t, err)
			require.NotNil(t, result)
			require.Equal(t, 5, result.Usage.InputTokens)
			require.Equal(t, 2, result.Usage.OutputTokens)
			require.Equal(t, "complete", result.CodexObservation.Usage.Status)
			require.Equal(t, "response.failed", result.CodexObservation.Usage.TerminalEvent)
			t.Logf("protocol=%s known_failed_usage=5/2 result_nil=%t downstream_status=%d", protocol, result == nil, rec.Code)
		})
	}
}
