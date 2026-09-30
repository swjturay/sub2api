package handler

import (
	"context"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const openAIUserExecutionLeaseKey = "openai_user_execution_lease"

// Waiting requests and non-CPR requests retain cancellation release. Once a
// CPR execution starts, its caller owns release until upstream teardown returns.
type openAIExecutionLease struct {
	mu       sync.Mutex
	ctx      context.Context
	held     bool
	released bool
	release  func()
	stop     func() bool
}

func newOpenAIExecutionLease(ctx context.Context, release func()) *openAIExecutionLease {
	if release == nil {
		release = func() {}
	}
	l := &openAIExecutionLease{ctx: ctx, release: sync.OnceFunc(release)}
	l.stop = context.AfterFunc(ctx, func() {
		l.mu.Lock()
		if l.held || l.released {
			l.mu.Unlock()
			return
		}
		l.released = true
		l.mu.Unlock()
		l.release()
	})
	return l
}

func (l *openAIExecutionLease) hold() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released || l.ctx.Err() != nil {
		return false
	}
	l.held = true
	return true
}

func (l *openAIExecutionLease) finish() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		l.release() // Join cancellation release before returning to the handler.
		return
	}
	l.released = true
	l.mu.Unlock()
	l.stop()
	if l.release != nil {
		l.release()
	}
}

func holdCPRUserExecution(c *gin.Context, account *service.Account) bool {
	if !account.IsCPR() {
		return true
	}
	if value, ok := c.Get(openAIUserExecutionLeaseKey); ok {
		if lease, ok := value.(*openAIExecutionLease); ok {
			return lease.hold()
		}
	}
	return c.Request.Context().Err() == nil
}

func wrapOpenAIAccountRelease(ctx context.Context, account *service.Account, release func()) func() {
	if account.IsCPR() {
		if release == nil {
			return nil
		}
		return sync.OnceFunc(release)
	}
	return wrapReleaseOnDone(ctx, release)
}
