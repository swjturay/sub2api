package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecordBestEffortWithoutConfiguredRuntimeReportsGap(t *testing.T) {
	defaultRuntime.Lock()
	old := defaultRuntime.recorder
	defaultRuntime.recorder = nil
	defaultRuntime.Unlock()
	t.Cleanup(func() { defaultRuntime.Lock(); defaultRuntime.recorder = old; defaultRuntime.Unlock() })
	require.False(t, RecordBestEffort(CallFact{StatisticalAt: time.Now()}))
}

func TestUnknownCoverageBecomesPartialOnFirstObservation(t *testing.T) {
	at := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	c := Coverage{Status: CoverageUnknown}
	if c.Status == CoverageUnknown {
		c = Coverage{Status: CoveragePartial, ObservedThrough: &at, Reason: "observed before trusted activation"}
	}
	require.Equal(t, CoveragePartial, c.Status)
	require.Equal(t, at, *c.ObservedThrough)
	require.False(t, c.TrustsFirstCall(at, at.Add(time.Hour)))
}

func TestGapDowngradeSurvivesFullNotificationQueue(t *testing.T) {
	at := time.Now()
	r := &runtimeRecorder{gaps: make(chan string, 1), coverage: Coverage{Status: CoverageComplete, TrustedSince: &at, ObservedThrough: &at}, usageCoverage: Coverage{Status: CoverageComplete, TrustedSince: &at, ObservedThrough: &at}}
	r.gaps <- "previous gap"
	defaultRuntime.Lock()
	old := defaultRuntime.recorder
	defaultRuntime.recorder = r
	defaultRuntime.Unlock()
	t.Cleanup(func() { defaultRuntime.Lock(); defaultRuntime.recorder = old; defaultRuntime.Unlock() })
	ReportGapBestEffort("lost final call")
	require.Equal(t, CoveragePartial, r.coverage.Status)
	require.Equal(t, CoverageComplete, r.usageCoverage.Status)
}

func TestUsageGapAndPendingTasksAreIndependentOfCallCoverage(t *testing.T) {
	at := time.Now()
	r := &runtimeRecorder{gaps: make(chan string, 1), coverage: Coverage{Status: CoverageComplete, TrustedSince: &at, ObservedThrough: &at}, usageCoverage: Coverage{Status: CoverageComplete, TrustedSince: &at, ObservedThrough: &at}}
	defaultRuntime.Lock()
	old := defaultRuntime.recorder
	defaultRuntime.recorder = r
	defaultRuntime.Unlock()
	t.Cleanup(func() { defaultRuntime.Lock(); defaultRuntime.recorder = old; defaultRuntime.Unlock() })
	finish := BeginUsageTask()
	require.Equal(t, 1, r.pendingUsage)
	ReportUsageGapBestEffort("usage write failed")
	require.Equal(t, CoverageComplete, r.coverage.Status)
	require.Equal(t, CoveragePartial, r.usageCoverage.Status)
	finish()
	finish()
	require.Zero(t, r.pendingUsage)
	require.Equal(t, CoveragePartial, r.usageCoverage.Status)
}
