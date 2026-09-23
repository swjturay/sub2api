package insights

import "time"

type CoverageStatus string

const (
	CoverageUnknown  CoverageStatus = "unknown"
	CoverageComplete CoverageStatus = "complete"
	CoveragePartial  CoverageStatus = "partial"
)

type Coverage struct {
	Status          CoverageStatus `json:"status"`
	TrustedSince    *time.Time     `json:"trusted_since,omitempty"`
	ObservedThrough *time.Time     `json:"observed_through,omitempty"`
	LastGapAt       *time.Time     `json:"last_gap_at,omitempty"`
	Reason          string         `json:"reason,omitempty"`
}

func (c Coverage) Activate(at time.Time) Coverage {
	if c.Status == CoverageComplete && c.TrustedSince != nil {
		return c
	}
	c.Status = CoverageComplete
	c.TrustedSince = timePtr(at)
	c.ObservedThrough = timePtr(at)
	c.Reason = ""
	return c
}

func (c Coverage) Observe(at time.Time) Coverage {
	if c.Status == CoverageUnknown {
		return c
	}
	if c.ObservedThrough == nil || at.After(*c.ObservedThrough) {
		c.ObservedThrough = timePtr(at)
	}
	return c
}

func (c Coverage) MarkGap(at time.Time, reason string) Coverage {
	c.Status = CoveragePartial
	c.LastGapAt = timePtr(at)
	c.ObservedThrough = timePtr(at)
	c.Reason = bounded(reason, 128)
	return c
}

func (c Coverage) TrustsFirstCall(userCreatedAt, callAt time.Time) bool {
	return c.Status == CoverageComplete && c.TrustedSince != nil && !userCreatedAt.Before(*c.TrustedSince) && !callAt.Before(userCreatedAt)
}

type AggregationConfig struct {
	Version          int    `json:"version"`
	PreviousTimezone string `json:"previous_timezone,omitempty"`
	Timezone         string `json:"timezone"`
	RebuildRequired  bool   `json:"rebuild_required"`
}

func (c AggregationConfig) Configure(version int, timezone string) AggregationConfig {
	if version <= 0 {
		version = 1
	}
	changed := c.Version != version || c.Timezone != timezone
	if c.Timezone != "" && c.Timezone != timezone {
		c.PreviousTimezone = c.Timezone
	}
	c.Version, c.Timezone = version, timezone
	if changed {
		c.RebuildRequired = true
	}
	return c
}

func timePtr(v time.Time) *time.Time { return &v }
