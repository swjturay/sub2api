package insights

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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
	bucket, interval, nominalMinutes := "hour", "1 hour", 60.0
	if step == "day" {
		bucket, interval, nominalMinutes = "day", "1 day", 1440
	}
	query := fmt.Sprintf(`WITH buckets AS(
		SELECT at,EXTRACT(EPOCH FROM (LEAST(at+$3::interval,$2::timestamptz)-GREATEST(at,$1::timestamptz)))/60.0 minutes
		FROM generate_series(date_trunc('%s',$1::timestamptz AT TIME ZONE $5) AT TIME ZONE $5,date_trunc('%s',($2::timestamptz-INTERVAL '1 microsecond') AT TIME ZONE $5) AT TIME ZONE $5,$3::interval) at
	),stats AS(
		SELECT date_trunc('%s',u.created_at AT TIME ZONE $5) AT TIME ZONE $5 at,COUNT(*) requests,SUM(u.input_tokens+u.cache_creation_tokens+u.cache_read_tokens+u.output_tokens) tokens,
		AVG(u.first_token_ms) FILTER(WHERE u.first_token_ms IS NOT NULL AND u.first_token_ms>=0) ttft,COUNT(u.first_token_ms) FILTER(WHERE u.first_token_ms>=0) ttft_n,
		AVG((u.duration_ms-u.first_token_ms)::float8/(u.output_tokens-1)) FILTER(WHERE u.stream=true AND u.output_tokens>1 AND u.first_token_ms IS NOT NULL AND u.first_token_ms>=0 AND u.duration_ms IS NOT NULL AND u.duration_ms>u.first_token_ms) tpot,
		COUNT(*) FILTER(WHERE u.stream=true AND u.output_tokens>1 AND u.first_token_ms IS NOT NULL AND u.first_token_ms>=0 AND u.duration_ms IS NOT NULL AND u.duration_ms>u.first_token_ms) tpot_n
		FROM usage_logs u LEFT JOIN accounts a ON a.id=u.account_id LEFT JOIN LATERAL(SELECT MIN(cf.platform) platform FROM insights_call_facts cf WHERE cf.request_id=u.request_id AND cf.user_id=u.user_id AND cf.api_key_id=u.api_key_id AND cf.model=COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model) AND cf.platform IS NOT NULL HAVING COUNT(DISTINCT cf.call_id)=1)f ON TRUE
		WHERE u.created_at >= $1 AND u.created_at < $2 AND lower(`+usageProviderSQL("f.platform", "a.platform")+`)=lower($4) AND lower(COALESCE(NULLIF(BTRIM(u.requested_model),''),u.model))=lower($6) GROUP BY 1
	) SELECT b.at,b.minutes,COALESCE(s.requests,0),COALESCE(s.tokens,0),s.ttft,COALESCE(s.ttft_n,0),s.tpot,COALESCE(s.tpot_n,0) FROM buckets b LEFT JOIN stats s USING(at) ORDER BY b.at`, bucket, bucket, bucket)
	rows, err := s.db.QueryContext(ctx, query, from, to, interval, id.Platform, s.loc.String(), id.Name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]ModelTrendPoint, 0)
	now := time.Now()
	for rows.Next() {
		var at time.Time
		var coveredMinutes float64
		var requests, tokens, ttftSamples, tpotSamples int64
		var ttft, tpot sql.NullFloat64
		if err = rows.Scan(&at, &coveredMinutes, &requests, &tokens, &ttft, &ttftSamples, &tpot, &tpotSamples); err != nil {
			return nil, err
		}
		perf := ModelPerformance{TPMSamples: requests, RPMSamples: requests, TTFTSamples: ttftSamples, TPOTSamples: tpotSamples}
		if coveredMinutes > 0 {
			tpm, rpm := float64(tokens)/coveredMinutes, float64(requests)/coveredMinutes
			perf.AverageTPM, perf.AverageRPM = &tpm, &rpm
		}
		if ttft.Valid {
			perf.AverageTTFTMS = &ttft.Float64
		}
		if tpot.Valid {
			perf.EstimatedTPOTMS = &tpot.Float64
		}
		complete := coveredMinutes == nominalMinutes && !at.Add(time.Duration(nominalMinutes)*time.Minute).After(now)
		out = append(out, ModelTrendPoint{At: at, Complete: complete, Performance: perf})
	}
	return out, rows.Err()
}
