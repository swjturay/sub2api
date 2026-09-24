package insights

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

var ErrConflict = errors.New("insights metadata edit conflict")
var ErrInvalidFilter = errors.New("invalid insights filter")
var ErrOutsideRetention = errors.New("insights range outside detail retention")

type CoverageInfo struct {
	Dataset string `json:"dataset"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

type Meta struct {
	Timezone            string         `json:"timezone"`
	GeneratedAt         time.Time      `json:"generated_at"`
	StatisticsStartDate string         `json:"statistics_start_date,omitempty"`
	Coverage            []CoverageInfo `json:"coverage"`
}

type Envelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type Tokens struct {
	Input      int64 `json:"input"`
	CacheWrite int64 `json:"cache_write"`
	CacheRead  int64 `json:"cache_read"`
	Output     int64 `json:"output"`
	Total      int64 `json:"total"`
}

func NewTokens(input, cacheWrite, cacheRead, output int64) Tokens {
	return Tokens{Input: input, CacheWrite: cacheWrite, CacheRead: cacheRead, Output: output, Total: input + cacheWrite + cacheRead + output}
}

func Ratio(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	v := float64(numerator) / float64(denominator)
	return &v
}

func CacheHitRatio(t Tokens) *float64 { return Ratio(t.CacheRead, t.Input+t.CacheWrite+t.CacheRead) }

// usageProviderSQL resolves the provider that actually handled a usage row.
// Composite and Antigravity are routing surfaces, not concrete providers.
func usageProviderSQL(factExpression, accountExpression string) string {
	return fmt.Sprintf(`COALESCE(
 CASE WHEN LOWER(BTRIM(COALESCE(%s,''))) NOT IN ('','antigravity','composite') THEN BTRIM(%s) END,
 CASE WHEN LOWER(BTRIM(COALESCE(%s,''))) NOT IN ('','antigravity','composite') THEN BTRIM(%s) END,
 'unknown')`, factExpression, factExpression, accountExpression, accountExpression)
}

type ModelIdentity struct {
	Platform    string `json:"platform"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
}

func ParseModelIdentity(value string) (ModelIdentity, error) {
	platform, name, ok := strings.Cut(strings.TrimSpace(value), ":")
	platform, name = strings.TrimSpace(platform), strings.TrimSpace(name)
	if !ok || platform == "" || name == "" {
		return ModelIdentity{}, fmt.Errorf("%w: model identity must be platform:name", ErrInvalidFilter)
	}
	return ModelIdentity{Platform: platform, Name: name, DisplayName: name}, nil
}

func EstimatedTPOT(durationMS, firstTokenMS, outputTokens int64) *float64 {
	if outputTokens <= 1 || durationMS <= firstTokenMS || firstTokenMS < 0 {
		return nil
	}
	v := float64(durationMS-firstTokenMS) / float64(outputTokens-1)
	return &v
}

type Query struct {
	db              *sql.DB
	timezone        string
	statisticsStart *time.Time
	now             func() time.Time
}

type Page struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type UsageLogItem struct {
	Metadata     map[string]any `json:"metadata"`
	ID           int64          `json:"id"`
	RecordedAt   time.Time      `json:"recorded_at"`
	Model        string         `json:"model"`
	APIKeyName   string         `json:"api_key_name"`
	Tokens       Tokens         `json:"tokens"`
	DurationMS   *int           `json:"duration_ms"`
	FirstTokenMS *int           `json:"first_token_ms"`
	Department   *string        `json:"department"`
}

type ErrorLogItem struct {
	Metadata   map[string]any `json:"metadata"`
	ID         string         `json:"id"`
	RecordedAt time.Time      `json:"recorded_at"`
	Model      string         `json:"model"`
	ErrorType  string         `json:"error_type"`
	Reason     string         `json:"reason"`
	Department *string        `json:"department"`
}

func (q *Query) currentDepartment(ctx context.Context, userID int64) (*string, CoverageInfo, error) {
	binding, dim, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return nil, CoverageInfo{}, err
	}
	if dim.Status != "configured" {
		return nil, CoverageInfo{Dataset: "department_attribute", Status: dim.Status, Detail: dim.Detail}, nil
	}
	var value sql.NullString
	err = q.db.QueryRowContext(ctx, `SELECT value FROM user_attribute_values WHERE user_id=$1 AND attribute_id=$2`, userID, binding.ID).Scan(&value)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, CoverageInfo{}, err
	}
	label := "未分配"
	if value.Valid && value.String != "" {
		label = value.String
		for _, option := range binding.Options {
			if option.Value == value.String {
				label = option.Label
				break
			}
		}
	}
	return &label, CoverageInfo{Dataset: "department_attribute", Status: "complete"}, nil
}
func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(value string) (time.Time, string, error) {
	if value == "" {
		return time.Time{}, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	return at, parts[1], err
}

func (q *Query) UsageLogs(ctx context.Context, userID int64, from, to time.Time, pageSize int, cursor string, models []string) (Envelope, error) {
	if pageSize != 20 && pageSize != 50 && pageSize != 100 {
		return Envelope{}, fmt.Errorf("%w: page size", ErrInvalidFilter)
	}
	cursorAt, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return Envelope{}, fmt.Errorf("%w: cursor", ErrInvalidFilter)
	}
	id, _ := strconv.ParseInt(cursorID, 10, 64)
	department, departmentCoverage, err := q.currentDepartment(ctx, userID)
	if err != nil {
		return Envelope{}, err
	}
	rows, err := q.db.QueryContext(ctx, `SELECT ul.id, ul.created_at, `+usageProviderSQL("f.platform", "a.platform")+` || ':' || COALESCE(NULLIF(ul.requested_model,''),ul.model), COALESCE(k.name,''), ul.input_tokens, ul.cache_creation_tokens, ul.cache_read_tokens, ul.output_tokens, ul.duration_ms, ul.first_token_ms, ul.stream, COALESCE(ul.request_id,'') FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id LEFT JOIN accounts a ON a.id=ul.account_id LEFT JOIN LATERAL (SELECT MIN(f0.platform) AS platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true WHERE ul.user_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3 AND ($4::timestamptz IS NULL OR (ul.created_at,ul.id)<($4,$5)) AND ($6::text[] IS NULL OR (`+usageProviderSQL("f.platform", "a.platform")+` || ':' || COALESCE(NULLIF(ul.requested_model,''),ul.model))=ANY($6::text[])) ORDER BY ul.created_at DESC,ul.id DESC LIMIT $7`, userID, from, to, nullableTime(cursorAt), id, nullableTextArray(models), pageSize+1)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]UsageLogItem, 0, pageSize+1)
	for rows.Next() {
		var item UsageLogItem
		var input, write, read, output int64
		var duration, first sql.NullInt64
		var stream bool
		var requestID string
		if err := rows.Scan(&item.ID, &item.RecordedAt, &item.Model, &item.APIKeyName, &input, &write, &read, &output, &duration, &first, &stream, &requestID); err != nil {
			return Envelope{}, err
		}
		item.Tokens = NewTokens(input, write, read, output)
		item.Metadata = map[string]any{"record_id": item.ID, "request_id": requestID, "stream": stream}
		item.Department = department
		if duration.Valid {
			v := int(duration.Int64)
			item.DurationMS = &v
		}
		if first.Valid {
			v := int(first.Int64)
			item.FirstTokenMS = &v
		}
		items = append(items, item)
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	page := Page{HasMore: hasMore}
	if hasMore {
		last := items[len(items)-1]
		page.NextCursor = encodeCursor(last.RecordedAt, strconv.FormatInt(last.ID, 10))
	}
	if err := rows.Err(); err != nil {
		return Envelope{}, err
	}
	usageCoverage, err := q.rawCoverageInfo(ctx, "usage", "usage_detail", from, to)
	if err != nil {
		return Envelope{}, err
	}
	return q.envelope(map[string]any{"items": items, "page": page}, []CoverageInfo{usageCoverage, departmentCoverage}), nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func (q *Query) ErrorLogs(ctx context.Context, userID int64, from, to time.Time, pageSize int, cursor string, models []string) (Envelope, error) {
	if pageSize != 20 && pageSize != 50 && pageSize != 100 {
		return Envelope{}, fmt.Errorf("%w: page size", ErrInvalidFilter)
	}
	cursorAt, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return Envelope{}, fmt.Errorf("%w: cursor", ErrInvalidFilter)
	}
	department, departmentCoverage, err := q.currentDepartment(ctx, userID)
	if err != nil {
		return Envelope{}, err
	}
	rows, err := q.db.QueryContext(ctx, `SELECT call_id::text, statistical_at, COALESCE(platform,'unknown') || ':' || COALESCE(model,''), error_type, COALESCE(error_summary,'') FROM insights_error_facts WHERE user_id=$1 AND statistical_at >= $2 AND statistical_at < $3 AND ($4::timestamptz IS NULL OR (statistical_at,call_id::text)<($4,$5)) AND ($6::text[] IS NULL OR (COALESCE(platform,'unknown') || ':' || COALESCE(model,''))=ANY($6::text[])) ORDER BY statistical_at DESC,call_id::text DESC LIMIT $7`, userID, from, to, nullableTime(cursorAt), cursorID, nullableTextArray(models), pageSize+1)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]ErrorLogItem, 0, pageSize+1)
	for rows.Next() {
		var item ErrorLogItem
		if err := rows.Scan(&item.ID, &item.RecordedAt, &item.Model, &item.ErrorType, &item.Reason); err != nil {
			return Envelope{}, err
		}
		item.Department = department
		item.Metadata = map[string]any{"call_id": item.ID}
		items = append(items, item)
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	page := Page{HasMore: hasMore}
	if hasMore {
		last := items[len(items)-1]
		page.NextCursor = encodeCursor(last.RecordedAt, last.ID)
	}
	if err := rows.Err(); err != nil {
		return Envelope{}, err
	}
	errorCoverage, err := q.rawCoverageInfo(ctx, "errors", "error_detail", from, to)
	if err != nil {
		return Envelope{}, err
	}
	return q.envelope(map[string]any{"items": items, "page": page}, []CoverageInfo{errorCoverage, departmentCoverage}), nil
}

func NewQuery(db *sql.DB, timezone string, statisticsStart *time.Time) *Query {
	if timezone == "" {
		timezone = "UTC"
	}
	return &Query{db: db, timezone: timezone, statisticsStart: statisticsStart, now: time.Now}
}

func (q *Query) Envelope(data any, coverage []CoverageInfo) Envelope {
	return q.envelope(data, coverage)
}

func (q *Query) envelope(data any, coverage []CoverageInfo) Envelope {
	start := ""
	if q.statisticsStart != nil {
		start = q.statisticsStart.In(mustLocation(q.timezone)).Format("2006-01-02")
	}
	return Envelope{Data: data, Meta: Meta{Timezone: q.timezone, GeneratedAt: q.now(), StatisticsStartDate: start, Coverage: coverage}}
}

type UsageSummary struct {
	RequestCount       int64    `json:"request_count"`
	Tokens             Tokens   `json:"tokens"`
	ActiveDays         int64    `json:"active_days"`
	DailyAverageTokens *float64 `json:"daily_average_tokens"`
	CacheHitRatio      *float64 `json:"cache_hit_ratio"`
}

type UsageBucket struct {
	Start    time.Time    `json:"start"`
	Complete bool         `json:"complete"`
	Metrics  UsageSummary `json:"metrics"`
}
type UsageModel struct {
	Model        string   `json:"model"`
	RequestCount int64    `json:"request_count"`
	Ratio        *float64 `json:"ratio"`
	Tokens       Tokens   `json:"tokens"`
}
type UsageData struct {
	Summary UsageSummary  `json:"summary"`
	Buckets []UsageBucket `json:"buckets"`
	Models  []UsageModel  `json:"models"`
}

type PersonalTodayUsage struct {
	Tokens     Tokens  `json:"tokens"`
	ActualCost float64 `json:"actual_cost"`
}

func (q *Query) PersonalToday(ctx context.Context, userID int64, from, to time.Time) (PersonalTodayUsage, CoverageInfo, error) {
	var input, write, read, output int64
	var actualCost float64
	err := q.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(input_tokens),0),COALESCE(SUM(cache_creation_tokens),0),COALESCE(SUM(cache_read_tokens),0),COALESCE(SUM(output_tokens),0),COALESCE(SUM(actual_cost),0)::float8 FROM usage_logs WHERE user_id=$1 AND created_at >= $2 AND created_at < $3`, userID, from, to).Scan(&input, &write, &read, &output, &actualCost)
	if err != nil {
		return PersonalTodayUsage{}, CoverageInfo{}, fmt.Errorf("query personal today usage: %w", err)
	}
	coverage, err := q.rawCoverageInfo(ctx, "usage", "usage_detail", from, to)
	if err != nil {
		return PersonalTodayUsage{}, CoverageInfo{}, err
	}
	return PersonalTodayUsage{Tokens: NewTokens(input, write, read, output), ActualCost: actualCost}, coverage, nil
}

func (q *Query) PersonalUsage(ctx context.Context, userID int64, from, to time.Time, granularity string, models []string) (Envelope, error) {
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return Envelope{}, fmt.Errorf("%w: granularity", ErrInvalidFilter)
	}
	filter := `($5::text[] IS NULL OR (` + usageProviderSQL("f.platform", "a.platform") + ` || ':' || COALESCE(NULLIF(ul.requested_model,''),ul.model))=ANY($5::text[]))`
	summaryFilter := `($4::text[] IS NULL OR (` + usageProviderSQL("f.platform", "a.platform") + ` || ':' || COALESCE(NULLIF(ul.requested_model,''),ul.model))=ANY($4::text[]))`
	row := q.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(ul.input_tokens),0), COALESCE(SUM(ul.cache_creation_tokens),0), COALESCE(SUM(ul.cache_read_tokens),0), COALESCE(SUM(ul.output_tokens),0), COUNT(DISTINCT CASE WHEN ul.input_tokens+ul.cache_creation_tokens+ul.cache_read_tokens+ul.output_tokens > 0 THEN (ul.created_at AT TIME ZONE $5)::date END)
		FROM usage_logs ul LEFT JOIN accounts a ON a.id=ul.account_id LEFT JOIN LATERAL (SELECT MIN(f0.platform) AS platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true
		WHERE ul.user_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3 AND `+summaryFilter, userID, from, to, nullableTextArray(models), q.timezone)
	var requests, input, write, read, output, active int64
	if err := row.Scan(&requests, &input, &write, &read, &output, &active); err != nil {
		return Envelope{}, fmt.Errorf("query personal usage: %w", err)
	}
	tokens := NewTokens(input, write, read, output)
	days := int64(math.Ceil(to.Sub(from).Hours() / 24))
	var daily *float64
	if days > 0 {
		v := float64(tokens.Total) / float64(days)
		daily = &v
	}
	summary := UsageSummary{RequestCount: requests, Tokens: tokens, ActiveDays: active, DailyAverageTokens: daily, CacheHitRatio: CacheHitRatio(tokens)}
	rows, err := q.db.QueryContext(ctx, `SELECT date_trunc($6,ul.created_at AT TIME ZONE $4),COUNT(*),COALESCE(SUM(ul.input_tokens),0),COALESCE(SUM(ul.cache_creation_tokens),0),COALESCE(SUM(ul.cache_read_tokens),0),COALESCE(SUM(ul.output_tokens),0),COUNT(DISTINCT CASE WHEN ul.input_tokens+ul.cache_creation_tokens+ul.cache_read_tokens+ul.output_tokens>0 THEN (ul.created_at AT TIME ZONE $4)::date END) FROM usage_logs ul LEFT JOIN accounts a ON a.id=ul.account_id LEFT JOIN LATERAL (SELECT MIN(f0.platform) AS platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true WHERE ul.user_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3 AND `+filter+` GROUP BY 1 ORDER BY 1`, userID, from, to, q.timezone, nullableTextArray(models), granularity)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	buckets := []UsageBucket{}
	for rows.Next() {
		var b UsageBucket
		var input, write, read, output int64
		if err := rows.Scan(&b.Start, &b.Metrics.RequestCount, &input, &write, &read, &output, &b.Metrics.ActiveDays); err != nil {
			return Envelope{}, err
		}
		b.Metrics.Tokens = NewTokens(input, write, read, output)
		b.Metrics.CacheHitRatio = CacheHitRatio(b.Metrics.Tokens)
		b.Complete = !bucketEnd(b.Start, granularity).After(q.now().In(mustLocation(q.timezone)))
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return Envelope{}, err
	}
	modelRows, err := q.db.QueryContext(ctx, `SELECT `+usageProviderSQL("f.platform", "a.platform")+` || ':' || COALESCE(NULLIF(ul.requested_model,''),ul.model),COUNT(*),COALESCE(SUM(ul.input_tokens),0),COALESCE(SUM(ul.cache_creation_tokens),0),COALESCE(SUM(ul.cache_read_tokens),0),COALESCE(SUM(ul.output_tokens),0) FROM usage_logs ul LEFT JOIN accounts a ON a.id=ul.account_id LEFT JOIN LATERAL (SELECT MIN(f0.platform) AS platform FROM insights_call_facts f0 WHERE f0.request_id=ul.request_id AND f0.user_id=ul.user_id AND f0.api_key_id=ul.api_key_id AND f0.model=COALESCE(NULLIF(ul.requested_model,''),ul.model) HAVING COUNT(DISTINCT f0.call_id)=1) f ON true WHERE ul.user_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3 AND `+summaryFilter+` GROUP BY 1 ORDER BY 2 DESC,1`, userID, from, to, nullableTextArray(models))
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = modelRows.Close() }()
	modelItems := []UsageModel{}
	for modelRows.Next() {
		var item UsageModel
		var input, write, read, output int64
		if err := modelRows.Scan(&item.Model, &item.RequestCount, &input, &write, &read, &output); err != nil {
			return Envelope{}, err
		}
		item.Tokens = NewTokens(input, write, read, output)
		item.Ratio = Ratio(item.RequestCount, requests)
		modelItems = append(modelItems, item)
	}
	if err := modelRows.Err(); err != nil {
		return Envelope{}, err
	}
	data := UsageData{Summary: summary, Buckets: buckets, Models: modelItems}
	coverage, err := q.rawCoverageInfo(ctx, "usage", "usage_detail", from, to)
	if err != nil {
		return Envelope{}, err
	}
	return q.envelope(data, []CoverageInfo{coverage}), nil
}

func nullableTextArray(values []string) any {
	if len(values) == 0 {
		return nil
	}
	return pq.Array(values)
}

func (q *Query) PersonalUsageDaily(ctx context.Context, userID int64, from, to time.Time, granularity string, models []string) (Envelope, error) {
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return Envelope{}, fmt.Errorf("%w: granularity", ErrInvalidFilter)
	}
	filter := `($4::text[] IS NULL OR (platform || ':' || model)=ANY($4::text[]))`
	var requests, input, write, read, output, active int64
	err := q.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(usage_count),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(cache_creation_tokens),0),COALESCE(SUM(cache_read_tokens),0),COALESCE(SUM(output_tokens),0),COUNT(DISTINCT stat_date) FILTER (WHERE input_tokens+cache_creation_tokens+cache_read_tokens+output_tokens>0) FROM insights_user_model_daily WHERE user_id=$1 AND stat_date >= $2::date AND stat_date < $3::date AND `+filter, userID, from, to, nullableTextArray(models)).Scan(&requests, &input, &write, &read, &output, &active)
	if err != nil {
		return Envelope{}, err
	}
	tokens := NewTokens(input, write, read, output)
	days := int64(math.Ceil(to.Sub(from).Hours() / 24))
	var daily *float64
	if days > 0 {
		v := float64(tokens.Total) / float64(days)
		daily = &v
	}
	summary := UsageSummary{RequestCount: requests, Tokens: tokens, ActiveDays: active, DailyAverageTokens: daily, CacheHitRatio: CacheHitRatio(tokens)}
	rows, err := q.db.QueryContext(ctx, `SELECT date_trunc($5,stat_date::timestamp),COALESCE(SUM(usage_count),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(cache_creation_tokens),0),COALESCE(SUM(cache_read_tokens),0),COALESCE(SUM(output_tokens),0),COUNT(DISTINCT stat_date) FILTER (WHERE input_tokens+cache_creation_tokens+cache_read_tokens+output_tokens>0) FROM insights_user_model_daily WHERE user_id=$1 AND stat_date >= $2::date AND stat_date < $3::date AND `+filter+` GROUP BY 1 ORDER BY 1`, userID, from, to, nullableTextArray(models), granularity)
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = rows.Close() }()
	buckets := []UsageBucket{}
	for rows.Next() {
		var b UsageBucket
		var i, w, r, o int64
		if err := rows.Scan(&b.Start, &b.Metrics.RequestCount, &i, &w, &r, &o, &b.Metrics.ActiveDays); err != nil {
			return Envelope{}, err
		}
		b.Metrics.Tokens = NewTokens(i, w, r, o)
		b.Metrics.CacheHitRatio = CacheHitRatio(b.Metrics.Tokens)
		b.Complete = true
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return Envelope{}, err
	}
	modelRows, err := q.db.QueryContext(ctx, `SELECT platform || ':' || model,COALESCE(SUM(usage_count),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(cache_creation_tokens),0),COALESCE(SUM(cache_read_tokens),0),COALESCE(SUM(output_tokens),0) FROM insights_user_model_daily WHERE user_id=$1 AND stat_date >= $2::date AND stat_date < $3::date AND `+filter+` GROUP BY 1 ORDER BY 2 DESC,1`, userID, from, to, nullableTextArray(models))
	if err != nil {
		return Envelope{}, err
	}
	defer func() { _ = modelRows.Close() }()
	items := []UsageModel{}
	for modelRows.Next() {
		var item UsageModel
		var i, w, r, o int64
		if err := modelRows.Scan(&item.Model, &item.RequestCount, &i, &w, &r, &o); err != nil {
			return Envelope{}, err
		}
		item.Tokens = NewTokens(i, w, r, o)
		item.Ratio = Ratio(item.RequestCount, requests)
		items = append(items, item)
	}
	if err := modelRows.Err(); err != nil {
		return Envelope{}, err
	}
	coverage, err := q.rollupCoverageInfo(ctx, "usage", "daily_user_model", from, to)
	if err != nil {
		return Envelope{}, err
	}
	return q.envelope(UsageData{Summary: summary, Buckets: buckets, Models: items}, []CoverageInfo{coverage}), nil
}

// PersonalUsageCombined reads closed historical days from the daily rollup and
// the retained/new portion from usage_logs, with no overlap at split.
func (q *Query) PersonalUsageCombined(ctx context.Context, userID int64, from, to, split time.Time, granularity string, models []string) (Envelope, error) {
	if !from.Before(split) {
		return q.PersonalUsage(ctx, userID, from, to, granularity, models)
	}
	if !split.Before(to) {
		return q.PersonalUsageDaily(ctx, userID, from, to, granularity, models)
	}
	oldEnv, err := q.PersonalUsageDaily(ctx, userID, from, split, granularity, models)
	if err != nil {
		return Envelope{}, err
	}
	newEnv, err := q.PersonalUsage(ctx, userID, split, to, granularity, models)
	if err != nil {
		return Envelope{}, err
	}
	oldData, ok := oldEnv.Data.(UsageData)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid historical usage response")
	}
	newData, ok := newEnv.Data.(UsageData)
	if !ok {
		return Envelope{}, fmt.Errorf("invalid current usage response")
	}
	data := mergeUsageData(oldData, newData, from, to, q.now().In(mustLocation(q.timezone)))
	dailyCoverage, err := q.rollupCoverageInfo(ctx, "usage", "daily_user_model", from, split)
	if err != nil {
		return Envelope{}, err
	}
	rawCoverage, err := q.rawCoverageInfo(ctx, "usage", "usage_detail", split, to)
	if err != nil {
		return Envelope{}, err
	}
	return q.envelope(data, []CoverageInfo{dailyCoverage, rawCoverage}), nil
}

func mergeUsageData(a, b UsageData, from, to, now time.Time) UsageData {
	out := UsageData{}
	out.Summary.RequestCount = a.Summary.RequestCount + b.Summary.RequestCount
	out.Summary.ActiveDays = a.Summary.ActiveDays + b.Summary.ActiveDays
	addTokens(&out.Summary.Tokens, a.Summary.Tokens)
	addTokens(&out.Summary.Tokens, b.Summary.Tokens)
	out.Summary.CacheHitRatio = CacheHitRatio(out.Summary.Tokens)
	if days := calendarDays(from, to, now); days > 0 {
		v := float64(out.Summary.Tokens.Total) / float64(days)
		out.Summary.DailyAverageTokens = &v
	}
	bucketMap := map[int64]*UsageBucket{}
	for _, src := range [][]UsageBucket{a.Buckets, b.Buckets} {
		for _, item := range src {
			key := item.Start.UnixNano()
			dst := bucketMap[key]
			if dst == nil {
				copy := item
				dst = &copy
				bucketMap[key] = dst
			} else {
				dst.Metrics.RequestCount += item.Metrics.RequestCount
				dst.Metrics.ActiveDays += item.Metrics.ActiveDays
				addTokens(&dst.Metrics.Tokens, item.Metrics.Tokens)
				dst.Metrics.CacheHitRatio = CacheHitRatio(dst.Metrics.Tokens)
				dst.Complete = dst.Complete && item.Complete
			}
		}
	}
	for _, item := range bucketMap {
		out.Buckets = append(out.Buckets, *item)
	}
	sort.Slice(out.Buckets, func(i, j int) bool { return out.Buckets[i].Start.Before(out.Buckets[j].Start) })
	models := map[string]*UsageModel{}
	for _, src := range [][]UsageModel{a.Models, b.Models} {
		for _, item := range src {
			dst := models[item.Model]
			if dst == nil {
				dst = &UsageModel{Model: item.Model}
				models[item.Model] = dst
			}
			dst.RequestCount += item.RequestCount
			addTokens(&dst.Tokens, item.Tokens)
		}
	}
	for _, item := range models {
		item.Ratio = Ratio(item.RequestCount, out.Summary.RequestCount)
		out.Models = append(out.Models, *item)
	}
	sort.Slice(out.Models, func(i, j int) bool {
		if out.Models[i].RequestCount == out.Models[j].RequestCount {
			return out.Models[i].Model < out.Models[j].Model
		}
		return out.Models[i].RequestCount > out.Models[j].RequestCount
	})
	return out
}

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
func bucketEnd(start time.Time, granularity string) time.Time {
	switch granularity {
	case "hour":
		return start.Add(time.Hour)
	case "week":
		return start.AddDate(0, 0, 7)
	case "month":
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}

type HeatmapDay struct {
	Date        string `json:"date"`
	TotalTokens int64  `json:"total_tokens"`
	State       string `json:"state"`
}

type HeatmapScale struct {
	MaxTokens  int64   `json:"max_tokens"`
	Thresholds []int64 `json:"thresholds"`
}

// Heatmap merges published daily aggregates before detailCutoff with retained raw
// usage at and after it. Every calendar day from the configured statistics start
// is in scope; the absence of a usage row means zero usage.
func (q *Query) Heatmap(ctx context.Context, userID int64, year int, detailCutoff time.Time) (Envelope, error) {
	loc := mustLocation(q.timezone)
	start := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, loc)
	cutoff := time.Date(detailCutoff.In(loc).Year(), detailCutoff.In(loc).Month(), detailCutoff.In(loc).Day(), 0, 0, 0, 0, loc)
	var statisticsStart *time.Time
	if q.statisticsStart != nil {
		startDay := DayAt(q.statisticsStart.In(loc), loc)
		statisticsStart = &startDay
	}
	observed := map[string]int64{}
	readDaily := func(from, to time.Time) error {
		if !from.Before(to) {
			return nil
		}
		rows, err := q.db.QueryContext(ctx, `SELECT stat_date,COALESCE(SUM(input_tokens+cache_creation_tokens+cache_read_tokens+output_tokens),0) FROM insights_user_model_daily WHERE user_id=$1 AND stat_date >= $2::date AND stat_date < $3::date GROUP BY 1`, userID, from, to)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var d time.Time
			var n int64
			if err := rows.Scan(&d, &n); err != nil {
				return err
			}
			observed[d.Format("2006-01-02")] += n
		}
		return rows.Err()
	}
	readRaw := func(from, to time.Time) error {
		if !from.Before(to) {
			return nil
		}
		rows, err := q.db.QueryContext(ctx, `SELECT (created_at AT TIME ZONE $4)::date,COALESCE(SUM(input_tokens+cache_creation_tokens+cache_read_tokens+output_tokens),0) FROM usage_logs WHERE user_id=$1 AND created_at >= $2 AND created_at < $3 GROUP BY 1`, userID, from, to, q.timezone)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var d time.Time
			var n int64
			if err := rows.Scan(&d, &n); err != nil {
				return err
			}
			observed[d.Format("2006-01-02")] += n
		}
		return rows.Err()
	}
	dailyEnd := minTime(end, cutoff)
	if err := readDaily(start, dailyEnd); err != nil {
		return Envelope{}, fmt.Errorf("query daily heatmap: %w", err)
	}
	rawStart := maxTime(start, cutoff)
	if err := readRaw(rawStart, end); err != nil {
		return Envelope{}, fmt.Errorf("query raw heatmap: %w", err)
	}

	// Anchor the scale to the retained calendar window, not the selected year.
	// Selecting an older year must not drop the current year's peak.
	currentYear := q.now().In(loc).Year()
	scaleStart := time.Date(currentYear-2, 1, 1, 0, 0, 0, 0, loc)
	scaleEnd := time.Date(currentYear+1, 1, 1, 0, 0, 0, 0, loc)
	var maxTokens int64
	var dailyMax, rawMax sql.NullInt64
	if scaleStart.Before(cutoff) {
		err := q.db.QueryRowContext(ctx, `SELECT MAX(total) FROM (SELECT stat_date,SUM(input_tokens+cache_creation_tokens+cache_read_tokens+output_tokens) total FROM insights_user_model_daily WHERE user_id=$1 AND stat_date >= $2::date AND stat_date < $3::date GROUP BY stat_date) x`, userID, scaleStart, minTime(scaleEnd, cutoff)).Scan(&dailyMax)
		if err != nil {
			return Envelope{}, err
		}
	}
	if scaleEnd.After(cutoff) {
		err := q.db.QueryRowContext(ctx, `SELECT MAX(total) FROM (SELECT (created_at AT TIME ZONE $4)::date,SUM(input_tokens+cache_creation_tokens+cache_read_tokens+output_tokens) total FROM usage_logs WHERE user_id=$1 AND created_at >= $2 AND created_at < $3 GROUP BY 1) x`, userID, maxTime(scaleStart, cutoff), scaleEnd, q.timezone).Scan(&rawMax)
		if err != nil {
			return Envelope{}, err
		}
	}
	if dailyMax.Valid {
		maxTokens = dailyMax.Int64
	}
	if rawMax.Valid && rawMax.Int64 > maxTokens {
		maxTokens = rawMax.Int64
	}
	thresholds := []int64{0, 0, 0, 0}
	if maxTokens > 0 {
		for i := range thresholds {
			thresholds[i] = (maxTokens*int64(i+1) + 3) / 4
		}
	}

	today := q.now().In(loc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	// Calendar dates before the service launch are outside the statistical scope.
	// From launch onward, a missing row is an explicit zero-use day.
	days := make([]HeatmapDay, 0, 366)
	for date := start; date.Before(end); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		tokens, observedDay := observed[key]
		d := HeatmapDay{
			Date:  key,
			State: classifyHeatmapDay(date, today, statisticsStart, observedDay, tokens),
		}
		if observedDay && (d.State == "value" || d.State == "zero") {
			d.TotalTokens = tokens
		}
		days = append(days, d)
	}
	coverage := []CoverageInfo{{Dataset: "daily_rollups", Status: "complete"}, {Dataset: "usage_detail", Status: "complete"}}
	return q.envelope(map[string]any{"year": year, "days": days, "scale": HeatmapScale{MaxTokens: maxTokens, Thresholds: thresholds}}, coverage), nil
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

type Quality struct {
	Total             *int64   `json:"total"`
	Success           *int64   `json:"success"`
	Failed            *int64   `json:"failed"`
	SuccessRate       *float64 `json:"success_rate"`
	ModelDurationMS   *float64 `json:"model_duration_ms"`
	GatewayDurationMS *float64 `json:"gateway_duration_ms"`
}

func (q *Query) GatewayQuality(ctx context.Context, from, to time.Time) (Envelope, error) {
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	windowStatus := q.coverageWindowStatus(states["call_facts"], from, to)
	if windowStatus != "complete" {
		return q.envelope(map[string]any{"summary": Quality{}, "buckets": []any{}}, []CoverageInfo{{Dataset: "call_results", Status: windowStatus, Detail: "coverage is evaluated through min(requested end, generated_at); collector heartbeat may lag briefly"}}), nil
	}
	row := q.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE outcome=$3), COUNT(*) FILTER (WHERE outcome<>$3), AVG(model_duration_ms) FILTER (WHERE outcome=$3), AVG(gateway_pre_forward_ms) FILTER (WHERE outcome=$3) FROM insights_call_facts WHERE statistical_at >= $1 AND statistical_at < $2`, from, to, OutcomeSuccess)
	var total, success, failed int64
	var model, gateway sql.NullFloat64
	if err := row.Scan(&total, &success, &failed, &model, &gateway); err != nil {
		return Envelope{}, err
	}
	quality := Quality{Total: &total, Success: &success, Failed: &failed, SuccessRate: Ratio(success, total)}
	if model.Valid {
		quality.ModelDurationMS = &model.Float64
	}
	if gateway.Valid {
		quality.GatewayDurationMS = &gateway.Float64
	}
	return q.envelope(map[string]any{"summary": quality, "buckets": []any{}}, []CoverageInfo{{Dataset: "call_results", Status: q.coverageWindowStatus(states["call_facts"], from, to)}}), nil
}

func (q *Query) coverageStates(ctx context.Context) (map[string]Coverage, error) {
	var raw []byte
	err := q.db.QueryRowContext(ctx, `SELECT value FROM insights_settings WHERE key='coverage'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]Coverage{}, nil
	}
	if err != nil {
		return nil, err
	}
	encoded := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("decode insights coverage: %w", err)
	}
	states := make(map[string]Coverage, len(encoded))
	for dataset, value := range encoded {
		var legacy string
		if len(value) > 0 && value[0] == '"' {
			if err := json.Unmarshal(value, &legacy); err != nil {
				return nil, fmt.Errorf("decode insights coverage %s: %w", dataset, err)
			}
			states[dataset] = Coverage{Status: CoverageStatus(legacy)}
			continue
		}
		var coverage Coverage
		if err := json.Unmarshal(value, &coverage); err != nil {
			return nil, fmt.Errorf("decode insights coverage %s: %w", dataset, err)
		}
		states[dataset] = coverage
	}
	return states, nil
}

func classifyHeatmapDay(date, today time.Time, statisticsStart *time.Time, observed bool, tokens int64) string {
	if date.After(today) {
		return "future"
	}
	if statisticsStart != nil && date.Before(*statisticsStart) {
		return "out_of_scope"
	}
	if observed && tokens > 0 {
		return "value"
	}
	return "zero"
}

func (q *Query) rawCoverageInfo(_ context.Context, _ string, dataset string, _, _ time.Time) (CoverageInfo, error) {
	return CoverageInfo{Dataset: dataset, Status: "complete"}, nil
}

func (q *Query) rollupCoverageInfo(_ context.Context, _ string, dataset string, _, _ time.Time) (CoverageInfo, error) {
	return CoverageInfo{Dataset: dataset, Status: "complete"}, nil
}

func (q *Query) coverageWindowStatus(_ Coverage, _, _ time.Time) string {
	return "complete"
}

type Frequency struct{ High, Medium, Low, Unclassified int64 }

func ClassifyFrequency(count int64) string {
	if count > 200 {
		return "high"
	}
	if count >= 20 {
		return "medium"
	}
	return "low"
}

func (q *Query) GatewayUsers(ctx context.Context, from, to time.Time) (Envelope, error) {
	states, err := q.coverageStates(ctx)
	if err != nil {
		return Envelope{}, err
	}
	var total, added int64
	if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2) FROM users WHERE deleted_at IS NULL AND created_at < $2`, from, to).Scan(&total, &added); err != nil {
		return Envelope{}, err
	}
	freq := Frequency{}
	if q.coverageWindowStatus(states["call_facts"], from, to) != "complete" {
		freq.Unclassified = total
	} else {
		rows, err := q.db.QueryContext(ctx, `SELECT u.id,COUNT(f.id) FROM users u LEFT JOIN insights_call_facts f ON f.user_id=u.id AND f.statistical_at >= $1 AND f.statistical_at < $2 WHERE u.deleted_at IS NULL AND u.created_at < $2 GROUP BY u.id`, from, to)
		if err != nil {
			return Envelope{}, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, count int64
			if err := rows.Scan(&id, &count); err != nil {
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
		if err := rows.Err(); err != nil {
			return Envelope{}, err
		}
	}
	status := "complete"
	if q.coverageWindowStatus(states["call_facts"], from, to) != "complete" {
		status = "partial"
	}
	data := map[string]any{"summary": map[string]int64{"end_users": total, "new_users": added}, "frequency": map[string]int64{"high": freq.High, "medium": freq.Medium, "low": freq.Low, "unclassified": freq.Unclassified}, "buckets": []any{}}
	return q.envelope(data, []CoverageInfo{{Dataset: "call_results", Status: status}}), nil
}

type RetentionLayer struct {
	Count  *int64   `json:"count"`
	Ratio  *float64 `json:"ratio"`
	Status string   `json:"status"`
}

func (q *Query) GatewayRetention(ctx context.Context, cutoff time.Time) (Envelope, error) {
	return q.GatewayRetentionFiltered(ctx, cutoff, nil)
}
