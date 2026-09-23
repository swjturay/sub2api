package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestQueryPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("INSIGHTS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INSIGHTS_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("insights_api_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`) }()
	if _, err = db.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE api_keys(id BIGINT PRIMARY KEY,name TEXT NOT NULL)`,
		`CREATE TABLE accounts(id BIGINT PRIMARY KEY,platform TEXT NOT NULL)`,
		`CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,api_key_id BIGINT NOT NULL,account_id BIGINT NOT NULL,request_id TEXT,stream BOOLEAN NOT NULL DEFAULT false,model TEXT NOT NULL,requested_model TEXT,input_tokens BIGINT NOT NULL,cache_creation_tokens BIGINT NOT NULL,cache_read_tokens BIGINT NOT NULL,output_tokens BIGINT NOT NULL,duration_ms INT,first_token_ms INT,created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE insights_settings(key TEXT PRIMARY KEY,value JSONB NOT NULL)`,
		`CREATE TABLE insights_call_facts(id BIGSERIAL PRIMARY KEY,call_id UUID,request_id TEXT,user_id BIGINT,api_key_id BIGINT,platform TEXT,model TEXT,outcome SMALLINT NOT NULL,model_duration_ms BIGINT,gateway_pre_forward_ms BIGINT,statistical_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE insights_error_facts(call_id UUID PRIMARY KEY,user_id BIGINT NOT NULL,platform TEXT NOT NULL,model TEXT NOT NULL,error_type TEXT NOT NULL,error_summary TEXT,statistical_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT NOT NULL,username TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE insights_user_lifecycle(user_id BIGINT PRIMARY KEY,first_observed_call_at TIMESTAMPTZ,first_call_at TIMESTAMPTZ,returned_day_1_at TIMESTAMPTZ,returned_day_7_at TIMESTAMPTZ,returned_day_30_at TIMESTAMPTZ,first_call_coverage_complete BOOLEAN NOT NULL DEFAULT FALSE,return_coverage_complete BOOLEAN NOT NULL DEFAULT FALSE)`,
		`CREATE TABLE insights_user_model_daily(stat_date DATE NOT NULL,user_id BIGINT NOT NULL,platform TEXT NOT NULL,model TEXT NOT NULL,usage_count BIGINT NOT NULL DEFAULT 0,input_tokens BIGINT NOT NULL DEFAULT 0,cache_creation_tokens BIGINT NOT NULL DEFAULT 0,cache_read_tokens BIGINT NOT NULL DEFAULT 0,output_tokens BIGINT NOT NULL DEFAULT 0,usage_first_token_sum_ms BIGINT NOT NULL DEFAULT 0,usage_first_token_samples BIGINT NOT NULL DEFAULT 0,usage_tpot_sum_ms DOUBLE PRECISION NOT NULL DEFAULT 0,usage_tpot_samples BIGINT NOT NULL DEFAULT 0,success_count BIGINT NOT NULL DEFAULT 0,failure_count BIGINT NOT NULL DEFAULT 0,gateway_model_duration_sum_ms BIGINT NOT NULL DEFAULT 0,gateway_model_duration_samples BIGINT NOT NULL DEFAULT 0,gateway_pre_forward_sum_ms BIGINT NOT NULL DEFAULT 0,gateway_pre_forward_samples BIGINT NOT NULL DEFAULT 0)`,
		`CREATE TABLE insights_rollup_coverage(source TEXT NOT NULL,stat_date DATE NOT NULL,complete BOOLEAN NOT NULL,PRIMARY KEY(source,stat_date))`,
		`CREATE TABLE user_attribute_definitions(id BIGINT PRIMARY KEY,key TEXT NOT NULL,name TEXT NOT NULL,type TEXT NOT NULL,options JSONB NOT NULL,enabled BOOLEAN NOT NULL,deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE user_attribute_values(user_id BIGINT NOT NULL,attribute_id BIGINT NOT NULL,value TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 7)
	if _, err = db.ExecContext(ctx, `INSERT INTO api_keys VALUES(1,'visible-key'); INSERT INTO accounts VALUES(1,'OpenAI')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO users VALUES(7,'u7@example','user7',$1,NULL),(8,'u8@example','user8',$1,NULL),(70,'u70@example','user70',$1,NULL),(71,'u71@example','user71',$1,NULL),(72,'u72@example','user72',$1,NULL),(9,'u9@example','user9',$1,NULL)`, from.AddDate(0, 0, -10)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,duration_ms,first_token_ms,created_at) VALUES (7,1,1,'req-a','upstream','public-model',100,20,30,50,1000,100,$1),(7,1,1,'req-b','free-model',NULL,0,0,0,0,NULL,NULL,$1 + INTERVAL '1 day'),(70,1,1,'req-other','public-model','public-model',999,0,0,0,NULL,NULL,$1)`, from); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(call_id,request_id,user_id,api_key_id,platform,model,outcome,statistical_at) VALUES('00000000-0000-0000-0000-000000000101','req-a',7,1,'OpenAI','public-model',1,$1),('00000000-0000-0000-0000-000000000102','req-b',7,1,'Other','free-model',1,$1)`, from.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	q := NewQuery(db, "UTC")
	q.now = func() time.Time { return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC) }
	usage, err := q.PersonalUsage(ctx, 7, from, to, "day", nil)
	if err != nil {
		t.Fatal(err)
	}
	summary := usage.Data.(UsageData).Summary
	if summary.RequestCount != 2 || summary.Tokens.Total != 200 || summary.ActiveDays != 1 {
		t.Fatalf("usage=%+v", summary)
	}
	filtered, err := q.PersonalUsage(ctx, 7, from, to, "day", []string{"OpenAI:public-model"})
	if err != nil {
		t.Fatal(err)
	}
	filteredSummary := filtered.Data.(UsageData).Summary
	if filteredSummary.RequestCount != 1 || filteredSummary.Tokens.Total != 200 {
		t.Fatalf("filtered=%+v", filteredSummary)
	}
	heatmap, err := q.Heatmap(ctx, 7, 2026, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	heatmapDays := heatmap.Data.(map[string]any)["days"].([]HeatmapDay)
	if len(heatmapDays) != 365 || heatmapDays[0].State != "missing" || heatmapDays[243].State != "value" || heatmapDays[243].TotalTokens != 200 || heatmapDays[245].State != "missing" {
		t.Fatalf("heatmap sample=%+v zero=%+v len=%d", heatmapDays[243], heatmapDays[245], len(heatmapDays))
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_settings VALUES('coverage',jsonb_build_object('usage',jsonb_build_object('status','complete','trusted_since',$1::timestamptz,'observed_through',$2::timestamptz)))`, from, to); err != nil {
		t.Fatal(err)
	}
	coveredHeatmap, e := q.Heatmap(ctx, 7, 2026, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range coveredHeatmap.Data.(map[string]any)["days"].([]HeatmapDay) {
		if d.Date == "2026-09-03" && d.State != "zero" {
			t.Fatalf("verified idle day=%+v", d)
		}
		if d.Date == "2026-08-15" && d.State != "missing" {
			t.Fatalf("retention policy invented coverage=%+v", d)
		}
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_rollup_coverage VALUES('usage','2025-09-23',true)`); err != nil {
		t.Fatal(err)
	}
	prunedHeatmap, e := q.Heatmap(ctx, 7, 2025, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range prunedHeatmap.Data.(map[string]any)["days"].([]HeatmapDay) {
		if d.Date == "2025-09-23" && d.State != "zero" {
			t.Fatalf("durable zero coverage lost=%+v", d)
		}
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM insights_settings WHERE key='coverage'`); err != nil {
		t.Fatal(err)
	}
	scale := heatmap.Data.(map[string]any)["scale"].(HeatmapScale)
	olderHeatmap, e := q.Heatmap(ctx, 7, 2025, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	olderScale := olderHeatmap.Data.(map[string]any)["scale"].(HeatmapScale)
	if olderScale.MaxTokens != scale.MaxTokens || fmt.Sprint(olderScale.Thresholds) != fmt.Sprint(scale.Thresholds) {
		t.Fatalf("year selection changed shared scale: current=%+v old=%+v", scale, olderScale)
	}
	if scale.MaxTokens != 200 || len(scale.Thresholds) != 4 {
		t.Fatalf("scale=%+v", scale)
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	localStart := time.Date(2026, 9, 2, 0, 0, 0, 0, shanghai)
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,created_at) VALUES(8,1,1,'req-before','boundary',1,0,0,0,$1),(8,1,1,'req-after','boundary',2,0,0,0,$2)`, localStart.Add(-time.Minute), localStart.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(call_id,request_id,user_id,api_key_id,platform,model,outcome,statistical_at) VALUES('00000000-0000-0000-0000-000000000103','req-before',8,1,'Boundary','boundary',1,$1),('00000000-0000-0000-0000-000000000104','req-after',8,1,'Boundary','boundary',1,$2)`, localStart.Add(-time.Minute), localStart.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	shanghaiQuery := NewQuery(db, "Asia/Shanghai")
	boundary, err := shanghaiQuery.PersonalUsage(ctx, 8, localStart, localStart.AddDate(0, 0, 1), "day", nil)
	if err != nil {
		t.Fatal(err)
	}
	boundarySummary := boundary.Data.(UsageData).Summary
	if boundarySummary.RequestCount != 1 || boundarySummary.Tokens.Total != 2 {
		t.Fatalf("boundary=%+v", boundarySummary)
	}

	if _, err = db.ExecContext(ctx, `INSERT INTO insights_user_model_daily(stat_date,user_id,platform,model,usage_count,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens) VALUES('2024-01-01',9,'OpenAI','A',2,10,1,2,3),('2024-01-02',9,'Other','B',4,20,2,4,6)`); err != nil {
		t.Fatal(err)
	}
	daily, err := q.PersonalUsageDaily(ctx, 9, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), "week", []string{"OpenAI:A"})
	if err != nil {
		t.Fatal(err)
	}
	dailyData := daily.Data.(UsageData)
	if dailyData.Summary.RequestCount != 2 || dailyData.Summary.Tokens.Total != 16 || len(dailyData.Buckets) != 1 || len(dailyData.Models) != 1 || dailyData.Models[0].Model != "OpenAI:A" {
		t.Fatalf("daily=%+v", dailyData)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_user_model_daily(stat_date,user_id,platform,model,usage_count,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens) VALUES('2026-07-31',7,'OpenAI','public-model',3,30,0,0,0)`); err != nil {
		t.Fatal(err)
	}
	combined, err := q.PersonalUsageCombined(ctx, 7, time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), to, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), "month", nil)
	if err != nil {
		t.Fatal(err)
	}
	combinedData := combined.Data.(UsageData)
	if combinedData.Summary.RequestCount != 5 || combinedData.Summary.Tokens.Total != 230 {
		t.Fatalf("combined=%+v", combinedData.Summary)
	}
	logs, err := q.UsageLogs(ctx, 7, from, to, 20, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	items := logs.Data.(map[string]any)["items"].([]UsageLogItem)
	if len(items) != 2 || items[1].Model != "OpenAI:public-model" || items[1].APIKeyName != "visible-key" {
		t.Fatalf("logs=%+v", items)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,input_tokens,cache_creation_tokens,cache_read_tokens,output_tokens,created_at) VALUES(71,1,1,'shared-id','same','same',1,0,0,0,$1),(71,1,1,'legacy-no-fact','legacy','legacy',1,0,0,0,$1)`, from); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(call_id,request_id,user_id,api_key_id,platform,model,outcome,statistical_at) VALUES('00000000-0000-0000-0000-000000000105','shared-id',71,1,'Own','same',1,$1),('00000000-0000-0000-0000-000000000106','shared-id',72,1,'OtherUser','same',1,$1),('00000000-0000-0000-0000-000000000107','shared-id',71,1,'WrongModel','different',1,$1),('00000000-0000-0000-0000-000000000108','shared-id',71,1,'Duplicate','same',1,$1)`, from); err != nil {
		t.Fatal(err)
	}
	identityLogs, err := q.UsageLogs(ctx, 71, from, to, 20, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	identityItems := identityLogs.Data.(map[string]any)["items"].([]UsageLogItem)
	identityModels := map[string]bool{}
	for _, item := range identityItems {
		identityModels[item.Model] = true
	}
	if len(identityItems) != 2 || !identityModels["OpenAI:same"] || !identityModels["OpenAI:legacy"] {
		t.Fatalf("identity logs=%+v", identityItems)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO user_attribute_definitions VALUES(11,'department','Department','select','[{"value":"A/B","label":"Team AB"}]',true,NULL); INSERT INTO user_attribute_values VALUES(7,11,'A/B'),(9,11,'A/B'); INSERT INTO insights_settings VALUES('department_attribute_id','11')`); err != nil {
		t.Fatal(err)
	}
	logs, err = q.UsageLogs(ctx, 7, from, to, 20, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	items = logs.Data.(map[string]any)["items"].([]UsageLogItem)
	if items[0].Department == nil || *items[0].Department != "Team AB" {
		t.Fatalf("department=%v", items[0].Department)
	}
	if len(items[0].Metadata) != 3 || items[0].Metadata["record_id"] != items[0].ID || items[0].Metadata["request_id"] != "req-b" {
		t.Fatalf("safe log metadata=%+v", items[0].Metadata)
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM insights_settings WHERE key='department_attribute_id'`); err != nil {
		t.Fatal(err)
	}
	autoDepartment, _, err := q.currentDepartment(ctx, 7)
	if err != nil || autoDepartment == nil || *autoDepartment != "Team AB" {
		t.Fatalf("auto-discovered log department=%v err=%v", autoDepartment, err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_settings VALUES('department_attribute_id','11')`); err != nil {
		t.Fatal(err)
	}
	dimension, err := q.DepartmentDimension(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dimension.Status != "configured" || len(dimension.Options) != 2 || dimension.Options[1].Value != "A/B" {
		t.Fatalf("dimension=%+v", dimension)
	}
	departments, err := q.Departments(ctx, from, to, "day", []string{"A/B"}, []string{"OpenAI:public-model"}, []string{"OpenAI:public-model"})
	if err != nil {
		t.Fatal(err)
	}
	departmentSummary := departments.Data.(map[string]any)["summary"].(DepartmentSummary)
	if departmentSummary.MemberCount != 2 || departmentSummary.ActiveMemberCount != 1 || departmentSummary.Tokens.Total != 200 || departmentSummary.RequestCount != 1 {
		t.Fatalf("department summary=%+v", departmentSummary)
	}
	dailyDepartments, err := q.DepartmentsDaily(ctx, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), "week", []string{"A/B"}, []string{"OpenAI:A"}, []string{"Other:B"})
	if err != nil {
		t.Fatal(err)
	}
	dailyDepartmentSummary := dailyDepartments.Data.(map[string]any)["summary"].(DepartmentSummary)
	dailyDepartmentPerformance := dailyDepartments.Data.(map[string]any)["performance"].(ModelPerformance)
	if dailyDepartmentSummary.RequestCount != 2 || dailyDepartmentSummary.Tokens.Total != 16 || dailyDepartmentPerformance.AverageRPM == nil || *dailyDepartmentPerformance.AverageRPM*10080 != 4 {
		t.Fatalf("daily department=%+v perf=%+v", dailyDepartmentSummary, dailyDepartmentPerformance)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_error_facts VALUES('00000000-0000-0000-0000-000000000001',7,'OpenAI','public-model','upstream','visible',$1),('00000000-0000-0000-0000-000000000002',7,'Other','free-model','timeout','filtered',$1),('00000000-0000-0000-0000-000000000003',70,'OpenAI','public-model','secret','other-user',$1)`, from); err != nil {
		t.Fatal(err)
	}
	errorsEnvelope, err := q.ErrorLogs(ctx, 7, from, to, 20, "", []string{"OpenAI:public-model"})
	if err != nil {
		t.Fatal(err)
	}
	errorItems := errorsEnvelope.Data.(map[string]any)["items"].([]ErrorLogItem)
	if len(errorItems) != 1 || errorItems[0].Reason != "visible" || errorItems[0].Department == nil || *errorItems[0].Department != "Team AB" {
		t.Fatalf("errors=%+v", errorItems)
	}

	if _, err = db.ExecContext(ctx, `INSERT INTO insights_settings VALUES('coverage',jsonb_build_object('call_facts',jsonb_build_object('status','complete','trusted_since',$1::timestamptz-interval '1 day','observed_through',$2::timestamptz),'lifecycle','complete'))`, from, to); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(user_id,outcome,model_duration_ms,gateway_pre_forward_ms,statistical_at) VALUES(1,1,900,100,$1),(1,2,NULL,NULL,$1),(7,1,700,70,$1)`, from); err != nil {
		t.Fatal(err)
	}
	quality, err := q.GatewayQuality(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	gotQuality := quality.Data.(map[string]any)["summary"].(Quality)
	if gotQuality.Total == nil || *gotQuality.Total != 9 || gotQuality.SuccessRate == nil || *gotQuality.SuccessRate != float64(8)/9 {
		t.Fatalf("quality=%+v", gotQuality)
	}
	filteredQuality, err := q.GatewayQualityFiltered(ctx, from, to, "day", []string{"A/B"})
	if err != nil {
		t.Fatal(err)
	}
	filteredQualitySummary := filteredQuality.Data.(map[string]any)["summary"].(Quality)
	if filteredQualitySummary.Total == nil || *filteredQualitySummary.Total != 1 || len(filteredQuality.Data.(map[string]any)["buckets"].([]GatewayQualityBucket)) != 7 {
		t.Fatalf("filtered quality=%+v", filteredQuality.Data)
	}
	preferences, err := q.GatewayModelPreferences(ctx, from, to, []string{"A/B"})
	if err != nil {
		t.Fatal(err)
	}
	prefs := preferences.Data.(map[string]any)["departments"].([]DepartmentPreference)
	if len(prefs) != 1 || prefs[0].Department != "A/B" || prefs[0].Total != 1 {
		t.Fatalf("preferences=%+v", prefs)
	}
	filteredUsers, err := q.GatewayUsersFiltered(ctx, from, to, "day", []string{"A/B"})
	if err != nil {
		t.Fatal(err)
	}
	if filteredUsers.Data.(map[string]any)["summary"].(map[string]int64)["end_users"] != 2 {
		t.Fatalf("filtered users=%+v", filteredUsers.Data)
	}
	oldFrom := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	split := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_user_model_daily(stat_date,user_id,platform,model,success_count,failure_count,gateway_model_duration_sum_ms,gateway_model_duration_samples,gateway_pre_forward_sum_ms,gateway_pre_forward_samples) VALUES('2026-08-29',7,'OpenAI','public-model',25,0,2500,25,250,25)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_rollup_coverage VALUES('call','2026-08-29',true),('call','2026-08-30',true),('call','2026-08-31',true)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE insights_settings SET value=jsonb_set(value,'{call_facts,trusted_since}',to_jsonb($1::timestamptz)) WHERE key='coverage'`, oldFrom); err != nil {
		t.Fatal(err)
	}
	longQuality, err := q.GatewayQualityLongTerm(ctx, oldFrom, to, split, "day", []string{"A/B"})
	if err != nil {
		t.Fatal(err)
	}
	longSummary := longQuality.Data.(map[string]any)["summary"].(Quality)
	if longSummary.Total == nil || *longSummary.Total != 26 {
		t.Fatalf("long quality=%+v", longSummary)
	}
	longPrefs, err := q.GatewayModelPreferencesLongTerm(ctx, oldFrom, to, split, []string{"A/B"})
	if err != nil {
		t.Fatal(err)
	}
	longPrefItems := longPrefs.Data.(map[string]any)["departments"].([]DepartmentPreference)
	if len(longPrefItems) != 1 || longPrefItems[0].Total != 26 {
		t.Fatalf("long prefs=%+v", longPrefItems)
	}
	longUsers, err := q.GatewayUsersLongTerm(ctx, oldFrom, to, split, "day", []string{"A/B"})
	if err != nil {
		t.Fatal(err)
	}
	longFreq := longUsers.Data.(map[string]any)["frequency"].(map[string]int64)
	if longFreq["medium"] != 1 || longFreq["low"] != 1 || longFreq["unclassified"] != 0 {
		t.Fatalf("long frequency=%+v", longFreq)
	}
	if _, err = q.GatewayQualityLongTerm(ctx, oldFrom, to, split, "hour", nil); !errors.Is(err, ErrOutsideRetention) {
		t.Fatalf("hour error=%v", err)
	}

	if _, err = db.ExecContext(ctx, `INSERT INTO users(id,email,username,created_at,deleted_at) VALUES(1,'u1@example','user1',$1,NULL),(2,'u2@example','user2',$1,NULL),(3,'u3@example','user3',$1,NULL)`, from.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(user_id,outcome,statistical_at) SELECT 2,1,$1 FROM generate_series(1,20)`, from); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO insights_call_facts(user_id,outcome,statistical_at) SELECT 3,1,$1 FROM generate_series(1,201)`, from); err != nil {
		t.Fatal(err)
	}
	users, err := q.GatewayUsers(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	frequency := users.Data.(map[string]any)["frequency"].(map[string]int64)
	if frequency["low"] != 7 || frequency["medium"] != 1 || frequency["high"] != 1 || frequency["unclassified"] != 0 {
		t.Fatalf("frequency=%v", frequency)
	}

	if _, err = db.ExecContext(ctx, `INSERT INTO insights_user_lifecycle(user_id,first_observed_call_at,first_call_at,returned_day_1_at,returned_day_7_at,returned_day_30_at,first_call_coverage_complete,return_coverage_complete) VALUES(1,$1,$1,$2,$3,$4,true,true),(2,$1,$1,$2,$3,NULL,true,true),(3,NULL,NULL,NULL,NULL,NULL,false,false)`, from.AddDate(0, 0, -40), from.AddDate(0, 0, -39), from.AddDate(0, 0, -33), from.AddDate(0, 0, -10)); err != nil {
		t.Fatal(err)
	}
	retention, err := q.GatewayRetention(ctx, to)
	if err != nil {
		t.Fatal(err)
	}
	data := retention.Data.(map[string]any)
	if data["next_day"].(RetentionLayer).Count == nil || *data["next_day"].(RetentionLayer).Count != 2 || data["day_7"].(RetentionLayer).Count == nil || *data["day_7"].(RetentionLayer).Count != 2 || data["day_30"].(RetentionLayer).Count == nil || *data["day_30"].(RetentionLayer).Count != 1 || data["first_request"].(RetentionLayer).Status != "partial" {
		t.Fatalf("retention=%v", data)
	}
	if _, err = db.ExecContext(ctx, `UPDATE user_attribute_definitions SET type='multi_select' WHERE id=11`); err != nil {
		t.Fatal(err)
	}
	invalidDimension, err := q.DepartmentDimension(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if invalidDimension.Status != "invalid" {
		t.Fatalf("multi-select dimension=%+v", invalidDimension)
	}
	if _, err = db.ExecContext(ctx, `UPDATE user_attribute_definitions SET type='text' WHERE id=11; INSERT INTO user_attribute_values VALUES(8,11,'Full / Path / Value')`); err != nil {
		t.Fatal(err)
	}
	textDimension, err := q.DepartmentDimension(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundFullValue := false
	for _, option := range textDimension.Options {
		if option.Value == "Full / Path / Value" {
			foundFullValue = true
		}
	}
	if textDimension.Status != "configured" || !foundFullValue {
		t.Fatalf("text dimension=%+v", textDimension)
	}
	hourNow := from.Add(25*time.Hour + 20*time.Minute)
	q.now = func() time.Time { return hourNow }
	hourCoverage, _ := json.Marshal(map[string]Coverage{"call_facts": {Status: CoverageComplete, TrustedSince: &from, ObservedThrough: &hourNow}})
	if _, err = db.ExecContext(ctx, `UPDATE insights_settings SET value=$1 WHERE key='coverage'`, string(hourCoverage)); err != nil {
		t.Fatal(err)
	}
	hourly, err := q.GatewayQualityFiltered(ctx, from, to, "hour", nil)
	if err != nil {
		t.Fatal(err)
	}
	hourBuckets := hourly.Data.(map[string]any)["buckets"].([]GatewayQualityBucket)
	if len(hourBuckets) != 26 || hourBuckets[len(hourBuckets)-1].Complete {
		t.Fatalf("hour buckets extend into future or mark open bucket complete: %+v", hourBuckets)
	}
	for _, b := range hourBuckets {
		if !b.Start.Before(hourNow) {
			t.Fatalf("future zero bucket: %+v", b)
		}
	}
}
