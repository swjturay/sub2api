package insights

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Transport int16

const (
	TransportUnknown Transport = iota
	TransportHTTPSync
	TransportHTTPStream
	TransportWebSocketTurn
)

type Outcome int16

const (
	OutcomeSuccess Outcome = iota + 1
	OutcomeRejected
	OutcomeError
	OutcomeTimeout
	OutcomeCancelled
	OutcomeStreamInterrupted
)

type Clock func() time.Time

type Identity struct {
	CallID, RequestID, ClientRequestID, Platform, Model string
	UserID, APIKeyID                                    *int64
	Transport                                           Transport
}

type CallFact struct {
	Identity
	Outcome                          Outcome
	ErrorType, ErrorSummary          string
	GatewayPreForward, ModelDuration *time.Duration
	FirstToken                       *time.Duration
	OutputTokens                     *int64
	AttemptCount                     int
	StatisticalAt                    time.Time
}

type Sink interface {
	StoreCall(context.Context, CallFact) error
}

type callContextKey struct{}

func WithCall(ctx context.Context, call *Call) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, callContextKey{}, call)
}

func CallFromContext(ctx context.Context) (*Call, bool) {
	if ctx == nil {
		return nil, false
	}
	call, ok := ctx.Value(callContextKey{}).(*Call)
	return call, ok && call != nil
}

type Call struct {
	mu            sync.Mutex
	id            Identity
	clock         Clock
	startedAt     time.Time
	firstSendAt   time.Time
	attemptSendAt time.Time
	attempts      int
	firstToken    *time.Duration
	fact          *CallFact
}

func (c *Call) UpdateIdentity(update Identity) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fact != nil {
		return
	}
	if update.RequestID != "" {
		c.id.RequestID = update.RequestID
	}
	if update.ClientRequestID != "" {
		c.id.ClientRequestID = update.ClientRequestID
	}
	if update.Platform != "" {
		c.id.Platform = update.Platform
	}
	if update.Model != "" {
		c.id.Model = update.Model
	}
	if update.UserID != nil {
		c.id.UserID = update.UserID
	}
	if update.APIKeyID != nil {
		c.id.APIKeyID = update.APIKeyID
	}
	if update.Transport != TransportUnknown {
		c.id.Transport = update.Transport
	}
}

func (c *Call) Finalized() (CallFact, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fact == nil {
		return CallFact{}, false
	}
	return *c.fact, true
}

func (c *Call) AttemptCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attempts
}

func NewCall(id Identity, startedAt time.Time, clock Clock) *Call {
	if clock == nil {
		clock = time.Now
	}
	if startedAt.IsZero() {
		startedAt = clock()
	}
	if strings.TrimSpace(id.CallID) == "" {
		id.CallID = uuid.NewString()
	}
	return &Call{id: id, clock: clock, startedAt: startedAt}
}

func (c *Call) MarkUpstreamSend(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fact != nil {
		return
	}
	if at.IsZero() {
		at = c.clock()
	}
	c.attempts++
	c.attemptSendAt = at
	if c.firstSendAt.IsZero() {
		c.firstSendAt = at
	}
	c.firstToken = nil
}

func (c *Call) MarkFirstToken(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fact != nil || c.attemptSendAt.IsZero() {
		return
	}
	if at.IsZero() {
		at = c.clock()
	}
	d := at.Sub(c.attemptSendAt)
	if d >= 0 {
		c.firstToken = &d
	}
}

func (c *Call) SetFirstTokenDuration(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fact != nil || c.attemptSendAt.IsZero() || d < 0 {
		return
	}
	c.firstToken = &d
}

func (c *Call) FinishSuccess(outputTokens *int64) CallFact {
	return c.finish(OutcomeSuccess, "", "", outputTokens)
}

func (c *Call) FinishFailure(outcome Outcome, errorType, summary string) CallFact {
	if outcome == OutcomeSuccess || outcome < OutcomeRejected || outcome > OutcomeStreamInterrupted {
		outcome = OutcomeError
	}
	return c.finish(outcome, errorType, summary, nil)
}

func (c *Call) finish(outcome Outcome, errorType, summary string, outputTokens *int64) CallFact {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fact != nil {
		return *c.fact
	}
	now := c.clock()
	f := CallFact{Identity: c.id, Outcome: outcome, ErrorType: bounded(errorType, 96), ErrorSummary: bounded(summary, 512), AttemptCount: c.attempts, StatisticalAt: now, OutputTokens: outputTokens}
	if !c.firstSendAt.IsZero() {
		d := c.firstSendAt.Sub(c.startedAt)
		if d >= 0 {
			f.GatewayPreForward = &d
		}
	}
	if outcome == OutcomeSuccess && !c.attemptSendAt.IsZero() {
		d := now.Sub(c.attemptSendAt)
		if d >= 0 {
			f.ModelDuration = &d
		}
		f.FirstToken = c.firstToken
	}
	c.fact = &f
	return f
}

func (c *Call) Persist(ctx context.Context, sink Sink, fact CallFact) error {
	if sink == nil {
		return errors.New("insights sink is nil")
	}
	return sink.StoreCall(ctx, fact)
}

func bounded(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// MarkActualSend records the moment a transport implementation is about to
// hand the request to the network stack. Non-inference requests have no call in
// their context and are unaffected.
func MarkActualSend(ctx context.Context) {
	if call, ok := CallFromContext(ctx); ok {
		call.MarkUpstreamSend(time.Now())
	}
}
