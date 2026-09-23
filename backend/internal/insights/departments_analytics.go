package insights

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type DepartmentSummary struct {
	MemberCount          int64    `json:"member_count"`
	ActiveMemberCount    int64    `json:"active_member_count"`
	Tokens               Tokens   `json:"tokens"`
	DailyAverageTokens   *float64 `json:"daily_average_tokens"`
	DailyPerMemberTokens *float64 `json:"daily_per_member_tokens"`
	RequestCount         int64    `json:"request_count"`
	OutputRatio          *float64 `json:"output_ratio"`
	CacheHitRatio        *float64 `json:"cache_hit_ratio"`
}

type DepartmentBucket struct {
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	Complete      bool      `json:"complete"`
	Tokens        Tokens    `json:"tokens"`
	RequestCount  int64     `json:"request_count"`
	OutputRatio   *float64  `json:"output_ratio"`
	CacheHitRatio *float64  `json:"cache_hit_ratio"`
}

type ParetoItem struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	TotalTokens     int64    `json:"total_tokens"`
	Ratio           *float64 `json:"ratio"`
	CumulativeRatio *float64 `json:"cumulative_ratio"`
}

type TopUser struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	TotalTokens int64  `json:"total_tokens"`
}

type departmentAggregate struct {
	bucket                      time.Time
	userID                      int64
	username, department, model string
	requests                    int64
	tokens                      Tokens
	ttftSum, tpotSum            float64
	ttftN, tpotN                int64
}

func (q *Query) Departments(ctx context.Context, from, to time.Time, granularity string, departments, models []string, performanceModels []string) (Envelope, error) {
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return Envelope{}, fmt.Errorf("%w: granularity", ErrInvalidFilter)
	}
	binding, dim, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return Envelope{}, err
	}
	if dim.Status != "configured" {
		return q.envelope(map[string]any{"summary": DepartmentSummary{}, "buckets": []DepartmentBucket{}, "models": []UsageModel{}, "pareto": map[string]any{"mode": "department", "items": []ParetoItem{}, "department_items": []ParetoItem{}, "member_items": []ParetoItem{}}, "top_users": []TopUser{}, "performance": ModelPerformance{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
	}
	validOptions := valuesOf(binding.Options)
	var memberCount int64
	err = q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$1 WHERE u.deleted_at IS NULL AND ($2::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($2)) AND ($3::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($3))`, binding.ID, nullableTextArray(departments), nullableTextArray(validOptions)).Scan(&memberCount)
	if err != nil {
		return Envelope{}, err
	}
	rows, err := q.db.QueryContext(ctx, `SELECT date_trunc($6,ul.created_at AT TIME ZONE $4) AT TIME ZONE $4,u.id,COALESCE(NULLIF(u.username,''),u.email),COALESCE(NULLIF(v.value,''),'__unassigned__'),COALESCE(f.platform,CASE WHEN lower(a.platform) NOT IN ('antigravity','composite') THEN a.platform END,'unknown')||':'||COALESCE(NULLIF(ul.requested_model,''),ul.model),COUNT(*),SUM(ul.input_tokens),SUM(ul.cache_creation_tokens),SUM(ul.cache_read_tokens),SUM(ul.output_tokens),COALESCE(SUM(ul.first_token_ms) FILTER(WHERE ul.first_token_ms>=0),0),COUNT(ul.first_token_ms) FILTER(WHERE ul.first_token_ms>=0),COALESCE(SUM((ul.duration_ms-ul.first_token_ms)::float8/(ul.output_tokens-1)) FILTER(WHERE ul.stream=true AND ul.output_tokens>1 AND ul.first_token_ms>=0 AND ul.duration_ms>ul.first_token_ms),0),COUNT(*) FILTER(WHERE ul.stream=true AND ul.output_tokens>1 AND ul.first_token_ms>=0 AND ul.duration_ms>ul.first_token_ms) FROM usage_logs ul LEFT JOIN accounts a ON a.id=ul.account_id JOIN users u ON u.id=ul.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 LEFT JOIN LATERAL (SELECT MIN(f0.platform) platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true WHERE ul.created_at >= $1 AND ul.created_at < $2 AND ($3::text[] IS NULL OR COALESCE(f.platform,CASE WHEN lower(a.platform) NOT IN ('antigravity','composite') THEN a.platform END,'unknown')||':'||COALESCE(NULLIF(ul.requested_model,''),ul.model)=ANY($3)) AND ($7::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($7)) AND ($8::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($8)) GROUP BY 1,2,3,4,5 ORDER BY 1`, from, to, nullableTextArray(models), q.timezone, binding.ID, granularity, nullableTextArray(departments), nullableTextArray(validOptions))
	if err != nil {
		return Envelope{}, err
	}
	aggs := []departmentAggregate{}
	for rows.Next() {
		var a departmentAggregate
		var i, w, r, o int64
		if err := rows.Scan(&a.bucket, &a.userID, &a.username, &a.department, &a.model, &a.requests, &i, &w, &r, &o, &a.ttftSum, &a.ttftN, &a.tpotSum, &a.tpotN); err != nil {
			_ = rows.Close()
			return Envelope{}, err
		}
		a.tokens = NewTokens(i, w, r, o)
		aggs = append(aggs, a)
	}
	if err := rows.Close(); err != nil {
		return Envelope{}, err
	}
	perf, err := q.departmentPerformance(ctx, from, to, binding, departments, performanceModels)
	if err != nil {
		return Envelope{}, err
	}
	return q.finishDepartments(from, to, granularity, departments, memberCount, aggs, perf, "usage_detail", binding), nil
}

func (q *Query) finishDepartments(from, to time.Time, granularity string, departments []string, memberCount int64, aggs []departmentAggregate, perf ModelPerformance, dataset string, binding departmentBinding) Envelope {
	summary := DepartmentSummary{MemberCount: memberCount}
	active := map[int64]bool{}
	bucketMap := map[time.Time]*DepartmentBucket{}
	modelMap := map[string]*UsageModel{}
	deptTotals := map[string]int64{}
	userTotals := map[int64]*TopUser{}
	for _, a := range aggs {
		summary.RequestCount += a.requests
		addTokens(&summary.Tokens, a.tokens)
		if a.tokens.Total > 0 {
			active[a.userID] = true
		}
		b := bucketMap[a.bucket]
		if b == nil {
			b = &DepartmentBucket{Start: a.bucket, End: bucketEnd(a.bucket, granularity)}
			bucketMap[a.bucket] = b
		}
		b.RequestCount += a.requests
		addTokens(&b.Tokens, a.tokens)
		m := modelMap[a.model]
		if m == nil {
			m = &UsageModel{Model: a.model}
			modelMap[a.model] = m
		}
		m.RequestCount += a.requests
		addTokens(&m.Tokens, a.tokens)
		deptTotals[a.department] += a.tokens.Total
		u := userTotals[a.userID]
		if u == nil {
			u = &TopUser{UserID: a.userID, Username: a.username}
			userTotals[a.userID] = u
		}
		u.TotalTokens += a.tokens.Total
	}
	summary.ActiveMemberCount = int64(len(active))
	days := calendarDays(from, to, q.now().In(mustLocation(q.timezone)))
	if days > 0 {
		v := float64(summary.Tokens.Total) / float64(days)
		summary.DailyAverageTokens = &v
		if memberCount > 0 {
			x := v / float64(memberCount)
			summary.DailyPerMemberTokens = &x
		}
	}
	summary.OutputRatio = Ratio(summary.Tokens.Output, summary.Tokens.Total)
	summary.CacheHitRatio = CacheHitRatio(summary.Tokens)
	buckets := make([]DepartmentBucket, 0, len(bucketMap))
	for _, b := range bucketMap {
		b.Complete = !b.End.After(q.now())
		b.OutputRatio = Ratio(b.Tokens.Output, b.Tokens.Total)
		b.CacheHitRatio = CacheHitRatio(b.Tokens)
		buckets = append(buckets, *b)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Start.Before(buckets[j].Start) })
	modelItems := make([]UsageModel, 0, len(modelMap))
	for _, m := range modelMap {
		m.Ratio = Ratio(m.RequestCount, summary.RequestCount)
		modelItems = append(modelItems, *m)
	}
	sort.Slice(modelItems, func(i, j int) bool { return modelItems[i].RequestCount > modelItems[j].RequestCount })
	mode := "department"
	if len(departments) == 1 {
		mode = "member"
	}
	departmentPareto := make([]ParetoItem, 0, len(deptTotals))
	for id, total := range deptTotals {
		departmentPareto = append(departmentPareto, ParetoItem{ID: id, Label: departmentLabel(binding, id), TotalTokens: total})
	}
	memberPareto := make([]ParetoItem, 0, len(userTotals))
	for id, u := range userTotals {
		memberPareto = append(memberPareto, ParetoItem{ID: fmt.Sprint(id), Label: u.Username, TotalTokens: u.TotalTokens})
	}
	finalizePareto := func(items []ParetoItem) []ParetoItem {
		sort.Slice(items, func(i, j int) bool { return items[i].TotalTokens > items[j].TotalTokens })
		var cumulative int64
		for i := range items {
			cumulative += items[i].TotalTokens
			items[i].Ratio = Ratio(items[i].TotalTokens, summary.Tokens.Total)
			items[i].CumulativeRatio = Ratio(cumulative, summary.Tokens.Total)
		}
		return items
	}
	departmentPareto = finalizePareto(departmentPareto)
	memberPareto = finalizePareto(memberPareto)
	pareto := departmentPareto
	if mode == "member" {
		pareto = memberPareto
	}
	top := make([]TopUser, 0, len(userTotals))
	for _, u := range userTotals {
		top = append(top, *u)
	}
	sort.Slice(top, func(i, j int) bool { return top[i].TotalTokens > top[j].TotalTokens })
	if len(top) > 10 {
		top = top[:10]
	}
	return q.envelope(map[string]any{"summary": summary, "buckets": buckets, "models": modelItems, "pareto": map[string]any{"mode": mode, "items": pareto, "department_items": departmentPareto, "member_items": memberPareto}, "top_users": top, "performance": perf}, []CoverageInfo{{Dataset: dataset, Status: "partial"}, {Dataset: "department_attribute", Status: "complete"}})
}

func addTokens(dst *Tokens, src Tokens) {
	dst.Input += src.Input
	dst.CacheWrite += src.CacheWrite
	dst.CacheRead += src.CacheRead
	dst.Output += src.Output
	dst.Total += src.Total
}
func calendarDays(from, to, now time.Time) int64 {
	if to.After(now) {
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	}
	var n int64
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		n++
	}
	return n
}

func (q *Query) departmentPerformance(ctx context.Context, from, to time.Time, binding departmentBinding, departments, models []string) (ModelPerformance, error) {
	validOptions := valuesOf(binding.Options)
	var requests, tokens, ttftN, tpotN int64
	var ttft, tpot *float64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(ul.input_tokens+ul.cache_creation_tokens+ul.cache_read_tokens+ul.output_tokens),0),AVG(ul.first_token_ms::float8) FILTER(WHERE ul.first_token_ms>=0),COUNT(ul.first_token_ms) FILTER(WHERE ul.first_token_ms>=0),AVG((ul.duration_ms-ul.first_token_ms)::float8/(ul.output_tokens-1)) FILTER(WHERE ul.stream=true AND ul.output_tokens>1 AND ul.first_token_ms>=0 AND ul.duration_ms>ul.first_token_ms),COUNT(*) FILTER(WHERE ul.stream=true AND ul.output_tokens>1 AND ul.first_token_ms>=0 AND ul.duration_ms>ul.first_token_ms) FROM usage_logs ul LEFT JOIN accounts a ON a.id=ul.account_id JOIN users u ON u.id=ul.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$3 LEFT JOIN LATERAL (SELECT MIN(f0.platform) platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true WHERE ul.created_at >= $1 AND ul.created_at < $2 AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($5::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($5)) AND ($6::text[] IS NULL OR COALESCE(f.platform,CASE WHEN lower(a.platform) NOT IN ('antigravity','composite') THEN a.platform END,'unknown')||':'||COALESCE(NULLIF(ul.requested_model,''),ul.model)=ANY($6))`, from, to, binding.ID, nullableTextArray(departments), nullableTextArray(validOptions), nullableTextArray(models)).Scan(&requests, &tokens, &ttft, &ttftN, &tpot, &tpotN)
	if err != nil {
		return ModelPerformance{}, err
	}
	perf := ModelPerformance{TPMSamples: requests, RPMSamples: requests, TTFTSamples: ttftN, TPOTSamples: tpotN}
	minutes := minTime(to, q.now()).Sub(from).Minutes()
	if minutes > 0 {
		a, b := float64(tokens)/minutes, float64(requests)/minutes
		perf.AverageTPM = &a
		perf.AverageRPM = &b
	}
	perf.AverageTTFTMS = ttft
	perf.EstimatedTPOTMS = tpot
	return perf, nil
}

func (q *Query) DepartmentsDaily(ctx context.Context, from, to time.Time, granularity string, departments, models, performanceModels []string) (Envelope, error) {
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return Envelope{}, fmt.Errorf("%w: granularity", ErrInvalidFilter)
	}
	binding, dim, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return Envelope{}, err
	}
	if dim.Status != "configured" {
		return q.envelope(map[string]any{"summary": DepartmentSummary{}, "buckets": []DepartmentBucket{}, "models": []UsageModel{}, "pareto": map[string]any{"mode": "department", "items": []ParetoItem{}, "department_items": []ParetoItem{}, "member_items": []ParetoItem{}}, "top_users": []TopUser{}, "performance": ModelPerformance{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
	}
	valid := valuesOf(binding.Options)
	var memberCount int64
	err = q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$1 WHERE u.deleted_at IS NULL AND ($2::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($2)) AND ($3::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($3))`, binding.ID, nullableTextArray(departments), nullableTextArray(valid)).Scan(&memberCount)
	if err != nil {
		return Envelope{}, err
	}
	loc := mustLocation(q.timezone)
	fromDate, toDate := from.In(loc).Format("2006-01-02"), to.In(loc).Format("2006-01-02")
	rows, err := q.db.QueryContext(ctx, `SELECT date_trunc($6,d.stat_date)::date,u.id,COALESCE(NULLIF(u.username,''),u.email),COALESCE(NULLIF(v.value,''),'__unassigned__'),d.platform||':'||d.model,SUM(d.usage_count),SUM(d.input_tokens),SUM(d.cache_creation_tokens),SUM(d.cache_read_tokens),SUM(d.output_tokens),SUM(d.usage_first_token_sum_ms),SUM(d.usage_first_token_samples),SUM(d.usage_tpot_sum_ms),SUM(d.usage_tpot_samples) FROM insights_user_model_daily d JOIN users u ON u.id=d.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 WHERE d.stat_date >= $1::date AND d.stat_date < $2::date AND ($3::text[] IS NULL OR d.platform||':'||d.model=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($7::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($7)) GROUP BY 1,2,3,4,5 ORDER BY 1`, fromDate, toDate, nullableTextArray(models), nullableTextArray(departments), binding.ID, granularity, nullableTextArray(valid))
	if err != nil {
		return Envelope{}, err
	}
	aggs := []departmentAggregate{}
	for rows.Next() {
		var a departmentAggregate
		var date time.Time
		var i, w, r, o int64
		if err := rows.Scan(&date, &a.userID, &a.username, &a.department, &a.model, &a.requests, &i, &w, &r, &o, &a.ttftSum, &a.ttftN, &a.tpotSum, &a.tpotN); err != nil {
			_ = rows.Close()
			return Envelope{}, err
		}
		a.bucket = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
		a.tokens = NewTokens(i, w, r, o)
		aggs = append(aggs, a)
	}
	if err := rows.Close(); err != nil {
		return Envelope{}, err
	}
	perf, err := q.departmentDailyPerformance(ctx, fromDate, toDate, binding, departments, valid, performanceModels, to.Sub(from).Minutes())
	if err != nil {
		return Envelope{}, err
	}
	return q.finishDepartments(from, to, granularity, departments, memberCount, aggs, perf, "daily_user_model", binding), nil
}
func (q *Query) departmentDailyPerformance(ctx context.Context, fromDate, toDate string, binding departmentBinding, departments, valid, models []string, minutes float64) (ModelPerformance, error) {
	var requests, tokens, ttftSum, ttftN, tpotN int64
	var tpotSum float64
	err := q.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(d.usage_count),0),COALESCE(SUM(d.input_tokens+d.cache_creation_tokens+d.cache_read_tokens+d.output_tokens),0),COALESCE(SUM(d.usage_first_token_sum_ms),0),COALESCE(SUM(d.usage_first_token_samples),0),COALESCE(SUM(d.usage_tpot_sum_ms),0),COALESCE(SUM(d.usage_tpot_samples),0) FROM insights_user_model_daily d JOIN users u ON u.id=d.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$3 WHERE d.stat_date >= $1::date AND d.stat_date < $2::date AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($5::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($5)) AND ($6::text[] IS NULL OR d.platform||':'||d.model=ANY($6))`, fromDate, toDate, binding.ID, nullableTextArray(departments), nullableTextArray(valid), nullableTextArray(models)).Scan(&requests, &tokens, &ttftSum, &ttftN, &tpotSum, &tpotN)
	if err != nil {
		return ModelPerformance{}, err
	}
	perf := ModelPerformance{TPMSamples: requests, RPMSamples: requests, TTFTSamples: ttftN, TPOTSamples: tpotN}
	if minutes > 0 {
		a, b := float64(tokens)/minutes, float64(requests)/minutes
		perf.AverageTPM = &a
		perf.AverageRPM = &b
	}
	if ttftN > 0 {
		v := float64(ttftSum) / float64(ttftN)
		perf.AverageTTFTMS = &v
	}
	if tpotN > 0 {
		v := tpotSum / float64(tpotN)
		perf.EstimatedTPOTMS = &v
	}
	return perf, nil
}

// DepartmentsCombined preserves historical daily aggregates and appends the
// retained raw slice. The split is exclusive, so tokens and requests cannot be
// counted twice.
func (q *Query) DepartmentsCombined(ctx context.Context, from, to, split time.Time, granularity string, departments, models, performanceModels []string) (Envelope, error) {
	if !from.Before(split) {
		return q.Departments(ctx, from, to, granularity, departments, models, performanceModels)
	}
	if !split.Before(to) {
		return q.DepartmentsDaily(ctx, from, to, granularity, departments, models, performanceModels)
	}
	oldEnv, err := q.DepartmentsDaily(ctx, from, split, granularity, departments, models, performanceModels)
	if err != nil {
		return Envelope{}, err
	}
	newEnv, err := q.Departments(ctx, split, to, granularity, departments, models, performanceModels)
	if err != nil {
		return Envelope{}, err
	}
	a, ok := oldEnv.Data.(map[string]any)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department response")
	}
	b, ok := newEnv.Data.(map[string]any)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department response")
	}
	sa, ok := a["summary"].(DepartmentSummary)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department summary")
	}
	sb, ok := b["summary"].(DepartmentSummary)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department summary")
	}
	summary := sa
	summary.MemberCount = sb.MemberCount
	summary.RequestCount += sb.RequestCount
	addTokens(&summary.Tokens, sb.Tokens)
	binding, dim, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return Envelope{}, err
	}
	if dim.Status == "configured" {
		valid := valuesOf(binding.Options)
		var active int64
		err = q.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM (SELECT d.user_id FROM insights_user_model_daily d JOIN users u ON u.id=d.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 WHERE d.stat_date >= $1::date AND d.stat_date < $2::date AND d.input_tokens+d.cache_creation_tokens+d.cache_read_tokens+d.output_tokens>0 AND ($3::text[] IS NULL OR d.platform||':'||d.model=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($6::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($6)) UNION ALL SELECT ul.user_id FROM usage_logs ul LEFT JOIN accounts ac ON ac.id=ul.account_id JOIN users u ON u.id=ul.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 LEFT JOIN LATERAL (SELECT MIN(f0.platform) platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true WHERE ul.created_at >= $2 AND ul.created_at < $7 AND ul.input_tokens+ul.cache_creation_tokens+ul.cache_read_tokens+ul.output_tokens>0 AND ($3::text[] IS NULL OR COALESCE(f.platform,CASE WHEN lower(ac.platform) NOT IN ('antigravity','composite') THEN ac.platform END,'unknown')||':'||COALESCE(NULLIF(ul.requested_model,''),ul.model)=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($6::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($6))) x`, from, split, nullableTextArray(models), nullableTextArray(departments), binding.ID, nullableTextArray(valid), to).Scan(&active)
		if err != nil {
			return Envelope{}, err
		}
		summary.ActiveMemberCount = active
	}
	if days := calendarDays(from, to, q.now().In(mustLocation(q.timezone))); days > 0 {
		v := float64(summary.Tokens.Total) / float64(days)
		summary.DailyAverageTokens = &v
		if summary.MemberCount > 0 {
			x := v / float64(summary.MemberCount)
			summary.DailyPerMemberTokens = &x
		}
	}
	summary.OutputRatio = Ratio(summary.Tokens.Output, summary.Tokens.Total)
	summary.CacheHitRatio = CacheHitRatio(summary.Tokens)
	oldBuckets, ok := a["buckets"].([]DepartmentBucket)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department buckets")
	}
	newBuckets, ok := b["buckets"].([]DepartmentBucket)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department buckets")
	}
	oldModels, ok := a["models"].([]UsageModel)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department models")
	}
	newModels, ok := b["models"].([]UsageModel)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department models")
	}
	oldPareto, ok := a["pareto"].(map[string]any)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department pareto")
	}
	newPareto, ok := b["pareto"].(map[string]any)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department pareto")
	}
	oldTop, ok := a["top_users"].([]TopUser)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department top users")
	}
	newTop, ok := b["top_users"].([]TopUser)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department top users")
	}
	oldPerf, ok := a["performance"].(ModelPerformance)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical department performance")
	}
	newPerf, ok := b["performance"].(ModelPerformance)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current department performance")
	}
	buckets := mergeDepartmentBuckets(oldBuckets, newBuckets)
	modelItems := mergeUsageModels(oldModels, newModels, summary.RequestCount)
	pareto := mergePareto(oldPareto, newPareto, summary.Tokens.Total)
	top := mergeTopUsers(oldTop, newTop)
	perf := mergePerformance(oldPerf, newPerf, split.Sub(from).Minutes(), to.Sub(split).Minutes())
	return q.envelope(map[string]any{"summary": summary, "buckets": buckets, "models": modelItems, "pareto": pareto, "top_users": top, "performance": perf}, []CoverageInfo{{Dataset: "daily_user_model", Status: "partial"}, {Dataset: "usage_detail", Status: "partial"}, {Dataset: "department_attribute", Status: "complete"}}), nil
}
func mergeDepartmentBuckets(a, b []DepartmentBucket) []DepartmentBucket {
	m := map[int64]*DepartmentBucket{}
	for _, src := range [][]DepartmentBucket{a, b} {
		for _, v := range src {
			k := v.Start.UnixNano()
			d := m[k]
			if d == nil {
				x := v
				d = &x
				m[k] = d
			} else {
				d.RequestCount += v.RequestCount
				addTokens(&d.Tokens, v.Tokens)
			}
			d.OutputRatio = Ratio(d.Tokens.Output, d.Tokens.Total)
			d.CacheHitRatio = CacheHitRatio(d.Tokens)
		}
	}
	out := []DepartmentBucket{}
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}
func mergeUsageModels(a, b []UsageModel, total int64) []UsageModel {
	m := map[string]*UsageModel{}
	for _, src := range [][]UsageModel{a, b} {
		for _, v := range src {
			d := m[v.Model]
			if d == nil {
				d = &UsageModel{Model: v.Model}
				m[v.Model] = d
			}
			d.RequestCount += v.RequestCount
			addTokens(&d.Tokens, v.Tokens)
		}
	}
	out := []UsageModel{}
	for _, v := range m {
		v.Ratio = Ratio(v.RequestCount, total)
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestCount > out[j].RequestCount })
	return out
}
func mergePareto(a, b map[string]any, total int64) map[string]any {
	mode, _ := a["mode"].(string)
	merge := func(key string) []ParetoItem {
		left, _ := a[key].([]ParetoItem)
		right, _ := b[key].([]ParetoItem)
		m := map[string]*ParetoItem{}
		for _, src := range [][]ParetoItem{left, right} {
			for _, v := range src {
				d := m[v.ID]
				if d == nil {
					x := v
					x.TotalTokens = 0
					d = &x
					m[v.ID] = d
				}
				d.TotalTokens += v.TotalTokens
			}
		}
		out := []ParetoItem{}
		for _, v := range m {
			out = append(out, *v)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].TotalTokens > out[j].TotalTokens })
		var sum int64
		for i := range out {
			sum += out[i].TotalTokens
			out[i].Ratio = Ratio(out[i].TotalTokens, total)
			out[i].CumulativeRatio = Ratio(sum, total)
		}
		return out
	}
	departments := merge("department_items")
	members := merge("member_items")
	items := departments
	if mode == "member" {
		items = members
	}
	return map[string]any{"mode": mode, "items": items, "department_items": departments, "member_items": members}
}
func mergeTopUsers(a, b []TopUser) []TopUser {
	m := map[int64]*TopUser{}
	for _, src := range [][]TopUser{a, b} {
		for _, v := range src {
			d := m[v.UserID]
			if d == nil {
				x := v
				x.TotalTokens = 0
				d = &x
				m[v.UserID] = d
			}
			d.TotalTokens += v.TotalTokens
		}
	}
	out := []TopUser{}
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalTokens > out[j].TotalTokens })
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
func mergePerformance(a, b ModelPerformance, am, bm float64) ModelPerformance {
	out := ModelPerformance{TPMSamples: a.TPMSamples + b.TPMSamples, RPMSamples: a.RPMSamples + b.RPMSamples, TTFTSamples: a.TTFTSamples + b.TTFTSamples, TPOTSamples: a.TPOTSamples + b.TPOTSamples}
	minutes := am + bm
	if minutes > 0 {
		var tokens, requests float64
		if a.AverageTPM != nil {
			tokens += *a.AverageTPM * am
		}
		if b.AverageTPM != nil {
			tokens += *b.AverageTPM * bm
		}
		if a.AverageRPM != nil {
			requests += *a.AverageRPM * am
		}
		if b.AverageRPM != nil {
			requests += *b.AverageRPM * bm
		}
		x, y := tokens/minutes, requests/minutes
		out.AverageTPM = &x
		out.AverageRPM = &y
	}
	if out.TTFTSamples > 0 {
		var sum float64
		if a.AverageTTFTMS != nil {
			sum += *a.AverageTTFTMS * float64(a.TTFTSamples)
		}
		if b.AverageTTFTMS != nil {
			sum += *b.AverageTTFTMS * float64(b.TTFTSamples)
		}
		v := sum / float64(out.TTFTSamples)
		out.AverageTTFTMS = &v
	}
	if out.TPOTSamples > 0 {
		var sum float64
		if a.EstimatedTPOTMS != nil {
			sum += *a.EstimatedTPOTMS * float64(a.TPOTSamples)
		}
		if b.EstimatedTPOTMS != nil {
			sum += *b.EstimatedTPOTMS * float64(b.TPOTSamples)
		}
		v := sum / float64(out.TPOTSamples)
		out.EstimatedTPOTMS = &v
	}
	return out
}
