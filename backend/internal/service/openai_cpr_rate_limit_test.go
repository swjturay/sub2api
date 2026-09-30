//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type cprCooldownRepo struct {
	AccountRepository
	mu     sync.Mutex
	until  time.Time
	writes int
}

func (r *cprCooldownRepo) SetRateLimitedIfLater(_ context.Context, _ int64, until time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if until.After(r.until) {
		r.until = until
	}
	r.writes++
	return nil
}
func (r *cprCooldownRepo) state() (time.Time, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.until, r.writes
}

func cprCooldownFixture(t *testing.T, detail string) (*Account, *RateLimitService, *cprCooldownRepo, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, detail)
	}))
	t.Cleanup(server.Close)
	account := newCPRTestAccount()
	account.Credentials["admin_base_url"] = server.URL
	repo := &cprCooldownRepo{}
	svc := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	return account, svc, repo, &calls
}

func cprExhaustedDetail(until time.Time) string {
	return cprTestDetailJSON("quota_exhausted", "plus", cprTestWindow("primary", 18000, 100, until.In(time.FixedZone("CST", 8*3600)).Format(cprDisplayTimeLayout)))
}

func TestCPR429DoesNotUseDirectOAuthRetryWindow(t *testing.T) {
	account := newCPRTestAccount()
	svc := &OpenAIGatewayService{}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","code":"rate_limit_exceeded"}}`)
	failure := svc.newOpenAIAccountFailoverError(account, http.StatusTooManyRequests, nil, body, "", false, true)
	require.True(t, failure.SameAccountRetryDeadline.IsZero(), "CPR retries must remain bounded by the configured count")
	require.False(t, svc.ShouldRetryOpenAIOAuth429(account, nil, body), "CPR must not postpone the existing cooldown")
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, 429, 3, nil), "CPR switch count must remain the configured count")
}

func TestCPRQuotaCooldownPersistsAcrossConcurrentGeneric429(t *testing.T) {
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	account, limits, repo, calls := cprCooldownFixture(t, cprExhaustedDetail(want))
	gw := &OpenAIGatewayService{rateLimitService: limits}
	limits.SetAccountRuntimeBlocker(gw)
	limits.handleCPRQuotaFailure(context.Background(), account)
	// The stale request has no RateLimitResetAt; only the atomic repository
	// extension can prevent its generic 429 from overwriting the longer reset.
	limits.handle429(context.Background(), account, nil, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	got, _ := repo.state()
	require.WithinDuration(t, want, got, time.Second)
	require.EqualValues(t, 1, calls.Load())
	require.True(t, gw.isOpenAIAccountRuntimeBlocked(account))
}

func TestCPRQuotaStateScopeAndFallback(t *testing.T) {
	future := time.Now().Add(time.Hour).Truncate(time.Second)
	for _, tc := range []struct {
		name, detail string
		long         bool
	}{
		{"exhausted", cprExhaustedDetail(future), true},
		{"normal_display_reached", strings.Replace(cprExhaustedDetail(future), `"status":"quota_exhausted"`, `"status":"normal"`, 1), false},
		{"spark_only", strings.ReplaceAll(cprExhaustedDetail(future), `"limitId":"codex"`, `"limitId":"codex_bengalfox"`), false},
		{"expired", cprExhaustedDetail(time.Now().Add(-time.Hour)), false},
		{"missing", cprTestDetailJSON("quota_exhausted", "plus", ""), false},
		{"wrong_binding", strings.ReplaceAll(cprExhaustedDetail(future), cprTestCPRAccount, "another-account"), false},
		{"admin_rejected", `{"code":40103,"message":"secret detail"}`, false},
		{"rate_limited", strings.Replace(cprTestDetailJSON("rate_limited", "plus", ""), `"rateLimitedUntil":null`, fmt.Sprintf(`"rateLimitedUntil":%q,"recoveryProbeRequired":true`, future.In(time.FixedZone("CST", 8*3600)).Format(cprDisplayTimeLayout)), 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account, limits, repo, _ := cprCooldownFixture(t, tc.detail)
			before := time.Now()
			limits.handleCPRQuotaFailure(context.Background(), account)
			got, calls := repo.state()
			require.Equal(t, 1, calls)
			if tc.long {
				require.WithinDuration(t, future, got, time.Second)
			} else {
				require.WithinDuration(t, before.Add(defaultRateLimit429CooldownSeconds*time.Second), got, time.Second)
			}
		})
	}
}

func TestCPRQuotaErrorDoesNotConsumeCapacityOrKeyErrors(t *testing.T) {
	for _, code := range []string{"account_capacity_unavailable", "key_daily_budget_exceeded", "key_weekly_budget_exceeded", "concurrency_queue_timeout", "concurrency_queue_full", "provider_infrastructure_unavailable", "runtime_configuration_unavailable", "key_budget_unavailable", "rate_limit_exceeded", "server_error", "model_not_found", "image_generation_limit_reached"} {
		account, limits, repo, calls := cprCooldownFixture(t, cprExhaustedDetail(time.Now().Add(time.Hour)))
		capture := &cprUsageCapture{}
		capture.observe([]byte(fmt.Sprintf(`{"type":"error","error":{"type":"usage_limit_reached","code":%q,"message":"quota_exhausted"}}`, code)), "")
		gw := &OpenAIGatewayService{rateLimitService: limits}
		require.False(t, gw.applyCapturedCPRQuota(context.Background(), account, capture), code)
		_, writes := repo.state()
		require.Zero(t, writes, code)
		require.Zero(t, calls.Load(), code)
	}
}

func TestCPRQuotaConcurrentQueriesCoalesceAndBindingChangesInvalidate(t *testing.T) {
	account, limits, _, calls := cprCooldownFixture(t, cprExhaustedDetail(time.Now().Add(time.Hour)))
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); limits.handleCPRQuotaFailure(context.Background(), account) }()
	}
	wg.Wait()
	require.EqualValues(t, 1, calls.Load())
	account.Credentials["admin_api_key"] = "rotated-key"
	limits.handleCPRQuotaFailure(context.Background(), account)
	require.EqualValues(t, 2, calls.Load())
}

func TestCPRQuotaFailureCapturedForAllHTTPProtocols(t *testing.T) {
	for _, protocol := range []string{"responses", "chat/completions", "messages"} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", protocol, streaming), func(t *testing.T) {
				want := time.Now().Add(time.Hour).Truncate(time.Second)
				adminAccount, limits, repo, calls := cprCooldownFixture(t, cprExhaustedDetail(want))
				failure := `{"error":{"type":"usage_limit_reached","code":"usage_limit_reached","message":"exhausted"}}`
				sse := "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failure\",\"error\":{\"code\":\"usage_limit_reached\"},\"usage\":{\"input_tokens\":11,\"output_tokens\":3}}}\n\n"
				svc, account := rawRelayTestSetup(t, func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, sse)
					} else {
						w.WriteHeader(429)
						_, _ = io.WriteString(w, failure)
					}
				})
				account.Credentials["admin_base_url"] = adminAccount.GetCPRAdminBaseURL()
				svc.rateLimitService = limits
				limits.SetAccountRuntimeBlocker(svc)
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.4","max_tokens":10,"input":"hi","messages":[{"role":"user","content":"hi"}],"stream":%t}`, streaming))
				c, rec, _ := rawRelayTestContext(t, context.Background(), "/v1/"+protocol, body, "", nil)
				var result *OpenAIForwardResult
				var err error
				switch protocol {
				case "responses":
					result, err = svc.Forward(context.Background(), c, account, body)
				case "messages":
					result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
				default:
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
				}
				require.Error(t, err)
				if !streaming {
					var f *UpstreamFailoverError
					require.ErrorAs(t, err, &f)
					require.False(t, f.RetryableOnSameAccount)
					require.True(t, f.SameAccountRetryDeadline.IsZero())
				}
				got, writes := repo.state()
				require.WithinDuration(t, want, got, time.Second)
				require.Equal(t, 1, writes)
				require.EqualValues(t, 1, calls.Load())
				if streaming {
					require.NotNil(t, result)
					require.Equal(t, 11, result.Usage.InputTokens)
					require.Equal(t, 3, result.Usage.OutputTokens)
					if protocol == "responses" {
						require.Equal(t, sse, rec.Body.String())
					}
				}
			})
		}
	}
}

func TestCPRQuotaResponseResetSurvivesUnavailableAdmin(t *testing.T) {
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	for _, nested := range []bool{false, true} {
		account, limits, repo, calls := cprCooldownFixture(t, `{"code":40103}`)
		body := fmt.Sprintf(`{"error":{"code":"usage_limit_reached","resets_at":%d}}`, want.Unix())
		if nested {
			body = `{"type":"response.failed","response":` + body + `}`
		}
		capture := &cprUsageCapture{streaming: nested}
		capture.observe([]byte(body), "")
		gw := &OpenAIGatewayService{rateLimitService: limits}
		require.True(t, gw.applyCapturedCPRQuota(context.Background(), account, capture))
		require.True(t, gw.applyCapturedCPRQuota(context.Background(), account, capture))
		got, writes := repo.state()
		require.Equal(t, want, got)
		require.Equal(t, 1, writes, "duplicate terminal processing must not settle twice")
		require.Zero(t, calls.Load(), "a response reset needs no management query")
	}
}

func TestCPRQuotaNonCapturedEndpointsCoolDown(t *testing.T) {
	for _, endpoint := range []string{"images/generations", "alpha/search"} {
		t.Run(endpoint, func(t *testing.T) {
			want := time.Now().Add(time.Hour).Truncate(time.Second)
			account, limits, repo, calls := cprCooldownFixture(t, cprExhaustedDetail(want))
			account.Credentials["pool_mode"] = true
			svc := &OpenAIGatewayService{cfg: cprTestConfig(), rateLimitService: limits, httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: 429, Header: http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{"error":{"type":"usage_limit_reached"}}`)),
			}}}
			body := []byte(`{"model":"gpt-5.5","prompt":"draw a cat","input":"hi","id":"search","commands":{}}`)
			if endpoint == "images/generations" {
				body = []byte(strings.ReplaceAll(string(body), "gpt-5.5", "gpt-image-1"))
			}
			c, _, _ := rawRelayTestContext(t, context.Background(), "/v1/"+endpoint, body, "", nil)
			var err error
			switch endpoint {
			case "images/generations":
				parsed, parseErr := svc.ParseOpenAIImagesRequest(c, body)
				require.NoError(t, parseErr)
				_, err = svc.ForwardImages(context.Background(), c, account, body, parsed, "")
			case "alpha/search":
				_, err = svc.ForwardAlphaSearch(context.Background(), c, account, body)
			}
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure)
			require.False(t, failure.RetryableOnSameAccount)
			require.True(t, failure.SameAccountRetryDeadline.IsZero())
			got, writes := repo.state()
			require.WithinDuration(t, want, got, time.Second)
			require.Equal(t, 1, writes)
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestCPRQuotaAdminTimeoutUsesFallbackAfterCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	account := newCPRTestAccount()
	account.Credentials["admin_base_url"] = server.URL
	repo := &cprCooldownRepo{}
	limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	settings := newMockSettingRepo()
	settings.data[SettingKeyRateLimit429CooldownSettings] = `{"enabled":true,"cooldown_seconds":1}`
	limits.SetSettingService(NewSettingService(settings, cprTestConfig()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := time.Now()
	limits.handleCPRQuotaFailure(ctx, account)
	require.Less(t, time.Since(before), 4*time.Second)
	got, writes := repo.state()
	require.Equal(t, 1, writes)
	require.WithinDuration(t, time.Now().Add(time.Second), got, 500*time.Millisecond)
}

func TestCPRQuotaWSErrorFrameResetHeadersSurviveAdminFailure(t *testing.T) {
	account, limits, repo, calls := cprCooldownFixture(t, `{"code":40103}`)
	capture := &cprUsageCapture{streaming: true}
	before := time.Now()
	capture.observe([]byte(`{"type":"error","status":429,"error":{"type":"usage_limit_reached"},"headers":{"x-codex-primary-used-percent":"100","x-codex-primary-reset-after-seconds":"3600","x-codex-primary-window-minutes":"300"}}`), "")
	gw := &OpenAIGatewayService{rateLimitService: limits}
	require.True(t, gw.applyCapturedCPRQuota(context.Background(), account, capture))
	got, writes := repo.state()
	require.WithinDuration(t, before.Add(time.Hour), got, time.Second)
	require.Equal(t, 1, writes)
	require.Zero(t, calls.Load())
}

func TestCPRQuotaFallbackRespectsSettings(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		account, limits, repo, calls := cprCooldownFixture(t, `{"code":40103}`)
		settings := newMockSettingRepo()
		settings.data[SettingKeyRateLimit429CooldownSettings] = fmt.Sprintf(`{"enabled":%t,"cooldown_seconds":12}`, enabled)
		limits.SetSettingService(NewSettingService(settings, cprTestConfig()))
		gw := &OpenAIGatewayService{rateLimitService: limits}
		limits.SetAccountRuntimeBlocker(gw)
		before := time.Now()
		limits.handleCPRQuotaFailure(context.Background(), account)
		limits.handleCPRQuotaFailure(context.Background(), account)
		got, writes := repo.state()
		require.EqualValues(t, 1, calls.Load(), "failed management queries also need storm protection")
		if enabled {
			require.Equal(t, 2, writes)
			require.WithinDuration(t, before.Add(12*time.Second), got, time.Second)
			require.True(t, gw.isOpenAIAccountRuntimeBlocked(account))
		} else {
			require.Zero(t, writes)
			require.False(t, gw.isOpenAIAccountRuntimeBlocked(account))
		}
	}
}

func TestCPRQuotaWSFramePrecedesManagementQueryAndRetainsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	queryStarted, release := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryStarted <- struct{}{}
		select {
		case <-release:
			_, _ = io.WriteString(w, cprExhaustedDetail(want))
		case <-r.Context().Done():
		}
	}))
	defer admin.Close()
	upstream := newRawRelayWSUpstream(t, nil)
	account := newCPRTestAccount()
	account.Credentials["base_url"] = upstream.server.URL
	account.Credentials["admin_base_url"] = admin.URL
	repo := &cprCooldownRepo{}
	limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	svc := &OpenAIGatewayService{cfg: cprTestConfig(), rateLimitService: limits}
	settled := make(chan *OpenAIForwardResult, 1)
	ingress, serverErr := startPassthroughLifecycleServerWithHooks(t, context.Background(), svc, account, func(*gin.Context) *OpenAIWSIngressHooks {
		return &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) { settled <- result }}
	})
	defer ingress.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(ingress.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(rawRelayWSFirstFrame)))
	rawRelayRecv(t, upstream.frames)
	up := rawRelayRecv(t, upstream.conns)
	frame := `{"type":"response.failed","response":{"id":"resp_quota","error":{"code":"usage_limit_reached"},"usage":{"input_tokens":11,"output_tokens":3}}}`
	require.NoError(t, up.Write(ctx, coderws.MessageText, []byte(frame)))
	rawRelayRecv(t, queryStarted)
	_, got, err := client.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, frame, string(got))
	result := rawRelayRecv(t, settled)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, "complete", result.CodexObservation.Usage.Status)
	unblock()
	require.Eventually(t, func() bool { until, writes := repo.state(); return writes == 1 && until.Equal(want) }, time.Second, 10*time.Millisecond)
	_ = client.CloseNow()
	rawRelayRecv(t, serverErr)
	rawRelayRecv(t, upstream.ended)
}

func TestCPRQuotaWSHandshakeCoolDown(t *testing.T) {
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	account, limits, repo, calls := cprCooldownFixture(t, cprExhaustedDetail(want))
	svc := &OpenAIGatewayService{cfg: cprTestConfig(), rateLimitService: limits}
	c, _, _ := rawRelayTestContext(t, context.Background(), "/v1/responses", nil, "", nil)
	err := svc.openAIRawRelayWSDialError(context.Background(), c, nil, account, 429, nil, &openAIWSHandshakeError{
		Body: []byte(`{"error":{"code":"usage_limit_reached"}}`),
	})
	var failure *UpstreamFailoverError
	require.ErrorAs(t, err, &failure)
	require.False(t, failure.RetryableOnSameAccount)
	got, writes := repo.state()
	require.WithinDuration(t, want, got, time.Second)
	require.Equal(t, 1, writes)
	require.EqualValues(t, 1, calls.Load())
}
