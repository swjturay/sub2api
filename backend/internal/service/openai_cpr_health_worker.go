package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CPRAccountHealthService only reads management state. It never refreshes OAuth
// tokens, sends inference probes, or re-enables a paused/quarantined account.
type CPRAccountHealthService struct {
	limits *RateLimitService
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
	wg     sync.WaitGroup
}

func NewCPRAccountHealthService(limits *RateLimitService) *CPRAccountHealthService {
	ctx, cancel := context.WithCancel(context.Background())
	return &CPRAccountHealthService{limits: limits, ctx: ctx, cancel: cancel}
}

func (s *CPRAccountHealthService) Start() {
	if s == nil || s.limits == nil || s.limits.accountRepo == nil {
		return
	}
	s.once.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				if s.ctx.Err() != nil {
					return
				}
				s.reconcile()
				select {
				case <-s.ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}

func (s *CPRAccountHealthService) Stop() {
	if s != nil {
		s.cancel()
		s.wg.Wait()
	}
}

func (s *CPRAccountHealthService) reconcile() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	accounts, err := s.limits.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	cancel()
	if err != nil {
		if s.ctx.Err() == nil {
			slog.Warn("cpr_health_scan_failed")
		}
		return
	}
	for i := range accounts {
		if s.ctx.Err() != nil {
			return
		}
		a := &accounts[i]
		if a.IsCPR() && a.IsActive() && a.Schedulable && a.GetCPRAdminAPIKey() != "" && a.GetCPRAccountID() != "" {
			s.limits.checkCPRAccountHealth(s.ctx, a, false)
		}
	}
}

// Resolve the gateway before starting the worker so its runtime blocker exists.
func ProvideCPRAccountHealthService(limits *RateLimitService, _ *OpenAIGatewayService) *CPRAccountHealthService {
	svc := NewCPRAccountHealthService(limits)
	svc.Start()
	return svc
}
