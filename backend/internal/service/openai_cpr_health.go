package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tidwall/gjson"
)

// A relay 503 alone is not evidence of invalid credentials. Only the bound
// management account can authorize a durable quarantine.
type cprHealthRepository interface {
	SetCPRAccountErrorIfMatch(context.Context, int64, string, time.Time, string) (bool, error)
}

type cprHealthMutationBlocker interface {
	QuarantineCPRAccount(*Account, func() (bool, error)) (bool, error)
	CooldownCPRAccount(context.Context, *Account, AccountRepository)
}

// CPR quota states are temporary scheduling facts, not credential failures.
// Keep a missing/expired management reset bounded but longer than the five
// second request fallback so the minute health scan can refresh the state
// before the account is admitted again.
const cprManagementStateFallbackCooldown = 2 * time.Minute

func cprManagementStateResetAt(state *CPRAccountState, now time.Time) (time.Time, bool) {
	if state == nil {
		return time.Time{}, false
	}
	var reset *time.Time
	switch state.Status {
	case CPRAccountStatusRateLimited:
		reset = state.RateLimitedUntil
	case CPRAccountStatusQuotaExhausted:
		reset = state.QuotaResetAt
	default:
		return time.Time{}, false
	}
	if reset != nil && reset.After(now) {
		return *reset, true
	}
	return now.Add(cprManagementStateFallbackCooldown), true
}

// Project a CPR management quota state into the existing account cooldown.
// This is deliberately separate from credential quarantine: a quota state must
// stop routing immediately without turning a recoverable account into status
// error or changing its manual schedulable setting.
func (s *RateLimitService) projectCPRManagementCooldown(ctx context.Context, account *Account, state *CPRAccountState) error {
	if s == nil || account == nil || state == nil || s.accountRepo == nil {
		return nil
	}
	now := time.Now()
	until, ok := cprManagementStateResetAt(state, now)
	if !ok {
		return nil
	}
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(until) {
		until = *account.RateLimitResetAt
	}
	account.RateLimitResetAt = &until
	s.notifyAccountSchedulingBlocked(account, until, "cpr_management_quota")
	if err := s.setRateLimitedPreservingCPR(ctx, account, until); err != nil {
		return err
	}
	slog.Info("cpr_management_quota_cooldown", "account_id", account.ID, "status", state.Status, "until", until.UTC())
	return nil
}

func isCPRHealthError(body []byte) bool {
	if !gjson.ValidBytes(body) {
		return false
	}
	for _, path := range []string{"error", "response.error"} {
		switch gjson.GetBytes(body, path+".code").String() {
		case "no_available_provider", "token_revoked", "token_expired", "invalid_token", "access_token_expired", "refresh_token_expired", "refresh_token_reused", "refresh_token_invalidated", "account_deactivated":
			return true
		}
	}
	return false
}

func cprCredentialFailureReason(state *CPRAccountState) string {
	if state == nil {
		return ""
	}
	if state.Status == CPRAccountStatusDisabled && !state.Enabled {
		return "upstream_disabled"
	}
	if state.Status != CPRAccountStatusError {
		return ""
	}
	switch state.ErrorReason {
	case "access_token_expired", "credential_invalid", "token_revoked", "refresh_token_expired", "refresh_token_reused", "refresh_token_invalidated", "account_deactivated":
		return state.ErrorReason
	}
	return ""
}

// No terminal-state cache: reauthorization can change CPR without changing the
// Sub2API binding. Concurrent checks share one query, but later checks are fresh.
func (s *RateLimitService) checkCPRAccountHealth(ctx context.Context, account *Account, failedRequest bool) {
	if s == nil || account == nil || !account.IsCPR() || s.cprQuotaService == nil || s.accountRepo == nil {
		return
	}
	repo, ok := s.accountRepo.(cprHealthRepository)
	if !ok {
		return
	}
	credentials, err := json.Marshal(account.Credentials)
	if err != nil {
		return
	}
	key := fmt.Sprintf("%d:%x", account.ID, sha256.Sum256(credentials))
	result := s.cprQuotaService.healthQueries.DoChan(key, func() (any, error) {
		// Cancellation closes inference immediately; the bounded management check
		// still persists evidence already received from that attempt.
		stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAccountStateUpdateTimeout)
		defer cancel()
		latest, err := s.accountRepo.GetByID(stateCtx, account.ID)
		if err != nil || latest == nil {
			return nil, err
		}
		current, err := json.Marshal(latest.Credentials)
		if err != nil || string(current) != string(credentials) || !latest.IsCPR() || !latest.IsActive() || !latest.Schedulable {
			return nil, err
		}
		queryCtx, queryCancel := context.WithTimeout(stateCtx, cprRateLimitQueryTimeout)
		state, err := s.cprQuotaService.FetchAccountState(queryCtx, latest)
		queryCancel()
		if err != nil || state == nil || state.AccountID != latest.GetCPRAccountID() {
			return latest, fmt.Errorf("CPR management state unavailable or binding mismatch")
		}
		reason := cprCredentialFailureReason(state)
		if reason == "" {
			switch state.Status {
			case CPRAccountStatusNormal:
				return nil, nil
			case CPRAccountStatusQuotaExhausted, CPRAccountStatusRateLimited:
				if err := s.projectCPRManagementCooldown(stateCtx, latest, state); err != nil {
					return latest, err
				}
				return nil, nil
			}
			return latest, fmt.Errorf("CPR management returned unrecognized health state")
		}
		message := "CPR account unavailable: " + reason + "; reauthorize or enable upstream, then verify before restoring scheduling"
		write := func() (bool, error) {
			return repo.SetCPRAccountErrorIfMatch(stateCtx, latest.ID, string(current), latest.UpdatedAt, message)
		}
		var changed bool
		if blocker, ok := s.runtimeBlocker.(cprHealthMutationBlocker); ok {
			changed, err = blocker.QuarantineCPRAccount(latest, write)
		} else {
			changed, err = write()
		}
		if changed {
			slog.Warn("cpr_account_quarantined", "account_id", latest.ID, "reason", reason)
		}
		return latest, err
	})
	// The query is independently bounded; a cancelled client must not discard
	// the already-observed credential failure before it is recorded.
	r := <-result
	if r.Err != nil {
		slog.Warn("cpr_account_health_check_failed", "account_id", account.ID)
		if failedRequest {
			if observed, ok := r.Val.(*Account); ok && observed != nil {
				if blocker, ok := s.runtimeBlocker.(cprHealthMutationBlocker); ok {
					blocker.CooldownCPRAccount(ctx, observed, s.accountRepo)
				}
			}
		}
	}
}

// Hold the same lock used by administrative recovery while committing and
// bridging the cache refresh. A recovery racing the write clears this block;
// a recovery before it changes UpdatedAt and causes the CAS to reject it.
func (s *OpenAIGatewayService) QuarantineCPRAccount(account *Account, write func() (bool, error)) (bool, error) {
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	changed, err := write()
	if changed {
		s.cprHealthRuntimeBlocks.Store(account.ID, cprHealthRuntimeBlock{until: time.Now().Add(openAIStopSchedulingBridgeCooldown), observedAt: account.UpdatedAt})
	}
	return changed, err
}

func (s *OpenAIGatewayService) applyCapturedCPRHealth(ctx context.Context, account *Account, capture *cprUsageCapture) bool {
	if capture == nil || s.rateLimitService == nil {
		return false
	}
	capture.mu.Lock()
	failed := capture.healthFailure
	needed := failed && !capture.healthHandled
	capture.healthHandled = capture.healthHandled || needed
	capture.mu.Unlock()
	if needed {
		s.rateLimitService.checkCPRAccountHealth(ctx, account, true)
	}
	return failed
}

// CPR health blocks are independent of quota cooldowns. The generic runtime
// predicate intentionally clears blocks when a snapshot has no quota fields.
type cprHealthRuntimeBlock struct{ until, observedAt time.Time }

func (s *OpenAIGatewayService) isCPRHealthRuntimeBlocked(account *Account) bool {
	if account == nil || !account.IsCPR() {
		return false
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	value, ok := s.cprHealthRuntimeBlocks.Load(account.ID)
	if !ok {
		return false
	}
	block, ok := value.(cprHealthRuntimeBlock)
	if !ok || !time.Now().Before(block.until) || account.UpdatedAt.After(block.observedAt) {
		s.cprHealthRuntimeBlocks.Delete(account.ID)
		return false
	}
	return true
}

func (s *OpenAIGatewayService) CooldownCPRAccount(ctx context.Context, observed *Account, repo AccountRepository) {
	mu := s.openAIAccountRuntimeBlockLock(observed.ID)
	mu.Lock()
	defer mu.Unlock()
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	current, err := repo.GetByID(stateCtx, observed.ID)
	if err != nil || current == nil || !current.IsActive() || !current.Schedulable || !current.UpdatedAt.Equal(observed.UpdatedAt) {
		return
	}
	oldJSON, _ := json.Marshal(observed.Credentials)
	newJSON, _ := json.Marshal(current.Credentials)
	if string(oldJSON) != string(newJSON) {
		return
	}
	until := time.Now().Add(defaultRateLimit429CooldownSeconds * time.Second)
	if v, ok := s.cprHealthRuntimeBlocks.Load(current.ID); ok {
		if b, ok := v.(cprHealthRuntimeBlock); ok && b.until.After(until) {
			return
		}
	}
	s.cprHealthRuntimeBlocks.Store(current.ID, cprHealthRuntimeBlock{until: until, observedAt: current.UpdatedAt})
}

func (s *OpenAIGatewayService) cprWSTurnAvailable(ctx context.Context, account *Account, model string) bool {
	if s.rateLimitService == nil || s.rateLimitService.accountRepo == nil {
		return true
	}
	latest, err := s.rateLimitService.accountRepo.GetByID(ctx, account.ID)
	if err != nil || latest == nil || !latest.IsSchedulable() || s.isOpenAIAccountRequestRuntimeBlocked(latest, model) {
		return false
	}
	before, _ := json.Marshal(account.Credentials)
	after, _ := json.Marshal(latest.Credentials)
	return string(before) == string(after)
}
