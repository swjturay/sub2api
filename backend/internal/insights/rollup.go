package insights

import "time"

type DailyUsage struct {
	Date                                                            time.Time
	UserID                                                          int64
	Platform, Model                                                 string
	UsageCount, SuccessCount, FailureCount                          int64
	InputTokens, OutputTokens, CacheCreationTokens, CacheReadTokens int64
	UsageDurationSumMS, UsageDurationSamples                        int64
	UsageFirstTokenSumMS, UsageFirstTokenSamples                    int64
	UsageTPOTSumMS                                                  float64
	UsageTPOTSamples                                                int64
	GatewayModelDurationSumMS, GatewayModelDurationSamples          int64
	GatewayPreForwardSumMS, GatewayPreForwardSamples                int64
}

func DayAt(at time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}
	y, m, d := at.In(location).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, location)
}

type Lifecycle struct {
	FirstCallDate, FirstCallAt                        time.Time
	ReturnedDay1At, ReturnedDay7At, ReturnedDay30At   *time.Time
	FirstCallCoverageComplete, ReturnCoverageComplete bool
}

func ApplyLifecycleCall(current Lifecycle, at time.Time, location *time.Location, trustworthyFirst bool) Lifecycle {
	if current.FirstCallAt.IsZero() {
		if !trustworthyFirst {
			return current
		}
		current.FirstCallAt, current.FirstCallDate = at, DayAt(at, location)
		current.FirstCallCoverageComplete = true
		return current
	}
	// A persisted event can arrive after a later event. Move the first-call
	// anchor backwards and re-evaluate the previous anchor as a return event.
	if trustworthyFirst && at.Before(current.FirstCallAt) {
		previous := current.FirstCallAt
		current.FirstCallAt, current.FirstCallDate = at, DayAt(at, location)
		current.ReturnedDay1At, current.ReturnedDay7At, current.ReturnedDay30At = nil, nil, nil
		current = applyLifecycleReturn(current, previous, location)
		return current
	}
	return applyLifecycleReturn(current, at, location)
}

func applyLifecycleReturn(current Lifecycle, at time.Time, location *time.Location) Lifecycle {
	currentDay := DayAt(at, location)
	firstOrdinal := time.Date(current.FirstCallDate.Year(), current.FirstCallDate.Month(), current.FirstCallDate.Day(), 0, 0, 0, 0, time.UTC)
	currentOrdinal := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), 0, 0, 0, 0, time.UTC)
	days := int(currentOrdinal.Sub(firstOrdinal).Hours() / 24)
	if days >= 1 && current.ReturnedDay1At == nil {
		v := at
		current.ReturnedDay1At = &v
	}
	if days >= 7 && current.ReturnedDay7At == nil {
		v := at
		current.ReturnedDay7At = &v
	}
	if days >= 30 && current.ReturnedDay30At == nil {
		v := at
		current.ReturnedDay30At = &v
	}
	return current
}
