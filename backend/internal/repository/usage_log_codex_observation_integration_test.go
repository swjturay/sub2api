//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestUsageLog_CodexObservationPersistence proves codex_observation round-trips from insert to
// read and is omitted (NULL) when absent.
func TestUsageLog_CodexObservationPersistence(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUsageLogRepositoryWithSQL(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{Email: "observation-" + uuid.NewString() + "@example.com"})
	t.Cleanup(func() { require.NoError(t, client.User.DeleteOneID(user.ID).Exec(ctx)) })
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-session-" + uuid.NewString(), Name: "k"})
	t.Cleanup(func() { require.NoError(t, client.APIKey.DeleteOneID(apiKey.ID).Exec(ctx)) })
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-session-" + uuid.NewString()})
	t.Cleanup(func() { require.NoError(t, client.Account.DeleteOneID(account.ID).Exec(ctx)) })
	// Keep the real batch-insert path, but do not leave usage in the shared
	// database where later dashboard suites would count these two requests.
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE api_key_id = $1", apiKey.ID)
		require.NoError(t, err)
	})

	observation := &service.CodexObservation{
		Transport: "http",
		Safety:    &service.CodexSafetyObservation{EnabledPresent: true},
		Route:     &service.CodexRouteObservation{OutboundDigest: "v1:out", ResponseDigest: "v1:response"},
		Usage:     &service.CPRUsageObservation{Status: "complete", TerminalEvent: "response.completed"},
	}

	withSession := &service.UsageLog{
		UserID:           user.ID,
		APIKeyID:         apiKey.ID,
		AccountID:        account.ID,
		RequestID:        uuid.NewString(),
		Model:            "claude-3",
		InputTokens:      10,
		OutputTokens:     5,
		TotalCost:        1.0,
		ActualCost:       1.0,
		CodexObservation: observation,
		CreatedAt:        time.Now().UTC(),
	}
	_, err := repo.Create(ctx, withSession)
	require.NoError(t, err)
	require.NotZero(t, withSession.ID)

	withoutSession := &service.UsageLog{
		UserID:       user.ID,
		APIKeyID:     apiKey.ID,
		AccountID:    account.ID,
		RequestID:    uuid.NewString(),
		Model:        "claude-3",
		InputTokens:  7,
		OutputTokens: 3,
		TotalCost:    0.5,
		ActualCost:   0.5,
		CreatedAt:    time.Now().UTC(),
	}
	_, err = repo.Create(ctx, withoutSession)
	require.NoError(t, err)

	// Round-trip: observation survives insert → read.
	got, err := repo.GetByID(ctx, withSession.ID)
	require.NoError(t, err)
	require.Equal(t, observation, got.CodexObservation)

	// Omission: absent observation reads back as nil (NULL), not empty string.
	gotNone, err := repo.GetByID(ctx, withoutSession.ID)
	require.NoError(t, err)
	require.Nil(t, gotNone.CodexObservation)
}
