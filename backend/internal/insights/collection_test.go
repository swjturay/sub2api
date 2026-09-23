package insights

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCallContextCarriesSameCollectorAcrossRetries(t *testing.T) {
	call := NewCall(Identity{}, time.Now(), nil)
	got, ok := CallFromContext(WithCall(context.Background(), call))
	require.True(t, ok)
	require.Same(t, call, got)
}

func TestCallMergesRetriesAndUsesFinalSuccessfulAttempt(t *testing.T) {
	base := time.Date(2026, 9, 22, 1, 0, 0, 0, time.UTC)
	now := base
	c := NewCall(Identity{Transport: TransportHTTPStream}, base, func() time.Time { return now })
	c.MarkUpstreamSend(base.Add(time.Second))
	c.MarkFirstToken(base.Add(1500 * time.Millisecond))
	c.MarkUpstreamSend(base.Add(4 * time.Second))
	c.MarkFirstToken(base.Add(4250 * time.Millisecond))
	now = base.Add(7 * time.Second)
	out := int64(10)
	f := c.FinishSuccess(&out)
	require.Equal(t, 2, f.AttemptCount)
	require.Equal(t, time.Second, *f.GatewayPreForward)
	require.Equal(t, 3*time.Second, *f.ModelDuration)
	require.Equal(t, 250*time.Millisecond, *f.FirstToken)
	require.Equal(t, now, f.StatisticalAt)
	require.Equal(t, f, c.FinishFailure(OutcomeError, "late", "ignored"))
}

func TestCallFailureDoesNotInventSuccessfulTiming(t *testing.T) {
	base := time.Date(2026, 9, 22, 1, 0, 0, 0, time.UTC)
	now := base.Add(2 * time.Second)
	c := NewCall(Identity{}, base, func() time.Time { return now })
	c.MarkUpstreamSend(base.Add(time.Second))
	f := c.FinishFailure(OutcomeCancelled, "client_cancelled", " disconnected ")
	require.Nil(t, f.ModelDuration)
	require.Nil(t, f.FirstToken)
	require.Equal(t, time.Second, *f.GatewayPreForward)
}

func TestApplyLifecycleCallMaintainsNestedMilestones(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	first := time.Date(2026, 1, 1, 23, 0, 0, 0, loc)
	l := ApplyLifecycleCall(Lifecycle{}, first, loc, true)
	l = ApplyLifecycleCall(l, first.AddDate(0, 0, 30), loc, true)
	require.NotNil(t, l.ReturnedDay1At)
	require.NotNil(t, l.ReturnedDay7At)
	require.NotNil(t, l.ReturnedDay30At)
}

func TestApplyLifecycleCallDoesNotFabricateUntrustedFirst(t *testing.T) {
	l := ApplyLifecycleCall(Lifecycle{}, time.Now(), time.UTC, false)
	require.True(t, l.FirstCallAt.IsZero())
}

func TestApplyLifecycleCallAcceptsEarlierLateArrival(t *testing.T) {
	later := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	l := ApplyLifecycleCall(Lifecycle{}, later, time.UTC, true)
	earlier := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	l = ApplyLifecycleCall(l, earlier, time.UTC, true)
	require.Equal(t, earlier, l.FirstCallAt)
	require.NotNil(t, l.ReturnedDay1At)
	require.NotNil(t, l.ReturnedDay7At)
	require.Nil(t, l.ReturnedDay30At)
}

func TestWSTurnCollectorsAreIndependentAndRetryMerges(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	now := base
	turn1 := NewCall(Identity{Transport: TransportWebSocketTurn}, base, func() time.Time { return now })
	turn2 := NewCall(Identity{Transport: TransportWebSocketTurn}, base.Add(time.Minute), func() time.Time { return now })
	turn1.MarkUpstreamSend(base.Add(time.Second))
	turn1.MarkUpstreamSend(base.Add(2 * time.Second)) // same logical turn retry
	now = base.Add(3 * time.Second)
	f1 := turn1.FinishSuccess(nil)
	turn2.MarkUpstreamSend(base.Add(time.Minute + time.Second))
	now = base.Add(time.Minute + 2*time.Second)
	f2 := turn2.FinishFailure(OutcomeStreamInterrupted, "connection_closed", "")
	require.NotEqual(t, f1.CallID, f2.CallID)
	require.Equal(t, 2, f1.AttemptCount)
	require.Equal(t, 1, f2.AttemptCount)
	require.Equal(t, OutcomeSuccess, f1.Outcome)
	require.Equal(t, OutcomeStreamInterrupted, f2.Outcome)
}
