package handler

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenAIExecutionLeaseCancellation(t *testing.T) {
	for _, executing := range []bool{false, true} {
		t.Run(map[bool]string{false: "waiting", true: "executing"}[executing], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			released := make(chan struct{})
			var count atomic.Int32
			lease := newOpenAIExecutionLease(ctx, func() {
				if count.Add(1) == 1 {
					close(released)
				}
			})
			if executing {
				require.True(t, lease.hold())
			}
			cancel()
			if executing {
				select {
				case <-released:
					t.Fatal("execution slot released before teardown")
				case <-time.After(10 * time.Millisecond):
				}
			} else {
				select {
				case <-released:
				case <-time.After(time.Second):
					t.Fatal("waiting slot not released")
				}
			}
			require.False(t, lease.hold(), "cancelled execution cannot start")
			lease.finish()
			lease.finish()
			require.Equal(t, int32(1), count.Load())
		})
	}
}

func TestOpenAIExecutionLeaseCancelStartRace(t *testing.T) {
	for i := 0; i < 300; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		var count atomic.Int32
		lease := newOpenAIExecutionLease(ctx, func() { count.Add(1) })
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); cancel() }()
		if lease.hold() {
			require.Zero(t, count.Load(), "held execution cannot already have released")
		}
		wg.Wait()
		lease.finish()
		require.Eventually(t, func() bool { return count.Load() == 1 }, time.Second, time.Millisecond)
	}
}

func TestOpenAIExecutionLeaseFinishJoinsCancellationRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, unblock, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	lease := newOpenAIExecutionLease(ctx, func() { close(entered); <-unblock })
	cancel()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("release callback did not start")
	}
	go func() { lease.finish(); close(done) }()
	select {
	case <-done:
		t.Fatal("finish returned while release callback was still running")
	case <-time.After(10 * time.Millisecond):
	}
	close(unblock)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("finish did not join completed release")
	}
}
