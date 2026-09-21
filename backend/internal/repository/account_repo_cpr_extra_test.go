//go:build unit

package repository

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCPRProxyExtraDoesNotRebuildScheduler(t *testing.T) {
	updates := map[string]any{
		service.CPROutboundProxyExtraKey:          "socks5h://host:1080",
		service.CPROutboundProxyStatusExtraKey:    "proxy",
		service.CPROutboundProxyUpdatedAtExtraKey: "2026-09-21T00:00:00Z",
	}
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(updates))
	updates[service.CPRPlanTypeExtraKey] = "pro"
	require.True(t, shouldEnqueueSchedulerOutboxForExtraUpdates(updates))
}
