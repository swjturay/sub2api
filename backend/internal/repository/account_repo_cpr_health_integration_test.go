//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCPRQuarantineCASAndScheduling(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	makeAccount := func(name string) *service.Account {
		a := mustCreateAccount(t, tx.Client(), &service.Account{Name: name, Platform: service.PlatformOpenAI, Type: service.AccountTypeCPR, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": "http://cpr:8080", "api_key": "client", "admin_api_key": "admin", "cpr_account_id": "acct-test"}})
		got, err := repo.GetByID(ctx, a.ID)
		require.NoError(t, err)
		return got
	}
	events := func(id int64) int {
		rows, err := repo.sql.QueryContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id=$1 AND event_type=$2", id, service.SchedulerOutboxEventAccountChanged)
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var count int
		require.NoError(t, rows.Scan(&count))
		return count
	}
	a := makeAccount("cpr-cas-match")
	beforeEvents := events(a.ID)
	credentials, err := json.Marshal(a.Credentials)
	require.NoError(t, err)
	changed, err := repo.SetCPRAccountErrorIfMatch(ctx, a.ID, string(credentials), a.UpdatedAt, "CPR account unavailable: access_token_expired")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, beforeEvents+1, events(a.ID), "quarantine must publish one durable scheduler event")
	got, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusError, got.Status)
	require.False(t, got.Schedulable)
	candidates, err := repo.ListSchedulableByPlatform(ctx, service.PlatformOpenAI)
	require.NoError(t, err)
	for _, candidate := range candidates {
		require.NotEqual(t, a.ID, candidate.ID)
	}
	changed, err = repo.SetCPRAccountErrorIfMatch(ctx, a.ID, string(credentials), a.UpdatedAt, "duplicate")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, beforeEvents+1, events(a.ID), "duplicate quarantine must not enqueue another event")
	for _, kind := range []string{"credentials", "version", "manual_pause", "wrong_platform"} {
		t.Run(kind, func(t *testing.T) {
			a := makeAccount("cpr-cas-" + kind)
			original, _ := json.Marshal(a.Credentials)
			switch kind {
			case "credentials":
				a.Credentials["api_key"] = "new-key"
			case "version":
				a.UpdatedAt = a.UpdatedAt.Add(time.Second)
			case "manual_pause":
				a.Status = "inactive"
				a.Schedulable = false
			case "wrong_platform":
				a.Platform = service.PlatformAnthropic
			}
			if kind != "version" {
				require.NoError(t, repo.Update(ctx, a))
			}
			changed, err := repo.SetCPRAccountErrorIfMatch(ctx, a.ID, string(original), a.UpdatedAt, "stale failure")
			require.NoError(t, err)
			require.False(t, changed)
		})
	}
}
