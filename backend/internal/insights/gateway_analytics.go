package insights

import (
	"context"
	"fmt"
	"sort"

	"github.com/lib/pq"
	"time"
)

type GatewayQualityBucket struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Complete bool      `json:"complete"`
	Quality  Quality   `json:"quality"`
}

func (q *Query) GatewayQualityFiltered(ctx context.Context, from, to time.Time, granularity string, departments []string) (Envelope, error) {
	if now := q.now(); to.After(now) {
		to = now
	}

	if granularity != "hour" && granularity != "day" && granularity != "week" && granularity != "month" {
		return Envelope{}, fmt.Errorf("%w: granularity", ErrInvalidFilter)
	}
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	windowStatus := q.coverageWindowStatus(states["call_facts"], from, to)
	if windowStatus != "complete" {
		return q.envelope(map[string]any{"summary": Quality{}, "buckets": []GatewayQualityBucket{}}, []CoverageInfo{{Dataset: "call_results", Status: windowStatus, Detail: "coverage is evaluated through min(requested end, generated_at); collector heartbeat may lag briefly"}}), nil
	}
	binding := departmentBinding{}
	dim := DepartmentDimension{Status: "complete"}
	valid := []string(nil)
	if len(departments) > 0 {
		binding, dim, err = q.resolveDepartmentBinding(ctx)
		if err != nil {
			return Envelope{}, err
		}
		if dim.Status != "configured" {
			return q.envelope(map[string]any{"summary": Quality{}, "buckets": []GatewayQualityBucket{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
		}
		valid = valuesOf(binding.Options)
	}
	predicate := `($4::text[] IS NULL OR EXISTS(SELECT 1 FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 WHERE u.id=f.user_id AND u.deleted_at IS NULL AND COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4) AND ($6::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($6))))`
	var total, success, failed int64
	var model, gateway *float64
	err = q.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE outcome=$3),COUNT(*) FILTER(WHERE outcome<>$3),AVG(model_duration_ms::float8) FILTER(WHERE outcome=$3),AVG(gateway_pre_forward_ms::float8) FILTER(WHERE outcome=$3) FROM insights_call_facts f WHERE statistical_at >= $1 AND statistical_at < $2 AND `+predicate, from, to, OutcomeSuccess, nullableTextArray(departments), binding.ID, nullableTextArray(valid)).Scan(&total, &success, &failed, &model, &gateway)
	if err != nil {
		return Envelope{}, err
	}
	summary := Quality{Total: &total, Success: &success, Failed: &failed, SuccessRate: Ratio(success, total), ModelDurationMS: model, GatewayDurationMS: gateway}
	rows, err := q.db.QueryContext(ctx, `SELECT date_trunc($7,statistical_at AT TIME ZONE $8) AT TIME ZONE $8,COUNT(*),COUNT(*) FILTER(WHERE outcome=$3),COUNT(*) FILTER(WHERE outcome<>$3),AVG(model_duration_ms::float8) FILTER(WHERE outcome=$3),AVG(gateway_pre_forward_ms::float8) FILTER(WHERE outcome=$3) FROM insights_call_facts f WHERE statistical_at >= $1 AND statistical_at < $2 AND `+predicate+` GROUP BY 1 ORDER BY 1`, from, to, OutcomeSuccess, nullableTextArray(departments), binding.ID, nullableTextArray(valid), granularity, q.timezone)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	observed := map[int64]GatewayQualityBucket{}
	loc := mustLocation(q.timezone)
	for rows.Next() {
		var b GatewayQualityBucket
		var t, successCount, failedCount int64
		if err := rows.Scan(&b.Start, &t, &successCount, &failedCount, &b.Quality.ModelDurationMS, &b.Quality.GatewayDurationMS); err != nil {
			return Envelope{}, err
		}
		b.Start = b.Start.In(loc)
		b.End = bucketEnd(b.Start, granularity)
		b.Complete = !b.End.After(q.now())
		b.Quality.Total = &t
		b.Quality.Success = &successCount
		b.Quality.Failed = &failedCount
		b.Quality.SuccessRate = Ratio(successCount, t)
		observed[b.Start.UnixNano()] = b
	}
	if err := rows.Err(); err != nil {
		return Envelope{}, err
	}
	buckets := []GatewayQualityBucket{}
	for start := bucketStart(from, granularity, loc); start.Before(to); start = bucketEnd(start, granularity) {
		if b, ok := observed[start.UnixNano()]; ok {
			buckets = append(buckets, b)
		} else {
			zero := int64(0)
			buckets = append(buckets, GatewayQualityBucket{Start: start, End: bucketEnd(start, granularity), Complete: !bucketEnd(start, granularity).After(q.now()), Quality: Quality{Total: &zero, Success: &zero, Failed: &zero}})
		}
	}
	coverage := []CoverageInfo{{Dataset: "call_results", Status: q.coverageWindowStatus(states["call_facts"], from, to)}}
	if len(departments) > 0 {
		coverage = append(coverage, CoverageInfo{Dataset: "department_attribute", Status: "complete"})
	}
	return q.envelope(map[string]any{"summary": summary, "buckets": buckets}, coverage), rows.Err()
}

type PreferenceModel struct {
	Model string   `json:"model"`
	Count int64    `json:"count"`
	Ratio *float64 `json:"ratio"`
}
type DepartmentPreference struct {
	Department string            `json:"department"`
	Label      string            `json:"label"`
	Total      int64             `json:"total"`
	Models     []PreferenceModel `json:"models"`
}

func (q *Query) GatewayModelPreferences(ctx context.Context, from, to time.Time, departments []string) (Envelope, error) {
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	windowStatus := q.coverageWindowStatus(states["call_facts"], from, to)
	if windowStatus != "complete" {
		return q.envelope(map[string]any{"departments": []DepartmentPreference{}}, []CoverageInfo{{Dataset: "call_results", Status: windowStatus}}), nil
	}
	binding, dim, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return Envelope{}, err
	}
	if dim.Status != "configured" {
		return q.envelope(map[string]any{"departments": []DepartmentPreference{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
	}
	valid := valuesOf(binding.Options)
	rows, err := q.db.QueryContext(ctx, `SELECT COALESCE(NULLIF(v.value,''),'__unassigned__'),COALESCE(f.platform,'unknown')||':'||COALESCE(f.model,''),COUNT(*) FROM insights_call_facts f JOIN users u ON u.id=f.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$3 WHERE f.statistical_at >= $1 AND f.statistical_at < $2 AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($5::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($5)) GROUP BY 1,2 ORDER BY 1,3 DESC`, from, to, binding.ID, nullableTextArray(departments), nullableTextArray(valid))
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	byDept := map[string]*DepartmentPreference{}
	for rows.Next() {
		var dept, model string
		var count int64
		if err := rows.Scan(&dept, &model, &count); err != nil {
			return Envelope{}, err
		}
		d := byDept[dept]
		if d == nil {
			d = &DepartmentPreference{Department: dept, Label: departmentLabel(binding, dept), Models: []PreferenceModel{}}
			byDept[dept] = d
		}
		d.Total += count
		d.Models = append(d.Models, PreferenceModel{Model: model, Count: count})
	}
	if err := rows.Err(); err != nil {
		return Envelope{}, err
	}
	out := make([]DepartmentPreference, 0, len(byDept))
	for _, d := range byDept {
		for i := range d.Models {
			d.Models[i].Ratio = Ratio(d.Models[i].Count, d.Total)
		}
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Department < out[j].Department })
	return q.envelope(map[string]any{"departments": out}, []CoverageInfo{{Dataset: "call_results", Status: windowStatus}, {Dataset: "department_attribute", Status: "complete"}}), nil
}

type GatewayUserBucket struct {
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	TotalUsers int64     `json:"total_users"`
	NewUsers   int64     `json:"new_users"`
	Complete   bool      `json:"complete"`
}

func (q *Query) GatewayUsersFiltered(ctx context.Context, from, to time.Time, granularity string, departments []string) (Envelope, error) {
	if now := q.now(); to.After(now) {
		to = now
	}

	if granularity != "hour" && granularity != "day" && granularity != "week" && granularity != "month" {
		return Envelope{}, fmt.Errorf("%w: granularity", ErrInvalidFilter)
	}
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	binding := departmentBinding{}
	dim := DepartmentDimension{Status: "complete"}
	valid := []string(nil)
	if len(departments) > 0 {
		binding, dim, err = q.resolveDepartmentBinding(ctx)
		if err != nil {
			return Envelope{}, err
		}
		if dim.Status != "configured" {
			return q.envelope(map[string]any{"summary": map[string]int64{"end_users": 0, "new_users": 0}, "buckets": []GatewayUserBucket{}, "frequency": map[string]int64{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
		}
		valid = valuesOf(binding.Options)
	}
	rows, err := q.db.QueryContext(ctx, `SELECT u.id,u.created_at FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$2 WHERE u.deleted_at IS NULL AND u.created_at < $1 AND ($3::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($4))`, to, binding.ID, nullableTextArray(departments), nullableTextArray(valid))
	if err != nil {
		return Envelope{}, err
	}
	ids := []int64{}
	created := []time.Time{}
	for rows.Next() {
		var id int64
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			_ = rows.Close()
			return Envelope{}, err
		}
		ids = append(ids, id)
		created = append(created, at)
	}
	if err := rows.Close(); err != nil {
		return Envelope{}, err
	}
	var newUsers int64
	loc := mustLocation(q.timezone)
	bucketMap := map[time.Time]*GatewayUserBucket{}
	for start := bucketStart(from, granularity, loc); start.Before(to); start = bucketEnd(start, granularity) {
		bucketMap[start] = &GatewayUserBucket{Start: start, End: bucketEnd(start, granularity), Complete: !bucketEnd(start, granularity).After(q.now())}
	}
	for _, at := range created {
		if !at.Before(from) {
			newUsers++
		}
		if !at.Before(from) && at.Before(to) {
			if b := bucketMap[bucketStart(at, granularity, loc)]; b != nil {
				b.NewUsers++
			}
		}
	}
	buckets := make([]GatewayUserBucket, 0, len(bucketMap))
	for _, b := range bucketMap {
		for _, at := range created {
			if at.Before(b.End) {
				b.TotalUsers++
			}
		}
		buckets = append(buckets, *b)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Start.Before(buckets[j].Start) })
	freq := Frequency{}
	if q.coverageWindowStatus(states["call_facts"], from, to) != "complete" {
		freq.Unclassified = int64(len(ids))
	} else if len(ids) > 0 {
		countRows, err := q.db.QueryContext(ctx, `SELECT u.id,COUNT(f.id) FROM unnest($3::bigint[]) u(id) LEFT JOIN insights_call_facts f ON f.user_id=u.id AND f.statistical_at >= $1 AND f.statistical_at < $2 GROUP BY u.id`, from, to, pq.Array(ids))
		if err != nil {
			return Envelope{}, err
		}
		for countRows.Next() {
			var id, count int64
			if err := countRows.Scan(&id, &count); err != nil {
				_ = countRows.Close()
				return Envelope{}, err
			}
			switch ClassifyFrequency(count) {
			case "high":
				freq.High++
			case "medium":
				freq.Medium++
			default:
				freq.Low++
			}
		}
		if err := countRows.Close(); err != nil {
			return Envelope{}, err
		}
	}
	coverage := []CoverageInfo{{Dataset: "call_results", Status: q.coverageWindowStatus(states["call_facts"], from, to)}}
	if len(departments) > 0 {
		coverage = append(coverage, CoverageInfo{Dataset: "department_attribute", Status: "complete"})
	}
	return q.envelope(map[string]any{"summary": map[string]int64{"end_users": int64(len(ids)), "new_users": newUsers}, "buckets": buckets, "frequency": map[string]int64{"high": freq.High, "medium": freq.Medium, "low": freq.Low, "unclassified": freq.Unclassified}}, coverage), nil
}
func bucketStart(at time.Time, granularity string, loc *time.Location) time.Time {
	v := at.In(loc)
	switch granularity {
	case "hour":
		return time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), 0, 0, 0, loc)
	case "week":
		d := time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, loc)
		offset := (int(d.Weekday()) + 6) % 7
		return d.AddDate(0, 0, -offset)
	case "month":
		return time.Date(v.Year(), v.Month(), 1, 0, 0, 0, 0, loc)
	default:
		return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, loc)
	}
}

func (q *Query) GatewayRetentionFiltered(ctx context.Context, cutoff time.Time, departments []string) (Envelope, error) {
	binding := departmentBinding{}
	dim := DepartmentDimension{Status: "complete"}
	valid := []string(nil)
	var err error
	if len(departments) > 0 {
		binding, dim, err = q.resolveDepartmentBinding(ctx)
		if err != nil {
			return Envelope{}, err
		}
		if dim.Status != "configured" {
			return q.envelope(map[string]any{}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
		}
		valid = valuesOf(binding.Options)
	}
	var total, first, d1, d7, d30, unknownFirst, unknownReturn int64
	err = q.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE l.first_observed_call_at<$1),COUNT(*) FILTER(WHERE l.returned_day_1_at<$1),COUNT(*) FILTER(WHERE l.returned_day_7_at<$1),COUNT(*) FILTER(WHERE l.returned_day_30_at<$1),COUNT(*) FILTER(WHERE l.first_observed_call_at IS NULL),COUNT(*) FILTER(WHERE l.first_observed_call_at<$1 AND (NOT COALESCE(l.first_call_coverage_complete,false) OR NOT COALESCE(l.return_coverage_complete,false))) FROM users u LEFT JOIN insights_user_lifecycle l ON l.user_id=u.id LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$2 WHERE u.deleted_at IS NULL AND u.created_at<$1 AND ($3::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($4))`, cutoff, binding.ID, nullableTextArray(departments), nullableTextArray(valid)).Scan(&total, &first, &d1, &d7, &d30, &unknownFirst, &unknownReturn)
	if err != nil {
		return Envelope{}, err
	}
	var pending1, pending7, pending30 int64
	err = q.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE l.first_call_at >= $1::timestamptz-INTERVAL '1 day' AND l.first_call_at<$1),COUNT(*) FILTER(WHERE l.first_call_at >= $1::timestamptz-INTERVAL '7 days' AND l.first_call_at<$1),COUNT(*) FILTER(WHERE l.first_call_at >= $1::timestamptz-INTERVAL '30 days' AND l.first_call_at<$1) FROM insights_user_lifecycle l JOIN users u ON u.id=l.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$2 WHERE l.first_call_coverage_complete AND l.return_coverage_complete AND ($3::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($4))`, cutoff, binding.ID, nullableTextArray(departments), nullableTextArray(valid)).Scan(&pending1, &pending7, &pending30)
	if err != nil {
		return Envelope{}, err
	}
	status := func(n, unknown int64) string {
		if unknown == 0 {
			return "observed"
		}
		if n == 0 {
			return "unknown"
		}
		return "partial"
	}
	firstStatus := status(first, unknownFirst)
	returnUnknown := unknownFirst + unknownReturn
	layer := func(n int64, layerStatus string, pending int64) RetentionLayer {
		if layerStatus == "observed" && pending > 0 {
			layerStatus = "pending"
		}
		var count *int64
		var ratio *float64
		if n > 0 || layerStatus == "observed" || layerStatus == "pending" {
			value := n
			count = &value
			ratio = Ratio(n, total)
		}
		return RetentionLayer{Count: count, Ratio: ratio, Status: layerStatus}
	}
	coverageStatus := "complete"
	if unknownFirst > 0 || unknownReturn > 0 {
		coverageStatus = "partial"
	}
	cov := []CoverageInfo{{Dataset: "first_activity", Status: coverageStatus, Detail: "first_request uses durable observed-call evidence; return milestones require a trusted lifetime first-call anchor"}}
	if len(departments) > 0 {
		cov = append(cov, CoverageInfo{Dataset: "department_attribute", Status: "complete"})
	}
	return q.envelope(map[string]any{"total_users": layer(total, "observed", 0), "first_request": layer(first, firstStatus, 0), "next_day": layer(d1, status(d1, returnUnknown), pending1), "day_7": layer(d7, status(d7, returnUnknown), pending7), "day_30": layer(d30, status(d30, returnUnknown), pending30)}, cov), nil
}

func (q *Query) callRollupComplete(ctx context.Context, from, to time.Time) (bool, error) {
	if !from.Before(to) {
		return true, nil
	}
	loc := mustLocation(q.timezone)
	fromDate := from.In(loc).Format("2006-01-02")
	toDate := to.In(loc).Format("2006-01-02")
	var complete bool
	err := q.db.QueryRowContext(ctx, `SELECT COALESCE(BOOL_AND(COALESCE(c.complete,false)),false) FROM generate_series($1::date,$2::date-1,interval '1 day') d LEFT JOIN insights_rollup_coverage c ON c.source='call' AND c.stat_date=d::date`, fromDate, toDate).Scan(&complete)
	return complete, err
}

func (q *Query) GatewayQualityLongTerm(ctx context.Context, from, to, split time.Time, granularity string, departments []string) (Envelope, error) {
	if now := q.now(); to.After(now) {
		to = now
	}

	if granularity == "hour" && from.Before(split) {
		return Envelope{}, fmt.Errorf("%w: hour outside detail retention", ErrOutsideRetention)
	}
	if !from.Before(split) {
		return q.GatewayQualityFiltered(ctx, from, to, granularity, departments)
	}
	oldTo := minTime(to, split)
	dailyOK, err := q.callRollupComplete(ctx, from, oldTo)
	if err != nil {
		return Envelope{}, err
	}
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	rawFrom := maxTime(from, split)
	rawOK := !rawFrom.Before(to) || q.coverageWindowStatus(states["call_facts"], rawFrom, to) == "complete"
	if !dailyOK || !rawOK {
		return q.envelope(map[string]any{"summary": Quality{}, "buckets": []GatewayQualityBucket{}}, []CoverageInfo{{Dataset: "daily_call_results", Status: boolCoverage(dailyOK)}, {Dataset: "call_results", Status: boolCoverage(rawOK)}}), nil
	}
	binding := departmentBinding{}
	valid := []string(nil)
	if len(departments) > 0 {
		var dim DepartmentDimension
		binding, dim, err = q.resolveDepartmentBinding(ctx)
		if err != nil {
			return Envelope{}, err
		}
		if dim.Status != "configured" {
			return q.envelope(map[string]any{"summary": Quality{}, "buckets": []GatewayQualityBucket{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
		}
		valid = valuesOf(binding.Options)
	}
	rows, err := q.db.QueryContext(ctx, `WITH x AS (
SELECT date_trunc($8,d.stat_date::timestamp) AT TIME ZONE $7 bucket,SUM(d.success_count)::bigint success,SUM(d.failure_count)::bigint failed,SUM(d.gateway_model_duration_sum_ms)::float8 model_sum,SUM(d.gateway_model_duration_samples)::bigint model_n,SUM(d.gateway_pre_forward_sum_ms)::float8 gateway_sum,SUM(d.gateway_pre_forward_samples)::bigint gateway_n FROM insights_user_model_daily d WHERE d.stat_date >= $1::date AND d.stat_date < $2::date AND ($4::text[] IS NULL OR EXISTS(SELECT 1 FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 WHERE u.id=d.user_id AND u.deleted_at IS NULL AND COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4) AND ($6::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($6)))) GROUP BY 1
UNION ALL
SELECT date_trunc($8,f.statistical_at AT TIME ZONE $7) AT TIME ZONE $7,COUNT(*) FILTER(WHERE f.outcome=1),COUNT(*) FILTER(WHERE f.outcome<>1),COALESCE(SUM(f.model_duration_ms) FILTER(WHERE f.outcome=1),0)::float8,COUNT(f.model_duration_ms) FILTER(WHERE f.outcome=1),COALESCE(SUM(f.gateway_pre_forward_ms) FILTER(WHERE f.outcome=1),0)::float8,COUNT(f.gateway_pre_forward_ms) FILTER(WHERE f.outcome=1) FROM insights_call_facts f WHERE f.statistical_at >= $2 AND f.statistical_at < $3 AND ($4::text[] IS NULL OR EXISTS(SELECT 1 FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$5 WHERE u.id=f.user_id AND u.deleted_at IS NULL AND COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4) AND ($6::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($6)))) GROUP BY 1)
SELECT bucket,SUM(success),SUM(failed),SUM(model_sum),SUM(model_n),SUM(gateway_sum),SUM(gateway_n) FROM x GROUP BY bucket ORDER BY bucket`, from, oldTo, to, nullableTextArray(departments), binding.ID, nullableTextArray(valid), q.timezone, granularity)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	observed := map[int64]GatewayQualityBucket{}
	var total, success, failed, modelN, gatewayN int64
	var modelSum, gatewaySum float64
	loc := mustLocation(q.timezone)
	for rows.Next() {
		var b GatewayQualityBucket
		var s0, f0, mn, gn int64
		var ms, gs float64
		if err = rows.Scan(&b.Start, &s0, &f0, &ms, &mn, &gs, &gn); err != nil {
			return Envelope{}, err
		}
		b.Start = b.Start.In(loc)
		b.End = bucketEnd(b.Start, granularity)
		b.Complete = !b.End.After(q.now())
		t := s0 + f0
		b.Quality.Total = &t
		b.Quality.Success = &s0
		b.Quality.Failed = &f0
		b.Quality.SuccessRate = Ratio(s0, t)
		if mn > 0 {
			x := ms / float64(mn)
			b.Quality.ModelDurationMS = &x
		}
		if gn > 0 {
			x := gs / float64(gn)
			b.Quality.GatewayDurationMS = &x
		}
		observed[b.Start.UnixNano()] = b
		total += t
		success += s0
		failed += f0
		modelSum += ms
		modelN += mn
		gatewaySum += gs
		gatewayN += gn
	}
	if err = rows.Err(); err != nil {
		return Envelope{}, err
	}
	summary := Quality{Total: &total, Success: &success, Failed: &failed, SuccessRate: Ratio(success, total)}
	if modelN > 0 {
		x := modelSum / float64(modelN)
		summary.ModelDurationMS = &x
	}
	if gatewayN > 0 {
		x := gatewaySum / float64(gatewayN)
		summary.GatewayDurationMS = &x
	}
	buckets := []GatewayQualityBucket{}
	for start := bucketStart(from, granularity, loc); start.Before(to); start = bucketEnd(start, granularity) {
		if b, ok := observed[start.UnixNano()]; ok {
			buckets = append(buckets, b)
		} else {
			z := int64(0)
			buckets = append(buckets, GatewayQualityBucket{Start: start, End: bucketEnd(start, granularity), Complete: !bucketEnd(start, granularity).After(q.now()), Quality: Quality{Total: &z, Success: &z, Failed: &z}})
		}
	}
	cov := []CoverageInfo{{Dataset: "daily_call_results", Status: "complete"}, {Dataset: "call_results", Status: boolCoverage(rawOK)}}
	if len(departments) > 0 {
		cov = append(cov, CoverageInfo{Dataset: "department_attribute", Status: "complete"})
	}
	return q.envelope(map[string]any{"summary": summary, "buckets": buckets}, cov), nil
}

func (q *Query) GatewayModelPreferencesLongTerm(ctx context.Context, from, to, split time.Time, departments []string) (Envelope, error) {
	if !from.Before(split) {
		return q.GatewayModelPreferences(ctx, from, to, departments)
	}
	oldTo := minTime(to, split)
	dailyOK, err := q.callRollupComplete(ctx, from, oldTo)
	if err != nil {
		return Envelope{}, err
	}
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	rawFrom := maxTime(from, split)
	rawOK := !rawFrom.Before(to) || q.coverageWindowStatus(states["call_facts"], rawFrom, to) == "complete"
	if !dailyOK || !rawOK {
		return q.envelope(map[string]any{"departments": []DepartmentPreference{}}, []CoverageInfo{{Dataset: "daily_call_results", Status: boolCoverage(dailyOK)}, {Dataset: "call_results", Status: boolCoverage(rawOK)}}), nil
	}
	binding, dim, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return Envelope{}, err
	}
	if dim.Status != "configured" {
		return q.envelope(map[string]any{"departments": []DepartmentPreference{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
	}
	valid := valuesOf(binding.Options)
	rows, err := q.db.QueryContext(ctx, `WITH x AS (SELECT COALESCE(NULLIF(v.value,''),'__unassigned__') dept,d.platform||':'||d.model model,SUM(d.success_count+d.failure_count)::bigint n FROM insights_user_model_daily d JOIN users u ON u.id=d.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$3 WHERE d.stat_date >= $1::date AND d.stat_date < $2::date AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($5::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($5)) GROUP BY 1,2 UNION ALL SELECT COALESCE(NULLIF(v.value,''),'__unassigned__'),COALESCE(f.platform,'unknown')||':'||COALESCE(f.model,'unknown'),COUNT(*) FROM insights_call_facts f JOIN users u ON u.id=f.user_id AND u.deleted_at IS NULL LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$3 WHERE f.statistical_at >= $2 AND f.statistical_at < $6 AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($4)) AND ($5::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($5)) GROUP BY 1,2) SELECT dept,model,SUM(n) FROM x GROUP BY 1,2 ORDER BY 1,3 DESC`, from, oldTo, binding.ID, nullableTextArray(departments), nullableTextArray(valid), to)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	by := map[string]*DepartmentPreference{}
	for rows.Next() {
		var dept, model string
		var n int64
		if err = rows.Scan(&dept, &model, &n); err != nil {
			return Envelope{}, err
		}
		d := by[dept]
		if d == nil {
			d = &DepartmentPreference{Department: dept, Label: departmentLabel(binding, dept), Models: []PreferenceModel{}}
			by[dept] = d
		}
		d.Total += n
		d.Models = append(d.Models, PreferenceModel{Model: model, Count: n})
	}
	if err = rows.Err(); err != nil {
		return Envelope{}, err
	}
	out := []DepartmentPreference{}
	for _, d := range by {
		for i := range d.Models {
			d.Models[i].Ratio = Ratio(d.Models[i].Count, d.Total)
		}
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Department < out[j].Department })
	return q.envelope(map[string]any{"departments": out}, []CoverageInfo{{Dataset: "daily_call_results", Status: "complete"}, {Dataset: "call_results", Status: boolCoverage(rawOK)}, {Dataset: "department_attribute", Status: "complete"}}), nil
}

func (q *Query) GatewayUsersLongTerm(ctx context.Context, from, to, split time.Time, granularity string, departments []string) (Envelope, error) {
	if now := q.now(); to.After(now) {
		to = now
	}

	if granularity == "hour" && from.Before(split) {
		return Envelope{}, fmt.Errorf("%w: hour outside detail retention", ErrOutsideRetention)
	}
	if !from.Before(split) {
		return q.GatewayUsersFiltered(ctx, from, to, granularity, departments)
	}
	oldTo := minTime(to, split)
	dailyOK, err := q.callRollupComplete(ctx, from, oldTo)
	if err != nil {
		return Envelope{}, err
	}
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	rawFrom := maxTime(from, split)
	rawOK := !rawFrom.Before(to) || q.coverageWindowStatus(states["call_facts"], rawFrom, to) == "complete"
	binding := departmentBinding{}
	valid := []string(nil)
	if len(departments) > 0 {
		var dim DepartmentDimension
		binding, dim, err = q.resolveDepartmentBinding(ctx)
		if err != nil {
			return Envelope{}, err
		}
		if dim.Status != "configured" {
			return q.envelope(map[string]any{"summary": map[string]int64{"end_users": 0, "new_users": 0}, "buckets": []GatewayUserBucket{}, "frequency": map[string]int64{}}, []CoverageInfo{{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}}), nil
		}
		valid = valuesOf(binding.Options)
	}
	rows, err := q.db.QueryContext(ctx, `SELECT u.id,u.created_at FROM users u LEFT JOIN user_attribute_values v ON v.user_id=u.id AND v.attribute_id=$2 WHERE u.deleted_at IS NULL AND u.created_at<$1 AND ($3::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')=ANY($3)) AND ($4::text[] IS NULL OR COALESCE(NULLIF(v.value,''),'__unassigned__')='__unassigned__' OR v.value=ANY($4))`, to, binding.ID, nullableTextArray(departments), nullableTextArray(valid))
	if err != nil {
		return Envelope{}, err
	}
	ids := []int64{}
	created := []time.Time{}
	for rows.Next() {
		var id int64
		var at time.Time
		if err = rows.Scan(&id, &at); err != nil {
			_ = rows.Close()
			return Envelope{}, err
		}
		ids = append(ids, id)
		created = append(created, at)
	}
	if err = rows.Close(); err != nil {
		return Envelope{}, err
	}
	loc := mustLocation(q.timezone)
	bucketMap := map[time.Time]*GatewayUserBucket{}
	for start := bucketStart(from, granularity, loc); start.Before(to); start = bucketEnd(start, granularity) {
		bucketMap[start] = &GatewayUserBucket{Start: start, End: bucketEnd(start, granularity), Complete: !bucketEnd(start, granularity).After(q.now())}
	}
	var newUsers int64
	for _, at := range created {
		if !at.Before(from) {
			newUsers++
		}
		if !at.Before(from) && at.Before(to) {
			if b := bucketMap[bucketStart(at, granularity, loc)]; b != nil {
				b.NewUsers++
			}
		}
	}
	buckets := []GatewayUserBucket{}
	for _, b := range bucketMap {
		for _, at := range created {
			if at.Before(b.End) {
				b.TotalUsers++
			}
		}
		buckets = append(buckets, *b)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Start.Before(buckets[j].Start) })
	freq := Frequency{}
	if !dailyOK || !rawOK {
		freq.Unclassified = int64(len(ids))
	} else if len(ids) > 0 {
		countRows, e := q.db.QueryContext(ctx, `WITH x AS (SELECT d.user_id,SUM(d.success_count+d.failure_count)::bigint n FROM insights_user_model_daily d WHERE d.stat_date >= $1::date AND d.stat_date < $2::date AND d.user_id=ANY($4) GROUP BY d.user_id UNION ALL SELECT f.user_id,COUNT(*) FROM insights_call_facts f WHERE f.statistical_at >= $2 AND f.statistical_at < $3 AND f.user_id=ANY($4) GROUP BY f.user_id), totals AS (SELECT user_id,SUM(n) n FROM x GROUP BY user_id) SELECT u.id,COALESCE(t.n,0) FROM unnest($4::bigint[]) u(id) LEFT JOIN totals t ON t.user_id=u.id`, from, oldTo, to, pq.Array(ids))
		if e != nil {
			return Envelope{}, e
		}
		for countRows.Next() {
			var id, n int64
			if e = countRows.Scan(&id, &n); e != nil {
				_ = countRows.Close()
				return Envelope{}, e
			}
			switch ClassifyFrequency(n) {
			case "high":
				freq.High++
			case "medium":
				freq.Medium++
			default:
				freq.Low++
			}
		}
		if e = countRows.Close(); e != nil {
			return Envelope{}, e
		}
	}
	cov := []CoverageInfo{{Dataset: "daily_call_results", Status: boolCoverage(dailyOK)}, {Dataset: "call_results", Status: boolCoverage(rawOK)}}
	if len(departments) > 0 {
		cov = append(cov, CoverageInfo{Dataset: "department_attribute", Status: "complete"})
	}
	return q.envelope(map[string]any{"summary": map[string]int64{"end_users": int64(len(ids)), "new_users": newUsers}, "buckets": buckets, "frequency": map[string]int64{"high": freq.High, "medium": freq.Medium, "low": freq.Low, "unclassified": freq.Unclassified}}, cov), nil
}
func boolCoverage(ok bool) string {
	if ok {
		return "complete"
	}
	return "partial"
}
