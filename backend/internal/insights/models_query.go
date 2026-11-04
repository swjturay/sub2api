package insights

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type modelTrendAggregate struct {
	performance ModelPerformance
	tokens      int64
}

func modelFilterSQL(ids []ModelIdentity, start int) (string, []any) {
	if len(ids) == 0 {
		return "TRUE", nil
	}
	parts := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)*2)
	for i, id := range ids {
		parts = append(parts, fmt.Sprintf("(lower("+usageProviderSQL("f.platform", "a.platform")+")=lower($%d) AND lower(COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model))=lower($%d))", start+i*2, start+i*2+1))
		args = append(args, id.Platform, id.Name)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// Performance aggregates every usage row, including zero-cost rows. TPOT is
// calculated for each valid request before averaging those request estimates.
func (s *ModelStore) Performance(ctx context.Context, ids []ModelIdentity, from, to time.Time) (map[string]ModelPerformance, error) {
	filter, args := modelFilterSQL(ids, 3)
	query := `SELECT ` + usageProviderSQL("f.platform", "a.platform") + `,COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model),COUNT(*),COALESCE(SUM(u.input_tokens+u.cache_creation_tokens+u.cache_read_tokens+u.output_tokens),0),AVG(u.first_token_ms) FILTER(WHERE u.first_token_ms IS NOT NULL AND u.first_token_ms>=0),COUNT(u.first_token_ms) FILTER(WHERE u.first_token_ms>=0),AVG((u.duration_ms-u.first_token_ms)::float8/(u.output_tokens-1)) FILTER(WHERE u.stream=true AND u.output_tokens>1 AND u.first_token_ms IS NOT NULL AND u.first_token_ms>=0 AND u.duration_ms IS NOT NULL AND u.duration_ms>u.first_token_ms),COUNT(*) FILTER(WHERE u.stream=true AND u.output_tokens>1 AND u.first_token_ms IS NOT NULL AND u.first_token_ms>=0 AND u.duration_ms IS NOT NULL AND u.duration_ms>u.first_token_ms) FROM usage_logs u LEFT JOIN accounts a ON a.id=u.account_id LEFT JOIN LATERAL(SELECT MIN(cf.platform) platform FROM insights_call_facts cf WHERE cf.request_id=u.request_id AND cf.user_id=u.user_id AND cf.api_key_id=u.api_key_id AND cf.model=COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model) AND cf.platform IS NOT NULL HAVING COUNT(DISTINCT cf.call_id)=1)f ON TRUE WHERE u.created_at >= $1 AND u.created_at < $2 AND ` + filter + ` GROUP BY 1,2`
	allArgs := append([]any{from, to}, args...)
	rows, err := s.db.QueryContext(ctx, query, allArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]ModelPerformance)
	minutes := to.Sub(from).Minutes()
	for rows.Next() {
		var platform, name string
		var requests, tokens, ttftSamples, tpotSamples int64
		var ttft, tpot sql.NullFloat64
		if err = rows.Scan(&platform, &name, &requests, &tokens, &ttft, &ttftSamples, &tpot, &tpotSamples); err != nil {
			return nil, err
		}
		perf := ModelPerformance{TPMSamples: requests, RPMSamples: requests, TTFTSamples: ttftSamples, TPOTSamples: tpotSamples}
		if minutes > 0 {
			tpm, rpm := float64(tokens)/minutes, float64(requests)/minutes
			perf.AverageTPM, perf.AverageRPM = &tpm, &rpm
		}
		if ttft.Valid {
			perf.AverageTTFTMS = &ttft.Float64
		}
		if tpot.Valid {
			perf.EstimatedTPOTMS = &tpot.Float64
		}
		out[NormalizeModelKey(ModelIdentity{Platform: platform, Name: name})] = perf
	}
	return out, rows.Err()
}

func (s *ModelStore) Trend(ctx context.Context, id ModelIdentity, from, to time.Time, step string) ([]ModelTrendPoint, error) {
	all, err := s.Trends(ctx, []ModelIdentity{id}, from, to, step)
	return all[NormalizeModelKey(id)], err
}

// Trends aggregates every requested model in one scan. The catalog endpoint
// previously executed one full trend query per model, making its latency grow
// linearly with catalog size.
func (s *ModelStore) Trends(ctx context.Context, ids []ModelIdentity, from, to time.Time, step string) (map[string][]ModelTrendPoint, error) {
	out := make(map[string][]ModelTrendPoint, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	bucket, nominalMinutes := "hour", 60.0
	if step == "day" {
		bucket, nominalMinutes = "day", 1440
	}
	filter, filterArgs := modelFilterSQL(ids, 4)
	query := fmt.Sprintf(`SELECT date_trunc('%s',u.created_at AT TIME ZONE $3) AT TIME ZONE $3,
		`+usageProviderSQL("f.platform", "a.platform")+`,COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model),COUNT(*),COALESCE(SUM(u.input_tokens+u.cache_creation_tokens+u.cache_read_tokens+u.output_tokens),0),
		AVG(u.first_token_ms) FILTER(WHERE u.first_token_ms IS NOT NULL AND u.first_token_ms>=0),COUNT(u.first_token_ms) FILTER(WHERE u.first_token_ms>=0),
		AVG((u.duration_ms-u.first_token_ms)::float8/(u.output_tokens-1)) FILTER(WHERE u.stream=true AND u.output_tokens>1 AND u.first_token_ms IS NOT NULL AND u.first_token_ms>=0 AND u.duration_ms IS NOT NULL AND u.duration_ms>u.first_token_ms),
		COUNT(*) FILTER(WHERE u.stream=true AND u.output_tokens>1 AND u.first_token_ms IS NOT NULL AND u.first_token_ms>=0 AND u.duration_ms IS NOT NULL AND u.duration_ms>u.first_token_ms)
		FROM usage_logs u LEFT JOIN accounts a ON a.id=u.account_id LEFT JOIN LATERAL(SELECT MIN(cf.platform) platform FROM insights_call_facts cf WHERE cf.request_id=u.request_id AND cf.user_id=u.user_id AND cf.api_key_id=u.api_key_id AND cf.model=COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model) AND cf.platform IS NOT NULL HAVING COUNT(DISTINCT cf.call_id)=1)f ON TRUE
		WHERE u.created_at >= $1 AND u.created_at < $2 AND %s GROUP BY 1,2,3 ORDER BY 1`, bucket, filter)
	args := append([]any{from, to, s.loc.String()}, filterArgs...)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	stats := make(map[string]map[int64]modelTrendAggregate, len(ids))
	for rows.Next() {
		var at time.Time
		var platform, name string
		var requests, tokens, ttftSamples, tpotSamples int64
		var ttft, tpot sql.NullFloat64
		if err = rows.Scan(&at, &platform, &name, &requests, &tokens, &ttft, &ttftSamples, &tpot, &tpotSamples); err != nil {
			return nil, err
		}
		perf := ModelPerformance{TPMSamples: requests, RPMSamples: requests, TTFTSamples: ttftSamples, TPOTSamples: tpotSamples}
		if ttft.Valid {
			perf.AverageTTFTMS = &ttft.Float64
		}
		if tpot.Valid {
			perf.EstimatedTPOTMS = &tpot.Float64
		}
		key := NormalizeModelKey(ModelIdentity{Platform: platform, Name: name})
		if stats[key] == nil {
			stats[key] = map[int64]modelTrendAggregate{}
		}
		stats[key][at.UnixNano()] = modelTrendAggregate{performance: perf, tokens: tokens}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	for _, id := range ids {
		key := NormalizeModelKey(id)
		points := make([]ModelTrendPoint, 0)
		for at := truncateModelBucket(from, s.loc, step); at.Before(to); at = nextModelBucket(at, step) {
			next := nextModelBucket(at, step)
			coveredMinutes := minTime(next, to).Sub(maxTime(at, from)).Minutes()
			aggregate := stats[key][at.UnixNano()]
			perf := aggregate.performance
			if coveredMinutes > 0 {
				tpm, rpm := float64(aggregate.tokens)/coveredMinutes, float64(perf.RPMSamples)/coveredMinutes
				perf.AverageTPM, perf.AverageRPM = &tpm, &rpm
			}
			points = append(points, ModelTrendPoint{At: at, Complete: coveredMinutes == nominalMinutes && !next.After(now), Performance: perf})
		}
		out[key] = points
	}
	return out, nil
}

func truncateModelBucket(value time.Time, loc *time.Location, step string) time.Time {
	local := value.In(loc)
	if step == "day" {
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	}
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
}

func nextModelBucket(value time.Time, step string) time.Time {
	if step == "day" {
		return value.AddDate(0, 0, 1)
	}
	return value.Add(time.Hour)
}
