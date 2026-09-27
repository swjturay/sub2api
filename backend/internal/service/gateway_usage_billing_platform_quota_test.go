//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Key the fixture by user and concrete platform to exercise sharing across keys,
// groups and billing modes without sharing another user's or provider's budget.
type sharedPlatformQuotaCache struct {
	BillingCache
	entries           map[UserPlatformQuotaKey]*UserPlatformQuotaCacheEntry
	increments        []incrCall
	subscriptionUsage float64
}

func (c *sharedPlatformQuotaCache) GetUserPlatformQuotaCache(_ context.Context, userID int64, platform string) (*UserPlatformQuotaCacheEntry, bool, error) {
	entry, ok := c.entries[UserPlatformQuotaKey{UserID: userID, Platform: platform}]
	return entry, ok, nil
}

func (c *sharedPlatformQuotaCache) SetUserPlatformQuotaCache(_ context.Context, userID int64, platform string, entry *UserPlatformQuotaCacheEntry, _ time.Duration) error {
	c.entries[UserPlatformQuotaKey{UserID: userID, Platform: platform}] = entry
	return nil
}

func (c *sharedPlatformQuotaCache) IncrUserPlatformQuotaUsageCache(_ context.Context, userID int64, platform string, cost float64, ttl time.Duration, markDirty bool) error {
	c.increments = append(c.increments, incrCall{userID: userID, platform: platform, cost: cost, ttl: ttl, markDirty: markDirty})
	entry := c.entries[UserPlatformQuotaKey{UserID: userID, Platform: platform}]
	entry.DailyUsageUSD += cost
	entry.WeeklyUsageUSD += cost
	entry.MonthlyUsageUSD += cost
	return nil
}

func (c *sharedPlatformQuotaCache) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	return &SubscriptionCacheData{Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(24 * time.Hour), DailyUsage: c.subscriptionUsage}, nil
}

func (c *sharedPlatformQuotaCache) GetUserBalance(context.Context, int64) (float64, error) {
	return 100, nil
}

func (c *sharedPlatformQuotaCache) InvalidateUserBalance(context.Context, int64) error { return nil }

type platformQuotaBillingRepo struct {
	UserPlatformQuotaRepository
	writes chan incrCall
}

func (r *platformQuotaBillingRepo) GetByUserPlatform(context.Context, int64, string) (*UserPlatformQuotaRecord, error) {
	return nil, nil
}

func (r *platformQuotaBillingRepo) IncrementUsageWithReset(_ context.Context, userID int64, platform string, cost float64, _ time.Time) error {
	r.writes <- incrCall{userID: userID, platform: platform, cost: cost}
	return nil
}

type platformQuotaSubscriptionRepo struct {
	UserSubscriptionRepository
	usage float64
}

func (r *platformQuotaSubscriptionRepo) IncrementUsage(_ context.Context, _ int64, cost float64) error {
	r.usage += cost
	return nil
}

func newSharedPlatformQuotaFixture() (*sharedPlatformQuotaCache, *BillingCacheService, *billingDeps, *postUsageBillingParams) {
	now := time.Now()
	limit := 1.0
	cache := &sharedPlatformQuotaCache{entries: map[UserPlatformQuotaKey]*UserPlatformQuotaCacheEntry{
		{UserID: 7, Platform: PlatformDeepseek}: {
			SchemaVersion: UserPlatformQuotaCacheSchemaV1, DailyLimitUSD: &limit,
			DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now,
		},
	}}
	cfg := &config.Config{}
	cfg.Billing.UserPlatformQuotaCacheTTLSeconds = 60
	cfg.Database.UserPlatformQuotaFlusherEnabled = true
	repo := &platformQuotaBillingRepo{writes: make(chan incrCall, 10)}
	svc := &BillingCacheService{cache: cache, cfg: cfg, userPlatformQuotaRepo: repo, cacheWriteChan: make(chan cacheWriteTask, 10)}
	deps := &billingDeps{cfg: cfg, billingCacheService: svc, userPlatformQuotaRepo: repo, userSubRepo: &platformQuotaSubscriptionRepo{}, deferredService: &DeferredService{}}
	group := &Group{ID: 5, Platform: PlatformComposite, SubscriptionType: "subscription"}
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{TotalCost: 1, ActualCost: 0.5}, User: &User{ID: 7},
		APIKey: &APIKey{ID: 13, GroupID: &group.ID, Group: group}, Account: &Account{ID: 9},
		Subscription: &UserSubscription{ID: 11}, IsSubscriptionBill: true,
	}
	p.Platform = QuotaPlatform(WithResolvedTargetPlatform(context.Background(), PlatformDeepseek), p.APIKey)
	return cache, svc, deps, p
}

func TestSubscriptionPlatformQuotaEligibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*UserPlatformQuotaCacheEntry, *sharedPlatformQuotaCache, *postUsageBillingParams, *config.Config)
		want      error
	}{
		{name: "below limit"},
		{name: "daily exhausted", configure: func(e *UserPlatformQuotaCacheEntry, _ *sharedPlatformQuotaCache, _ *postUsageBillingParams, _ *config.Config) {
			e.DailyUsageUSD = 1
		}, want: ErrUserPlatformDailyQuotaExhausted},
		{name: "weekly exhausted", configure: func(e *UserPlatformQuotaCacheEntry, _ *sharedPlatformQuotaCache, _ *postUsageBillingParams, _ *config.Config) {
			e.WeeklyLimitUSD = e.DailyLimitUSD
			e.WeeklyUsageUSD = 1
		}, want: ErrUserPlatformWeeklyQuotaExhausted},
		{name: "monthly exhausted", configure: func(e *UserPlatformQuotaCacheEntry, _ *sharedPlatformQuotaCache, _ *postUsageBillingParams, _ *config.Config) {
			e.MonthlyLimitUSD = e.DailyLimitUSD
			e.MonthlyUsageUSD = 1
		}, want: ErrUserPlatformMonthlyQuotaExhausted},
		{name: "unlimited", configure: func(e *UserPlatformQuotaCacheEntry, _ *sharedPlatformQuotaCache, _ *postUsageBillingParams, _ *config.Config) {
			e.DailyLimitUSD = nil
			e.DailyUsageUSD = 100
		}},
		{name: "expired daily window", configure: func(e *UserPlatformQuotaCacheEntry, _ *sharedPlatformQuotaCache, _ *postUsageBillingParams, _ *config.Config) {
			old := time.Now().Add(-48 * time.Hour)
			e.DailyWindowStart = &old
			e.DailyUsageUSD = 1
		}},
		{name: "subscription still enforced", configure: func(e *UserPlatformQuotaCacheEntry, c *sharedPlatformQuotaCache, p *postUsageBillingParams, _ *config.Config) {
			p.APIKey.Group.DailyLimitUSD = e.DailyLimitUSD
			c.subscriptionUsage = 1
		}, want: ErrDailyLimitExceeded},
		{name: "simple mode still exempt", configure: func(e *UserPlatformQuotaCacheEntry, _ *sharedPlatformQuotaCache, _ *postUsageBillingParams, cfg *config.Config) {
			e.DailyUsageUSD = 1
			cfg.RunMode = config.RunModeSimple
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache, svc, deps, p := newSharedPlatformQuotaFixture()
			entry := cache.entries[UserPlatformQuotaKey{UserID: p.User.ID, Platform: p.Platform}]
			if tc.configure != nil {
				tc.configure(entry, cache, p, deps.cfg)
			}
			err := svc.CheckBillingEligibility(context.Background(), p.User, p.APIKey, p.APIKey.Group, p.Subscription, p.Platform)
			if tc.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}

func TestSubscriptionPlatformQuotaBillingPersistence(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, flusher := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%t/flusher=%t", legacy, flusher), func(t *testing.T) {
				cache, svc, deps, p := newSharedPlatformQuotaFixture()
				deps.cfg.Database.UserPlatformQuotaFlusherEnabled = flusher
				repo := &simpleModeUsageBillingRepoStub{}
				if legacy {
					postUsageBilling(context.Background(), p, deps)
					require.Equal(t, 0.5, deps.userSubRepo.(*platformQuotaSubscriptionRepo).usage)
				} else {
					applied, err := applyUsageBilling(context.Background(), "deepseek-req", nil, p, deps, repo)
					require.NoError(t, err)
					require.True(t, applied)
					require.Equal(t, 0.5, repo.cmds[0].SubscriptionCost)
					require.Zero(t, repo.cmds[0].BalanceCost, "subscription must not deduct the balance")
					write := <-svc.cacheWriteChan
					require.Equal(t, cacheWriteUpdateSubscriptionUsage, write.kind)
					require.Equal(t, 0.5, write.amount)
					applied, err = applyUsageBilling(context.Background(), "deepseek-req", nil, p, deps, repo)
					require.NoError(t, err)
					require.False(t, applied, "replayed billing must not increment platform usage twice")
				}
				require.Len(t, cache.increments, 1)
				require.Equal(t, incrCall{userID: 7, platform: PlatformDeepseek, cost: 0.5, ttl: time.Minute, markDirty: flusher}, cache.increments[0])
				writes := deps.userPlatformQuotaRepo.(*platformQuotaBillingRepo).writes
				if flusher {
					require.Empty(t, writes, "dirty cache is persisted by the existing flusher")
				} else {
					select {
					case write := <-writes:
						require.Equal(t, incrCall{userID: 7, platform: PlatformDeepseek, cost: 0.5}, write)
					case <-time.After(3 * time.Second):
						t.Fatal("platform usage was not persisted")
					}
				}
			})
		}
	}
}

func TestPlatformQuotaSharedAcrossSubscriptionBalanceAndKeys(t *testing.T) {
	cache, svc, deps, p := newSharedPlatformQuotaFixture()
	repo := &simpleModeUsageBillingRepoStub{}
	for i := range 2 {
		require.NoError(t, svc.CheckBillingEligibility(context.Background(), p.User, p.APIKey, p.APIKey.Group, p.Subscription, p.Platform))
		applied, err := applyUsageBilling(context.Background(), fmt.Sprintf("request-%d", i), nil, p, deps, repo)
		require.NoError(t, err)
		require.True(t, applied)
		if i == 0 {
			require.Equal(t, 0.5, repo.cmds[0].SubscriptionCost)
			next := *p
			p = &next
			p.APIKey = &APIKey{ID: 14, Group: &Group{ID: 6, Platform: PlatformDeepseek}}
			p.IsSubscriptionBill = false
			p.Subscription = nil
		}
	}
	require.Equal(t, 0.5, repo.cmds[1].BalanceCost)
	require.Zero(t, repo.cmds[1].SubscriptionCost)
	require.Len(t, cache.increments, 2)
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), p.User, p.APIKey, p.APIKey.Group, nil, p.Platform), ErrUserPlatformDailyQuotaExhausted)
	group := &Group{ID: 8, Platform: PlatformComposite, SubscriptionType: "subscription"}
	key := &APIKey{ID: 15, GroupID: &group.ID, Group: group}
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), p.User, key, group, &UserSubscription{ID: 12}, p.Platform), ErrUserPlatformDailyQuotaExhausted)
	// Other providers and users remain available even after this user's DeepSeek budget is exhausted.
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), p.User, key, group, &UserSubscription{ID: 12}, PlatformOpenAI))
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 8}, key, group, &UserSubscription{ID: 13}, PlatformDeepseek))
}

func TestSubscriptionPlatformQuotaSkipsFreeAndUnlimitedUsage(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%t/unlimited=%t", legacy, unlimited), func(t *testing.T) {
				cache, _, deps, p := newSharedPlatformQuotaFixture()
				if unlimited {
					cache.entries[UserPlatformQuotaKey{UserID: 7, Platform: PlatformDeepseek}].DailyLimitUSD = nil
				} else {
					p.Cost.ActualCost = 0
				}
				if legacy {
					postUsageBilling(context.Background(), p, deps)
				} else {
					_, err := applyUsageBilling(context.Background(), "free-or-unlimited", nil, p, deps, &simpleModeUsageBillingRepoStub{})
					require.NoError(t, err)
				}
				require.Empty(t, cache.increments)
				require.Empty(t, deps.userPlatformQuotaRepo.(*platformQuotaBillingRepo).writes)
			})
		}
	}
}
