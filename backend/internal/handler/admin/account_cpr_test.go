//go:build unit

package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCPRStoredAdminKeyNeverGoesToChangedAddress(t *testing.T) {
	existing := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeCPR, Credentials: map[string]any{"base_url": "http://127.0.0.1:18081", "admin_api_key": "test-stored-admin"}}
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"http://127.0.0.1:18081/", true},
		{"http://other.invalid:18081", false},
		{"http://127.0.0.1:18082", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			next := &service.Account{Type: service.AccountTypeCPR, Credentials: map[string]any{"base_url": tc.url}}
			err := resolveCPRAdminKey(next, existing)
			if tc.allowed {
				require.NoError(t, err)
				require.Equal(t, "test-stored-admin", next.GetCPRAdminAPIKey())
			} else {
				require.Error(t, err)
				require.Empty(t, next.GetCPRAdminAPIKey())
			}
		})
	}
}
