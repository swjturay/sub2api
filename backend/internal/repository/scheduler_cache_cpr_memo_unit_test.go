//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheMemoPreservesCPRPolicyAndHealthUpdates(t *testing.T) {
	cache, _, recorder := newSchedulerCacheMemoUnit(t, defaultSchedulerSnapshotWriteChunkSize)
	bucket := service.SchedulerBucket{GroupID: 9310, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	account := memoTestAccountOn(service.PlatformOpenAI, 9311, "cpr")
	account.Type = service.AccountTypeCPR
	account.Extra = map[string]any{service.CPRPlanTypeExtraKey: "pro"}
	account.Credentials = map[string]any{
		"model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"},
	}
	memoPublish(t, cache, bucket, account)
	first := memoRead(t, cache, bucket)
	recorder.reset()
	cached := memoRead(t, cache, bucket)
	require.Zero(t, recorder.countPrefix("mget "+schedulerAccountMetaPrefix), "exercise the in-process cache hit")
	for _, accounts := range [][]*service.Account{first, cached} {
		require.Len(t, accounts, 1)
		require.True(t, accounts[0].IsCPR())
		require.True(t, accounts[0].IsOpenAIChatGPTSubscription())
		require.True(t, accounts[0].IsSchedulable())
		require.Equal(t, "gpt-6-astra", accounts[0].GetMappedModel("gpt-6-astra"))
	}

	// A confirmed quarantine must invalidate the hot snapshot immediately.
	account.Status, account.Schedulable = service.StatusError, false
	require.NoError(t, cache.SetAccount(context.Background(), &account))
	quarantined := memoRead(t, cache, bucket)[0]
	require.False(t, quarantined.IsSchedulable())
	require.Equal(t, service.StatusError, quarantined.Status)
	require.True(t, cached[0].IsSchedulable(), "previously returned copies remain unchanged")

	// Explicit administrative recovery must likewise replace the cached state.
	account.Status, account.Schedulable = service.StatusActive, true
	require.NoError(t, cache.SetAccount(context.Background(), &account))
	require.True(t, memoRead(t, cache, bucket)[0].IsSchedulable())
	require.False(t, quarantined.IsSchedulable())
}
