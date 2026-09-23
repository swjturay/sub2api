package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoverageOnlyTrustsNewAccountsInsideContinuousInterval(t *testing.T) {
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	c := (Coverage{}).Activate(start)
	require.False(t, c.TrustsFirstCall(start.Add(-time.Second), start.Add(time.Hour)))
	require.True(t, c.TrustsFirstCall(start.Add(time.Minute), start.Add(time.Hour)))
	c = c.MarkGap(start.Add(2*time.Hour), "store unavailable")
	require.Equal(t, CoveragePartial, c.Status)
	require.False(t, c.TrustsFirstCall(start.Add(time.Minute), start.Add(3*time.Hour)))
	restarted := c.Activate(start.Add(4 * time.Hour))
	require.True(t, restarted.TrustsFirstCall(start.Add(5*time.Hour), start.Add(6*time.Hour)))
	require.False(t, restarted.TrustsFirstCall(start.Add(3*time.Hour), start.Add(6*time.Hour)))
}

func TestAggregationConfigRequiresRebuildOnTimezoneOrVersionChange(t *testing.T) {
	c := AggregationConfig{Version: 1, Timezone: "Asia/Shanghai"}
	require.False(t, c.Configure(1, "Asia/Shanghai").RebuildRequired)
	require.True(t, c.Configure(1, "UTC").RebuildRequired)
	require.True(t, c.Configure(2, "Asia/Shanghai").RebuildRequired)
}

func TestHealthyReplicaCannotEraseFleetGap(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	gap := start.Add(time.Hour)
	later := gap.Add(time.Hour)
	previous := Coverage{Status: CoveragePartial, TrustedSince: &start, LastGapAt: &gap, ObservedThrough: &gap, Reason: "one replica lost a fact"}
	stale := Coverage{Status: CoverageComplete, TrustedSince: &start, ObservedThrough: &later}
	merged := mergeCoverage(previous, stale)
	require.Equal(t, CoveragePartial, merged.Status)
	require.Equal(t, gap, *merged.LastGapAt)
	activated := Coverage{Status: CoverageComplete, TrustedSince: &later, ObservedThrough: &later}
	repaired := mergeCoverage(previous, activated)
	require.Equal(t, CoverageComplete, repaired.Status)
	protected := mergeCoverage(repaired, stale)
	require.Equal(t, later, *protected.TrustedSince)
	// Even an old healthy writer after cutover cannot re-certify the old gap.
	require.Equal(t, CoverageComplete, protected.Status)
}
