package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoverageActivationTracksObservationAndGaps(t *testing.T) {
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	c := (Coverage{}).Activate(start)
	require.Equal(t, CoverageComplete, c.Status)
	require.Equal(t, start, *c.ObservedThrough)
	c = c.MarkGap(start.Add(2*time.Hour), "store unavailable")
	require.Equal(t, CoveragePartial, c.Status)
	restarted := c.Activate(start.Add(4 * time.Hour))
	require.Equal(t, CoverageComplete, restarted.Status)
	require.Equal(t, start.Add(4*time.Hour), *restarted.ObservedThrough)
}

func TestAggregationConfigRequiresRebuildOnTimezoneOrVersionChange(t *testing.T) {
	c := AggregationConfig{Version: 1, Timezone: "Asia/Shanghai"}
	require.False(t, c.Configure(1, "Asia/Shanghai").RebuildRequired)
	require.True(t, c.Configure(1, "UTC").RebuildRequired)
	require.True(t, c.Configure(2, "Asia/Shanghai").RebuildRequired)
}

func TestCoverageMergePreservesGapDiagnostics(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	gap := start.Add(time.Hour)
	later := gap.Add(time.Hour)
	previous := Coverage{Status: CoveragePartial, LastGapAt: &gap, ObservedThrough: &gap, Reason: "one replica lost a fact"}
	stale := Coverage{Status: CoverageComplete, ObservedThrough: &later}
	merged := mergeCoverage(previous, stale)
	require.Equal(t, CoverageComplete, merged.Status)
	require.Equal(t, gap, *merged.LastGapAt)
	activated := Coverage{Status: CoverageComplete, ObservedThrough: &later}
	repaired := mergeCoverage(previous, activated)
	require.Equal(t, CoverageComplete, repaired.Status)
	protected := mergeCoverage(repaired, stale)
	require.Equal(t, later, *protected.ObservedThrough)
	require.Equal(t, gap, *protected.LastGapAt)
	// The diagnostic survives even though it no longer narrows user statistics.
	require.Equal(t, CoverageComplete, protected.Status)
}
