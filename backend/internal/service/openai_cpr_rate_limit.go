package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
)

// Only structured quota errors are account-quota evidence. CPR also emits
// key-budget/queue 429s and capacity 503s; their existing policy is unchanged.
func isCPRQuotaError(body []byte) bool {
	if !gjson.ValidBytes(body) {
		return false
	}
	for _, path := range []string{"error", "response.error"} {
		if isCPRQuotaErrorObject(gjson.GetBytes(body, path)) {
			return true
		}
	}
	return false
}

func isCPRQuotaErrorObject(err gjson.Result) bool {
	// A specific model/key/capacity code takes precedence over a broad type.
	code := err.Get("code").String()
	if code == "" {
		code = err.Get("type").String()
	}
	return code == "usage_limit_reached" || code == "quota_exhausted"
}

// Preserve structured upstream reset evidence even when management is unavailable.
// Retry-After alone is not proof of an account quota window.
func cprQuotaResponseReset(headers http.Header, body []byte) time.Time {
	var until time.Time
	if reset := calculateOpenAI429ResetTime(headers); reset != nil && reset.After(time.Now()) {
		until = *reset
	}
	for _, path := range []string{"error", "response.error"} {
		err := gjson.GetBytes(body, path)
		if !isCPRQuotaErrorObject(err) {
			continue
		}
		var reset time.Time
		if seconds := err.Get("resets_at").Int(); seconds > 0 {
			reset = time.Unix(seconds, 0)
		} else if seconds := err.Get("resets_in_seconds").Int(); seconds > 0 && seconds <= int64((1<<63-1)/time.Second) {
			reset = time.Now().Add(time.Duration(seconds) * time.Second)
		}
		if reset.After(time.Now()) && reset.After(until) {
			until = reset
		}
	}
	return until
}

// Error-path cache only: coalesce duplicate attempts without treating the
// management fetch time as a fresh OpenAI quota observation.
type cprRateLimitState struct {
	state   *CPRAccountState
	err     error
	expires time.Time
}

const cprRateLimitQueryTimeout = 2 * time.Second

func (s *CPRQuotaService) fetchRateLimitState(ctx context.Context, account *Account) (*CPRAccountState, error) {
	identity, _ := json.Marshal([]string{account.GetCPRAdminBaseURL(), account.GetCPRAdminAPIKey(), account.GetCPRAccountID()})
	key := fmt.Sprintf("%d:%x", account.ID, sha256.Sum256(identity))
	load := func() (cprRateLimitState, bool) {
		v, ok := s.rateLimitCache.Load(key)
		if !ok {
			return cprRateLimitState{}, false
		}
		entry, ok := v.(*cprRateLimitState)
		if !ok || entry == nil {
			return cprRateLimitState{}, false
		}
		return *entry, time.Now().Before(entry.expires)
	}
	if entry, ok := load(); ok {
		return entry.state, entry.err
	}
	result := s.rateLimitQueries.DoChan(key, func() (any, error) {
		if entry, ok := load(); ok {
			return entry, nil
		}
		queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cprRateLimitQueryTimeout)
		defer cancel()
		state, err := s.FetchAccountState(queryCtx, account)
		now := time.Now()
		entry := cprRateLimitState{state: state, err: err, expires: now.Add(defaultRateLimit429CooldownSeconds * time.Second)}
		s.rateLimitCache.Range(func(k, v any) bool {
			if cached, ok := v.(*cprRateLimitState); ok && !now.Before(cached.expires) {
				s.rateLimitCache.CompareAndDelete(k, v)
			}
			return true
		})
		s.rateLimitCache.Store(key, &entry)
		return entry, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-result:
		if r.Err != nil {
			return nil, r.Err
		}
		entry, ok := r.Val.(cprRateLimitState)
		if !ok {
			return nil, errors.New("invalid CPR rate limit query result")
		}
		return entry.state, entry.err
	}
}

// Called only after a received quota error. It extends existing cooldown state;
// it never disables an account, clears a block or starts a recovery probe.
func (s *RateLimitService) handleCPRQuotaFailure(ctx context.Context, account *Account, responseReset ...time.Time) {
	if s == nil || account == nil || !account.IsCPR() {
		return
	}
	account = snapshotOpenAIOutboundAccount(account)
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	now := time.Now()
	var until time.Time
	for _, reset := range responseReset {
		if reset.After(now) && reset.After(until) {
			until = reset
		}
	}
	hasResponseReset := until.After(now)
	cooldown, fallbackEnabled := s.get429FallbackCooldown(stateCtx, account)
	if fallbackEnabled && cooldown > 0 {
		if fallback := now.Add(cooldown); fallback.After(until) {
			until = fallback
		}
	}
	if until.After(now) {
		s.notifyAccountSchedulingBlocked(account, until, "cpr_quota")
	}
	if !hasResponseReset && s.cprQuotaService != nil {
		state, err := s.cprQuotaService.fetchRateLimitState(stateCtx, account)
		if err == nil && state != nil && state.AccountID == account.GetCPRAccountID() {
			var reset *time.Time
			switch state.Status {
			case CPRAccountStatusQuotaExhausted:
				reset = state.QuotaResetAt
			case CPRAccountStatusRateLimited:
				reset = state.RateLimitedUntil
			}
			if reset != nil && reset.After(until) && reset.After(time.Now()) {
				until = *reset
			}
		} else if err != nil {
			slog.Warn("cpr_quota_cooldown_query_failed", "account_id", account.ID)
		}
	}
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(until) {
		until = *account.RateLimitResetAt
	}
	// A short configured cooldown may have elapsed during the bounded admin query.
	if !until.After(time.Now()) && fallbackEnabled && cooldown > 0 {
		until = time.Now().Add(cooldown)
	}
	if !until.After(time.Now()) {
		return
	}
	s.notifyAccountSchedulingBlocked(account, until, "cpr_quota")
	if s.accountRepo == nil {
		return
	}
	err := s.setRateLimitedPreservingCPR(stateCtx, account, until)
	if err != nil {
		slog.Warn("cpr_quota_cooldown_write_failed", "account_id", account.ID)
	}
}

func (s *OpenAIGatewayService) applyCapturedCPRQuota(ctx context.Context, account *Account, capture *cprUsageCapture) bool {
	if capture == nil {
		return false
	}
	capture.mu.Lock()
	quota := capture.quotaFailure
	reset := capture.quotaReset
	handled := capture.quotaHandled
	capture.quotaHandled = quota || handled
	capture.mu.Unlock()
	if quota && !handled && s.rateLimitService != nil {
		s.rateLimitService.handleCPRQuotaFailure(ctx, account, reset)
	}
	return quota
}

// Keep an in-flight generic CPR 429 from shortening a previously confirmed
// quota reset. Non-CPR persistence keeps its existing semantics.
func (s *RateLimitService) setRateLimitedPreservingCPR(ctx context.Context, account *Account, until time.Time) error {
	if account.IsCPR() {
		if repo, ok := s.accountRepo.(grokRateLimitExtendingRepository); ok {
			return repo.SetRateLimitedIfLater(ctx, account.ID, until)
		}
		if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(until) {
			until = *account.RateLimitResetAt
		}
	}
	return s.accountRepo.SetRateLimited(ctx, account.ID, until)
}
