//go:build unit

package service

import (
	"context"
	"encoding/json"
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
	"github.com/stretchr/testify/require"
)

type cprHealthRepo struct {
	AccountRepository
	mu      sync.Mutex
	account *Account
	writes  int
}

func (r *cprHealthRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	return r.SetRateLimitedIfLater(context.Background(), id, resetAt)
}

func (r *cprHealthRepo) SetRateLimitedIfLater(_ context.Context, _ int64, resetAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.account.RateLimitResetAt != nil && !resetAt.After(*r.account.RateLimitResetAt) {
		return nil
	}
	now := time.Now()
	r.account.RateLimitedAt = &now
	r.account.RateLimitResetAt = &resetAt
	r.writes++
	return nil
}

func (r *cprHealthRepo) GetByID(context.Context, int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return snapshotOpenAIOutboundAccount(r.account), nil
}

func (r *cprHealthRepo) SetCPRAccountErrorIfMatch(_ context.Context, _ int64, credentials string, updatedAt time.Time, reason string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, _ := json.Marshal(r.account.Credentials)
	if string(current) != credentials || !r.account.UpdatedAt.Equal(updatedAt) || !r.account.IsActive() || !r.account.Schedulable {
		return false, nil
	}
	r.account.Status, r.account.Schedulable, r.account.ErrorMessage = StatusError, false, reason
	r.writes++
	return true, nil
}

func TestCPRHealthFailureStopsScheduling(t *testing.T) {
	detail := strings.Replace(cprTestDetailJSON("error", "free", ""), `"errorReason":null`, `"errorReason":"access_token_expired"`, 1)
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, detail) }))
	defer admin.Close()
	svc, account := rawRelayTestSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"error":{"code":"no_available_provider","type":"server_error"}}`)
	})
	account.Credentials["admin_base_url"] = admin.URL
	repo := &cprHealthRepo{account: snapshotOpenAIOutboundAccount(account)}
	limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	svc.rateLimitService = limits
	limits.SetAccountRuntimeBlocker(svc)
	body := []byte(`{"model":"gpt-5.5","input":"hello"}`)
	c, _, _ := rawRelayTestContext(t, context.Background(), "/v1/responses", body, "", nil)
	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	current, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	require.False(t, current.IsSchedulable(), "a confirmed invalid CPR credential must not receive the next request")
	require.Contains(t, current.ErrorMessage, "access_token_expired")
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5"), "the old scheduler snapshot must also be blocked")
}

func TestCPRHealthFailureAcrossHTTPProtocols(t *testing.T) {
	for _, protocol := range []string{"responses", "chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", protocol, stream), func(t *testing.T) {
				detail := strings.Replace(cprTestDetailJSON("error", "free", ""), `"errorReason":null`, `"errorReason":"access_token_expired"`, 1)
				admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, detail) }))
				defer admin.Close()
				frame := "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\",\"error\":{\"code\":\"token_revoked\"},\"usage\":{\"input_tokens\":11,\"output_tokens\":3}}}\n\n"
				svc, account := rawRelayTestSetup(t, func(w http.ResponseWriter, r *http.Request) {
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, frame)
					} else {
						w.WriteHeader(503)
						_, _ = io.WriteString(w, `{"error":{"code":"no_available_provider"}}`)
					}
				})
				account.Credentials["admin_base_url"] = admin.URL
				repo := &cprHealthRepo{account: snapshotOpenAIOutboundAccount(account)}
				limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
				svc.rateLimitService = limits
				limits.SetAccountRuntimeBlocker(svc)
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.5","input":"hi","messages":[{"role":"user","content":"hi"}],"max_tokens":10,"stream":%t}`, stream))
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
				got, _ := repo.GetByID(context.Background(), account.ID)
				require.False(t, got.IsSchedulable())
				if stream {
					require.NotNil(t, result)
					require.Equal(t, 11, result.Usage.InputTokens)
					require.Equal(t, 3, result.Usage.OutputTokens)
					if protocol == "responses" {
						require.Equal(t, frame, rec.Body.String())
					}
				}
			})
		}
	}
}

func TestCPRHealthWSFailurePreservesFrameAndQuarantines(t *testing.T) {
	detail := strings.Replace(cprTestDetailJSON("error", "free", ""), `"errorReason":null`, `"errorReason":"access_token_expired"`, 1)
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, detail) }))
	defer admin.Close()
	upstream := newRawRelayWSUpstream(t, nil)
	account := newCPRTestAccount()
	account.Credentials["base_url"] = upstream.server.URL
	account.Credentials["admin_base_url"] = admin.URL
	repo := &cprHealthRepo{account: snapshotOpenAIOutboundAccount(account)}
	limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	svc := &OpenAIGatewayService{cfg: cprTestConfig(), rateLimitService: limits}
	limits.SetAccountRuntimeBlocker(svc)
	ingress, serverErr := startPassthroughLifecycleServerWithHooks(t, context.Background(), svc, account, nil)
	defer ingress.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(ingress.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(rawRelayWSFirstFrame)))
	rawRelayRecv(t, upstream.frames)
	up := rawRelayRecv(t, upstream.conns)
	frame := `{"type":"error","status":503,"error":{"code":"no_available_provider"}}`
	require.NoError(t, up.Write(ctx, coderws.MessageText, []byte(frame)))
	_, got, err := client.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, frame, string(got))
	require.Eventually(t, func() bool { a, _ := repo.GetByID(ctx, account.ID); return !a.IsSchedulable() }, time.Second, 10*time.Millisecond)
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(rawRelayWSFirstFrame)))
	_, _, err = client.Read(ctx)
	require.Error(t, err, "a quarantined live connection must reject the next turn")
	_ = client.CloseNow()
	rawRelayRecv(t, serverErr)
	select {
	case <-upstream.frames:
		t.Fatal("second turn reached quarantined upstream")
	default:
	}
	rawRelayRecv(t, upstream.ended)
}

func (r *cprHealthRepo) ListByPlatform(ctx context.Context, _ string) ([]Account, error) {
	a, err := r.GetByID(ctx, 0)
	return []Account{*a}, err
}

func cprHealthFixture(t *testing.T, status, reason string) (*RateLimitService, *cprHealthRepo, *OpenAIGatewayService) {
	t.Helper()
	detail := strings.Replace(cprTestDetailJSON(status, "free", ""), `"errorReason":null`, fmt.Sprintf(`"errorReason":%q`, reason), 1)
	if status == "disabled" {
		detail = strings.Replace(detail, `"enabled":true`, `"enabled":false`, 1)
	}
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, detail) }))
	t.Cleanup(admin.Close)
	account := newCPRTestAccount()
	account.Credentials["admin_base_url"] = admin.URL
	repo := &cprHealthRepo{account: account}
	limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: limits}
	limits.SetAccountRuntimeBlocker(svc)
	return limits, repo, svc
}

func TestCPRHealthOnlyConfirmedCredentialFailuresQuarantine(t *testing.T) {
	for _, tc := range []struct {
		status, reason string
		wantQuarantine bool
		wantCooldown   bool
	}{
		{"error", "access_token_expired", true, false}, {"error", "credential_invalid", true, false}, {"disabled", "", true, false},
		{"normal", "access_token_expired", false, false}, {"error", "upstream_capacity_unavailable", false, false},
		{"error", "", false, false}, {"quota_exhausted", "", false, true}, {"rate_limited", "", false, true},
	} {
		t.Run(tc.status+"/"+tc.reason, func(t *testing.T) {
			limits, repo, _ := cprHealthFixture(t, tc.status, tc.reason)
			account, _ := repo.GetByID(context.Background(), 0)
			limits.checkCPRAccountHealth(context.Background(), account, true)
			got, _ := repo.GetByID(context.Background(), 0)
			require.Equal(t, tc.wantQuarantine, got.Status == StatusError)
			require.Equal(t, tc.wantCooldown, got.RateLimitResetAt != nil)
		})
	}
}

func TestCPRHealthBackgroundScanStopsWithoutInference(t *testing.T) {
	limits, repo, _ := cprHealthFixture(t, "error", "access_token_expired")
	worker := NewCPRAccountHealthService(limits)
	worker.Start()
	worker.Start()
	defer worker.Stop()
	require.Eventually(t, func() bool { a, _ := repo.GetByID(context.Background(), 0); return !a.IsSchedulable() }, time.Second, 10*time.Millisecond)
}

func TestCPRHealthBackgroundScanProjectsTemporaryQuotaState(t *testing.T) {
	future := time.Now().Add(time.Hour).Truncate(time.Second)
	detail := strings.Replace(
		cprTestDetailJSON("rate_limited", "free", ""),
		`"rateLimitedUntil":null`,
		fmt.Sprintf(`"rateLimitedUntil":%q`, future.In(time.FixedZone("CST", 8*3600)).Format(cprDisplayTimeLayout)),
		1,
	)
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, detail) }))
	defer admin.Close()
	account := newCPRTestAccount()
	account.Credentials["admin_base_url"] = admin.URL
	repo := &cprHealthRepo{account: account}
	limits := NewRateLimitService(repo, nil, cprTestConfig(), nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: limits}
	limits.SetAccountRuntimeBlocker(svc)
	worker := NewCPRAccountHealthService(limits)
	worker.reconcile()

	got, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotNil(t, got.RateLimitResetAt)
	require.WithinDuration(t, future, *got.RateLimitResetAt, time.Second)
	require.False(t, got.IsSchedulable(), "a CPR management rate limit must remove the binding from scheduling")
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(got))
	require.Equal(t, 1, repo.writes)
}

func TestCPRHealthQuotaStateWithoutResetUsesBoundedProjection(t *testing.T) {
	limits, repo, svc := cprHealthFixture(t, "quota_exhausted", "")
	before := time.Now()
	limits.checkCPRAccountHealth(context.Background(), repo.account, false)

	got, err := repo.GetByID(context.Background(), repo.account.ID)
	require.NoError(t, err)
	require.NotNil(t, got.RateLimitResetAt)
	require.GreaterOrEqual(t, got.RateLimitResetAt.Sub(before), cprManagementStateFallbackCooldown-time.Second)
	require.False(t, got.IsSchedulable())
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(got))
}

func TestCPRHealthNeverRestoresManualSuspension(t *testing.T) {
	limits, repo, _ := cprHealthFixture(t, "normal", "")
	repo.account.Status = "inactive"
	repo.account.Schedulable = false
	worker := NewCPRAccountHealthService(limits)
	worker.reconcile()
	got, _ := repo.GetByID(context.Background(), 0)
	require.Equal(t, "inactive", got.Status)
	require.False(t, got.Schedulable)
	require.Zero(t, repo.writes)
}

func TestCPRHealthReauthorizationDuringQueryRejectsOldResult(t *testing.T) {
	for _, change := range []string{"credentials", "recovery", "manual_pause"} {
		t.Run(change, func(t *testing.T) {
			limits, repo, svc := cprHealthFixture(t, "error", "access_token_expired")
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				<-release
				_, _ = io.WriteString(w, strings.Replace(cprTestDetailJSON("error", "free", ""), `"errorReason":null`, `"errorReason":"access_token_expired"`, 1))
			}))
			defer admin.Close()
			repo.account.Credentials["admin_base_url"] = admin.URL
			account, _ := repo.GetByID(context.Background(), 0)
			done := make(chan struct{})
			go func() { limits.checkCPRAccountHealth(context.Background(), account, true); close(done) }()
			<-started
			repo.mu.Lock()
			switch change {
			case "credentials":
				repo.account.Credentials["api_key"] = "new-client-key"
			case "manual_pause":
				repo.account.Status = "inactive"
				repo.account.Schedulable = false
			}
			repo.account.UpdatedAt = time.Now()
			repo.mu.Unlock()
			unblock()
			<-done
			got, _ := repo.GetByID(context.Background(), 0)
			require.NotEqual(t, StatusError, got.Status)
			require.Zero(t, repo.writes)
			require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5"))
		})
	}
}

func TestCPRHealthUnknownManagementDoesNotPermanentlyDisable(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"invalid_admin", `{"code":40103,"message":"secret"}`, 401},
		{"wrong_binding", strings.ReplaceAll(cprTestDetailJSON("error", "free", ""), cprTestCPRAccount, "wrong"), 200},
		{"bad_json", "not-json", 200},
		{"missing_status", cprTestDetailJSON("", "free", ""), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits, repo, svc := cprHealthFixture(t, "normal", "")
			admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer admin.Close()
			repo.account.Credentials["admin_base_url"] = admin.URL
			a, _ := repo.GetByID(context.Background(), 0)
			limits.checkCPRAccountHealth(context.Background(), a, true)
			got, _ := repo.GetByID(context.Background(), 0)
			require.Equal(t, StatusActive, got.Status)
			require.True(t, got.Schedulable)
			require.Zero(t, repo.writes)
			require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(a, "gpt-5.5"))
		})
	}
}

func TestCPRHealthWSHandshakeStopsScheduling(t *testing.T) {
	limits, repo, svc := cprHealthFixture(t, "error", "access_token_expired")
	svc.rateLimitService = limits
	a, _ := repo.GetByID(context.Background(), 0)
	c, _, _ := rawRelayTestContext(t, context.Background(), "/v1/responses", nil, "", nil)
	err := svc.openAIRawRelayWSDialError(context.Background(), c, nil, a, 503, nil, &openAIWSHandshakeError{Body: []byte(`{"error":{"code":"no_available_provider"}}`)})
	var failure *UpstreamFailoverError
	require.ErrorAs(t, err, &failure)
	got, _ := repo.GetByID(context.Background(), 0)
	require.False(t, got.IsSchedulable())
}

func TestCPRHealthLateManagementFailureDoesNotBlockRecovery(t *testing.T) {
	limits, repo, svc := cprHealthFixture(t, "normal", "")
	started, release := make(chan struct{}), make(chan struct{})
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"code":503}`)
	}))
	defer admin.Close()
	repo.account.Credentials["admin_base_url"] = admin.URL
	a, _ := repo.GetByID(context.Background(), 0)
	done := make(chan struct{})
	go func() { limits.checkCPRAccountHealth(context.Background(), a, true); close(done) }()
	<-started
	repo.mu.Lock()
	repo.account.UpdatedAt = time.Now()
	repo.account.Credentials["api_key"] = "new-key"
	repo.mu.Unlock()
	svc.ClearAccountSchedulingBlock(a.ID)
	close(release)
	<-done
	current, _ := repo.GetByID(context.Background(), 0)
	require.True(t, current.IsSchedulable())
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(current, "gpt-5.5"))
}

func TestCPRHealthRuntimeBlockClearedByExplicitRecovery(t *testing.T) {
	limits, repo, svc := cprHealthFixture(t, "error", "access_token_expired")
	a, _ := repo.GetByID(context.Background(), 0)
	limits.checkCPRAccountHealth(context.Background(), a, true)
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(a, "gpt-5.5"))
	svc.ClearAccountSchedulingBlock(a.ID)
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(a, "gpt-5.5"))
}

func TestCPRHealthFreshChecksDoNotCacheRevokedState(t *testing.T) {
	limits, repo, svc := cprHealthFixture(t, "normal", "")
	var calls atomic.Int32
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		status, reason := "normal", ""
		if n == 1 {
			status, reason = "error", "access_token_expired"
		}
		_, _ = io.WriteString(w, strings.Replace(cprTestDetailJSON(status, "free", ""), `"errorReason":null`, fmt.Sprintf(`"errorReason":%q`, reason), 1))
	}))
	defer admin.Close()
	repo.account.Credentials["admin_base_url"] = admin.URL
	a, _ := repo.GetByID(context.Background(), 0)
	limits.checkCPRAccountHealth(context.Background(), a, true)
	require.Equal(t, 1, repo.writes)
	repo.mu.Lock()
	repo.account.Status = StatusActive
	repo.account.Schedulable = true
	repo.account.UpdatedAt = time.Now()
	repo.mu.Unlock()
	svc.ClearAccountSchedulingBlock(a.ID)
	a, _ = repo.GetByID(context.Background(), 0)
	limits.checkCPRAccountHealth(context.Background(), a, true)
	require.EqualValues(t, 2, calls.Load())
	require.Equal(t, 1, repo.writes)
}

func TestCPRHealthManagementTimeoutIsBounded(t *testing.T) {
	limits, repo, svc := cprHealthFixture(t, "normal", "")
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer admin.Close()
	repo.account.Credentials["admin_base_url"] = admin.URL
	a, _ := repo.GetByID(context.Background(), 0)
	start := time.Now()
	limits.checkCPRAccountHealth(context.Background(), a, true)
	require.Less(t, time.Since(start), 4*time.Second)
	got, _ := repo.GetByID(context.Background(), 0)
	require.True(t, got.IsSchedulable())
	require.Zero(t, repo.writes)
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(a, "gpt-5.5"))
}
