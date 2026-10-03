package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type mergeValidationAccountRepo struct {
	*upstreamBillingProbeAccountRepo
}

func (r *mergeValidationAccountRepo) ListShadowsByParent(context.Context, int64) ([]*Account, error) {
	return nil, nil
}

func TestAccountPlatformValidationAfterUpstreamMerge(t *testing.T) {
	for _, tc := range []struct {
		name, platform, accountType string
		valid                       bool
	}{
		{"openai_cpr", PlatformOpenAI, AccountTypeCPR, true},
		{"anthropic_cpr", PlatformAnthropic, AccountTypeCPR, false},
		{"typesafe_cpr", PlatformTypeSafe, AccountTypeCPR, false},
		{"typesafe_oauth", PlatformTypeSafe, AccountTypeOAuth, false},
		{"typesafe_apikey", PlatformTypeSafe, AccountTypeAPIKey, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			t.Run("create", func(t *testing.T) {
				repo := &upstreamBillingProbeAccountRepo{accounts: make(map[int64]*Account)}
				result, err := NewAccountService(repo, nil).Create(ctx, CreateAccountRequest{
					Name: tc.name, Platform: tc.platform, Type: tc.accountType,
				})
				if tc.valid {
					require.NoError(t, err)
					require.Equal(t, tc.accountType, result.Type)
				} else {
					require.Error(t, err)
					require.Empty(t, repo.accounts)
				}
			})
			t.Run("admin_update", func(t *testing.T) {
				account := &Account{ID: 1, Platform: tc.platform, Type: AccountTypeAPIKey, Status: StatusActive}
				repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{1: account}}
				svc := &adminServiceImpl{accountRepo: &mergeValidationAccountRepo{repo}}
				result, err := svc.UpdateAccount(ctx, 1, &UpdateAccountInput{Type: tc.accountType})
				if tc.valid {
					require.NoError(t, err)
					require.Equal(t, tc.accountType, result.Type)
				} else {
					require.Error(t, err)
					require.Equal(t, AccountTypeAPIKey, repo.accounts[1].Type)
				}
			})
		})
	}
}
