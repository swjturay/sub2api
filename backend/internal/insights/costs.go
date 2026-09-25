package insights

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrCostAccountNotFound     = errors.New("cost account not found")
	ErrCostContributorNotFound = errors.New("cost contributor not found")
	ErrCostShadowAccount       = errors.New("cost account is a linked child")
	ErrCostDeletedAccount      = errors.New("cost account is deleted for the selected month")
	ErrCostStopConflict        = errors.New("cost stop conflicts with an entered next month")
)

var costAmountPattern = regexp.MustCompile(`^\d+(?:\.\d{1,2})?$`)

type CostFilter struct {
	Month        time.Time
	Department   string
	Platform     string
	Search       string
	Contributor  string
	Payment      string
	Status       string
	Completeness string
	Registration string
	Page         int
	PageSize     int
}

type CostContributor struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	DepartmentID string `json:"department_id"`
	Department   string `json:"department"`
	Deleted      bool   `json:"deleted,omitempty"`
}

type CostAccountItem struct {
	ID                 int64            `json:"id"`
	Name               string           `json:"name"`
	Platform           string           `json:"platform"`
	Type               string           `json:"type"`
	Status             string           `json:"status"`
	ExpiresAt          *time.Time       `json:"expires_at"`
	DeletedAt          *time.Time       `json:"deleted_at"`
	Registered         bool             `json:"registered"`
	Inherited          bool             `json:"inherited"`
	ConfigurationMonth string           `json:"configuration_month,omitempty"`
	Contributor        *CostContributor `json:"contributor,omitempty"`
	PaymentMethod      string           `json:"payment_method,omitempty"`
	RequestCount       int64            `json:"request_count"`
	Tokens             Tokens           `json:"tokens"`
	PlatformCost       string           `json:"platform_cost"`
	ActualCost         *string          `json:"actual_cost"`
	Savings            *string          `json:"savings"`
	Notes              string           `json:"notes"`
	UpdatedBy          *CostContributor `json:"updated_by,omitempty"`
	UpdatedAt          *time.Time       `json:"updated_at"`
	platformCostExact  decimal.Decimal
}

type CostSummary struct {
	ContributorCount  int    `json:"contributor_count"`
	AccountCount      int    `json:"account_count"`
	PlatformCost      string `json:"platform_cost"`
	ActualCost        string `json:"actual_cost"`
	Savings           string `json:"savings"`
	CompletedAccounts int    `json:"completed_accounts"`
	TotalAccounts     int    `json:"total_accounts"`
}

type CostTrendPoint struct {
	Month             string `json:"month"`
	PlatformCost      string `json:"platform_cost"`
	ActualCost        string `json:"actual_cost"`
	Savings           string `json:"savings"`
	CompletedAccounts int    `json:"completed_accounts"`
	TotalAccounts     int    `json:"total_accounts"`
}

type CostContributionDepartment struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ContributorCount  int    `json:"contributor_count"`
	AccountCount      int    `json:"account_count"`
	RequestCount      int64  `json:"request_count"`
	Tokens            int64  `json:"tokens"`
	PlatformCost      string `json:"platform_cost"`
	ActualCost        string `json:"actual_cost"`
	Savings           string `json:"savings"`
	CompletedAccounts int    `json:"completed_accounts"`
	TotalAccounts     int    `json:"total_accounts"`
}

type CostUsageDepartment struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RequestCount int64  `json:"request_count"`
	Tokens       int64  `json:"tokens"`
	PlatformCost string `json:"platform_cost"`
}

type CostDepartmentFlow struct {
	ContributionDepartmentID string `json:"contribution_department_id"`
	ContributionDepartment   string `json:"contribution_department"`
	UsageDepartmentID        string `json:"usage_department_id"`
	UsageDepartment          string `json:"usage_department"`
	PlatformCost             string `json:"platform_cost"`
}

type CostDimensions struct {
	Contributors   []CostContributor `json:"contributors"`
	Departments    []DimensionOption `json:"departments"`
	Platforms      []string          `json:"platforms"`
	Statuses       []string          `json:"statuses"`
	PaymentMethods []DimensionOption `json:"payment_methods"`
}

type CostAccountPage struct {
	Items    []CostAccountItem `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Pages    int               `json:"pages"`
}

type CostDashboard struct {
	Month                   string                       `json:"month"`
	InProgress              bool                         `json:"in_progress"`
	Summary                 CostSummary                  `json:"summary"`
	Trend                   []CostTrendPoint             `json:"trend"`
	ContributionDepartments []CostContributionDepartment `json:"contribution_departments"`
	UsageDepartments        []CostUsageDepartment        `json:"usage_departments"`
	Flows                   []CostDepartmentFlow         `json:"flows"`
	Accounts                CostAccountPage              `json:"accounts"`
	Dimensions              CostDimensions               `json:"dimensions"`
}

type SaveCostMonthInput struct {
	AccountID     int64
	Month         time.Time
	ContributorID int64
	PaymentMethod string
	ActualCost    *string
	Notes         string
	UpdatedBy     int64
}

func money(value decimal.Decimal) string { return value.Round(2).StringFixed(2) }

func decimalFromDB(value string) (decimal.Decimal, error) {
	if strings.TrimSpace(value) == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

func (q *Query) CostDashboard(ctx context.Context, filter CostFilter) (Envelope, error) {
	binding, dimension, err := q.resolveDepartmentBinding(ctx)
	if err != nil {
		return Envelope{}, err
	}
	rows, err := q.loadCostAccountRows(ctx, filter.Month, binding)
	if err != nil {
		return Envelope{}, err
	}
	pageRows := filterCostRows(rows, CostFilter{Department: filter.Department, Platform: filter.Platform})
	activeRows := make([]CostAccountItem, 0, len(pageRows))
	for _, row := range pageRows {
		if row.Registered {
			activeRows = append(activeRows, row)
		}
	}
	summary := summarizeCostRows(activeRows)
	trend, err := q.loadCostTrend(ctx, filter, binding)
	if err != nil {
		return Envelope{}, err
	}
	contributionDepartments := summarizeContributionDepartments(activeRows)
	usageDepartments, flows, err := q.loadCostUsageDepartments(ctx, filter, binding)
	if err != nil {
		return Envelope{}, err
	}
	dimensions, err := q.loadCostDimensions(ctx, rows, binding, dimension)
	if err != nil {
		return Envelope{}, err
	}
	tableRows := filterCostRows(pageRows, CostFilter{
		Search: filter.Search, Contributor: filter.Contributor, Payment: filter.Payment,
		Status: filter.Status, Completeness: filter.Completeness, Registration: filter.Registration,
	})
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize != 20 && pageSize != 50 && pageSize != 100 {
		pageSize = 20
	}
	total := len(tableRows)
	pages := (total + pageSize - 1) / pageSize
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	now := q.now().In(mustLocation(q.timezone))
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	data := CostDashboard{
		Month: filter.Month.Format("2006-01"), InProgress: filter.Month.Equal(currentMonth), Summary: summary,
		Trend: trend, ContributionDepartments: contributionDepartments, UsageDepartments: usageDepartments,
		Flows: flows, Accounts: CostAccountPage{Items: tableRows[start:end], Total: total, Page: page, PageSize: pageSize, Pages: pages},
		Dimensions: dimensions,
	}
	coverage := []CoverageInfo{{Dataset: "cost_data", Status: "complete"}, {Dataset: "department_attribute", Status: dimension.Status, Detail: dimension.Detail}}
	return q.Envelope(data, coverage), nil
}

func (q *Query) loadCostAccountRows(ctx context.Context, month time.Time, binding departmentBinding) ([]CostAccountItem, error) {
	monthEnd := month.AddDate(0, 1, 0)
	rows, err := q.db.QueryContext(ctx, `
WITH monthly_usage AS (
    SELECT COALESCE(source.parent_account_id, source.id) AS account_id,
           COUNT(*) AS request_count,
           COALESCE(SUM(ul.input_tokens),0) AS input_tokens,
           COALESCE(SUM(ul.cache_creation_tokens),0) AS cache_creation_tokens,
           COALESCE(SUM(ul.cache_read_tokens),0) AS cache_read_tokens,
           COALESCE(SUM(ul.output_tokens),0) AS output_tokens,
           COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier,1)),0)::text AS platform_cost
    FROM usage_logs ul
    JOIN accounts source ON source.id=ul.account_id
    WHERE ul.created_at >= $2 AND ul.created_at < $3
    GROUP BY COALESCE(source.parent_account_id, source.id)
)
SELECT a.id,a.name,a.platform,a.type,a.status,a.expires_at,a.deleted_at,
       COALESCE(cfg.registered,false),cfg.month,cfg.contributor_user_id,cfg.payment_method,
       COALESCE(contributor.username,''),COALESCE(contributor.email,''),COALESCE(contributor.status,''),contributor.deleted_at,
       COALESCE(NULLIF(contributor_department.value,''),'__unassigned__'),
       exact.actual_cost::text,COALESCE(exact.notes,''),
       COALESCE(exact.updated_at,cfg.updated_at),COALESCE(exact.updated_by,cfg.updated_by),
       COALESCE(editor.username,''),COALESCE(editor.email,''),COALESCE(editor.status,''),editor.deleted_at,
       COALESCE(usage.request_count,0),COALESCE(usage.input_tokens,0),COALESCE(usage.cache_creation_tokens,0),
       COALESCE(usage.cache_read_tokens,0),COALESCE(usage.output_tokens,0),COALESCE(usage.platform_cost,'0')
FROM accounts a
LEFT JOIN LATERAL (
    SELECT month,registered,contributor_user_id,payment_method,updated_by,updated_at
    FROM insights_cost_account_months value
    WHERE value.account_id=a.id AND value.month <= $1
    ORDER BY value.month DESC LIMIT 1
) cfg ON TRUE
LEFT JOIN insights_cost_account_months exact ON exact.account_id=a.id AND exact.month=$1
LEFT JOIN users contributor ON contributor.id=cfg.contributor_user_id
LEFT JOIN user_attribute_values contributor_department ON contributor_department.user_id=contributor.id AND contributor_department.attribute_id=$4
LEFT JOIN users editor ON editor.id=COALESCE(exact.updated_by,cfg.updated_by)
LEFT JOIN monthly_usage usage ON usage.account_id=a.id
		WHERE a.parent_account_id IS NULL
		ORDER BY LOWER(a.platform),LOWER(a.name),a.id`, month.Format("2006-01-02"), month, monthEnd, binding.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]CostAccountItem, 0)
	for rows.Next() {
		var item CostAccountItem
		var expires, deleted, configMonth, contributorDeleted, updatedAt, editorDeleted sql.NullTime
		var contributorID, editorID sql.NullInt64
		var payment, actual sql.NullString
		var contributorName, contributorEmail, contributorStatus, departmentID string
		var editorName, editorEmail, editorStatus, platformCost string
		var input, write, read, output int64
		if err := rows.Scan(&item.ID, &item.Name, &item.Platform, &item.Type, &item.Status, &expires, &deleted,
			&item.Registered, &configMonth, &contributorID, &payment,
			&contributorName, &contributorEmail, &contributorStatus, &contributorDeleted, &departmentID,
			&actual, &item.Notes, &updatedAt, &editorID, &editorName, &editorEmail, &editorStatus, &editorDeleted,
			&item.RequestCount, &input, &write, &read, &output, &platformCost); err != nil {
			return nil, err
		}
		if expires.Valid {
			value := expires.Time
			item.ExpiresAt = &value
		}
		if deleted.Valid {
			value := deleted.Time
			item.DeletedAt = &value
		}
		if item.DeletedAt != nil {
			deletedMonth := time.Date(item.DeletedAt.In(mustLocation(q.timezone)).Year(), item.DeletedAt.In(mustLocation(q.timezone)).Month(), 1, 0, 0, 0, 0, month.Location())
			if month.After(deletedMonth) {
				item.Registered = false
			}
		}
		item.PaymentMethod = payment.String
		item.Tokens = NewTokens(input, write, read, output)
		item.platformCostExact, err = decimalFromDB(platformCost)
		if err != nil {
			return nil, err
		}
		item.PlatformCost = money(item.platformCostExact)
		if configMonth.Valid {
			item.ConfigurationMonth = configMonth.Time.Format("2006-01")
			item.Inherited = item.Registered && configMonth.Time.Format("2006-01") != month.Format("2006-01")
		}
		if item.Registered && contributorID.Valid {
			item.Contributor = &CostContributor{ID: contributorID.Int64, Name: costPersonName(contributorName, contributorEmail), Email: contributorEmail, Status: contributorStatus, DepartmentID: departmentID, Department: departmentLabel(binding, departmentID), Deleted: contributorDeleted.Valid}
		}
		if actual.Valid && item.Registered {
			value, parseErr := decimalFromDB(actual.String)
			if parseErr != nil {
				return nil, parseErr
			}
			formatted := money(value)
			savings := money(item.platformCostExact.Sub(value))
			item.ActualCost, item.Savings = &formatted, &savings
		}
		if updatedAt.Valid {
			value := updatedAt.Time
			item.UpdatedAt = &value
		}
		if editorID.Valid {
			item.UpdatedBy = &CostContributor{ID: editorID.Int64, Name: costPersonName(editorName, editorEmail), Email: editorEmail, Status: editorStatus, Deleted: editorDeleted.Valid}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func costPersonName(username, email string) string {
	if strings.TrimSpace(username) != "" {
		return username
	}
	return email
}

func filterCostRows(rows []CostAccountItem, filter CostFilter) []CostAccountItem {
	search, contributor := strings.ToLower(strings.TrimSpace(filter.Search)), strings.ToLower(strings.TrimSpace(filter.Contributor))
	out := make([]CostAccountItem, 0, len(rows))
	for _, row := range rows {
		if filter.Platform != "" && row.Platform != filter.Platform {
			continue
		}
		if filter.Department != "" && (row.Contributor == nil || row.Contributor.DepartmentID != filter.Department) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(row.Name), search) && !strings.Contains(strconv.FormatInt(row.ID, 10), search) {
			continue
		}
		if contributor != "" && (row.Contributor == nil || (!strings.Contains(strings.ToLower(row.Contributor.Name), contributor) && !strings.Contains(strings.ToLower(row.Contributor.Email), contributor))) {
			continue
		}
		if filter.Payment != "" && row.PaymentMethod != filter.Payment {
			continue
		}
		if filter.Status != "" {
			if filter.Status == "deleted" && row.DeletedAt == nil {
				continue
			}
			if filter.Status != "deleted" && (row.DeletedAt != nil || row.Status != filter.Status) {
				continue
			}
		}
		if filter.Completeness == "complete" && row.ActualCost == nil {
			continue
		}
		if filter.Completeness == "missing" && (!row.Registered || row.ActualCost != nil) {
			continue
		}
		if filter.Registration == "registered" && !row.Registered {
			continue
		}
		if filter.Registration == "unregistered" && row.Registered {
			continue
		}
		out = append(out, row)
	}
	return out
}

func summarizeCostRows(rows []CostAccountItem) CostSummary {
	contributors := map[int64]struct{}{}
	platformCost, actualCost, savings := decimal.Zero, decimal.Zero, decimal.Zero
	completed := 0
	for _, row := range rows {
		platformCost = platformCost.Add(row.platformCostExact)
		if row.Contributor != nil {
			contributors[row.Contributor.ID] = struct{}{}
		}
		if row.ActualCost != nil {
			actual, _ := decimal.NewFromString(*row.ActualCost)
			actualCost = actualCost.Add(actual)
			savings = savings.Add(row.platformCostExact.Sub(actual))
			completed++
		}
	}
	return CostSummary{ContributorCount: len(contributors), AccountCount: len(rows), PlatformCost: money(platformCost), ActualCost: money(actualCost), Savings: money(savings), CompletedAccounts: completed, TotalAccounts: len(rows)}
}

func summarizeContributionDepartments(rows []CostAccountItem) []CostContributionDepartment {
	type aggregate struct {
		item                      CostContributionDepartment
		contributors              map[int64]struct{}
		platform, actual, savings decimal.Decimal
	}
	byDepartment := map[string]*aggregate{}
	for _, row := range rows {
		id, name := "__unassigned__", "未分配"
		if row.Contributor != nil {
			id, name = row.Contributor.DepartmentID, row.Contributor.Department
		}
		a := byDepartment[id]
		if a == nil {
			a = &aggregate{item: CostContributionDepartment{ID: id, Name: name}, contributors: map[int64]struct{}{}}
			byDepartment[id] = a
		}
		a.item.AccountCount++
		a.item.TotalAccounts++
		a.item.RequestCount += row.RequestCount
		a.item.Tokens += row.Tokens.Total
		a.platform = a.platform.Add(row.platformCostExact)
		if row.Contributor != nil {
			a.contributors[row.Contributor.ID] = struct{}{}
		}
		if row.ActualCost != nil {
			actual, _ := decimal.NewFromString(*row.ActualCost)
			a.actual = a.actual.Add(actual)
			a.savings = a.savings.Add(row.platformCostExact.Sub(actual))
			a.item.CompletedAccounts++
		}
	}
	out := make([]CostContributionDepartment, 0, len(byDepartment))
	for _, a := range byDepartment {
		a.item.ContributorCount = len(a.contributors)
		a.item.PlatformCost = money(a.platform)
		a.item.ActualCost = money(a.actual)
		a.item.Savings = money(a.savings)
		out = append(out, a.item)
	}
	sort.Slice(out, func(i, j int) bool {
		return moneyValue(out[i].PlatformCost).GreaterThan(moneyValue(out[j].PlatformCost))
	})
	return out
}

func (q *Query) loadCostTrend(ctx context.Context, filter CostFilter, binding departmentBinding) ([]CostTrendPoint, error) {
	start := filter.Month.AddDate(0, -11, 0)
	minimum := time.Date(2026, 6, 1, 0, 0, 0, 0, filter.Month.Location())
	if start.Before(minimum) {
		start = minimum
	}
	rows, err := q.db.QueryContext(ctx, `
WITH months AS (SELECT generate_series($1::date,$2::date,INTERVAL '1 month')::date AS month),
roots AS (SELECT id,platform,deleted_at FROM accounts WHERE parent_account_id IS NULL),
monthly_usage AS (
 SELECT date_trunc('month',ul.created_at AT TIME ZONE $3)::date AS month,
        COALESCE(source.parent_account_id,source.id) AS account_id,
        COALESCE(SUM(COALESCE(ul.account_stats_cost,ul.total_cost)*COALESCE(ul.account_rate_multiplier,1)),0)::text AS platform_cost
 FROM usage_logs ul JOIN accounts source ON source.id=ul.account_id
 WHERE ul.created_at >= $4 AND ul.created_at < $5
 GROUP BY 1,2)
SELECT months.month,roots.platform,roots.deleted_at,COALESCE(cfg.registered,false),cfg.contributor_user_id,
       COALESCE(NULLIF(department.value,''),'__unassigned__'),exact.actual_cost::text,COALESCE(usage.platform_cost,'0')
FROM months CROSS JOIN roots
LEFT JOIN LATERAL (SELECT registered,contributor_user_id FROM insights_cost_account_months value WHERE value.account_id=roots.id AND value.month<=months.month ORDER BY value.month DESC LIMIT 1) cfg ON TRUE
LEFT JOIN insights_cost_account_months exact ON exact.account_id=roots.id AND exact.month=months.month
LEFT JOIN user_attribute_values department ON department.user_id=cfg.contributor_user_id AND department.attribute_id=$6
LEFT JOIN monthly_usage usage ON usage.account_id=roots.id AND usage.month=months.month
		ORDER BY months.month`, start.Format("2006-01-02"), filter.Month.Format("2006-01-02"), q.timezone, start, filter.Month.AddDate(0, 1, 0), binding.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	type agg struct {
		platform, actual, savings decimal.Decimal
		completed, total          int
	}
	byMonth := map[string]*agg{}
	for rows.Next() {
		var month time.Time
		var platform, departmentID, platformRaw string
		var deleted sql.NullTime
		var registered bool
		var contributor sql.NullInt64
		var actual sql.NullString
		if err := rows.Scan(&month, &platform, &deleted, &registered, &contributor, &departmentID, &actual, &platformRaw); err != nil {
			return nil, err
		}
		if !registered || (deleted.Valid && month.After(time.Date(deleted.Time.In(mustLocation(q.timezone)).Year(), deleted.Time.In(mustLocation(q.timezone)).Month(), 1, 0, 0, 0, 0, month.Location()))) {
			continue
		}
		if filter.Platform != "" && platform != filter.Platform {
			continue
		}
		if filter.Department != "" && departmentID != filter.Department {
			continue
		}
		key := month.Format("2006-01")
		a := byMonth[key]
		if a == nil {
			a = &agg{}
			byMonth[key] = a
		}
		cost, e := decimalFromDB(platformRaw)
		if e != nil {
			return nil, e
		}
		a.platform = a.platform.Add(cost)
		a.total++
		if actual.Valid {
			value, e := decimalFromDB(actual.String)
			if e != nil {
				return nil, e
			}
			a.actual = a.actual.Add(value)
			a.savings = a.savings.Add(cost.Sub(value))
			a.completed++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]CostTrendPoint, 0)
	for month := start; !month.After(filter.Month); month = month.AddDate(0, 1, 0) {
		key := month.Format("2006-01")
		a := byMonth[key]
		if a == nil {
			a = &agg{}
		}
		out = append(out, CostTrendPoint{Month: key, PlatformCost: money(a.platform), ActualCost: money(a.actual), Savings: money(a.savings), CompletedAccounts: a.completed, TotalAccounts: a.total})
	}
	return out, nil
}

func (q *Query) loadCostUsageDepartments(ctx context.Context, filter CostFilter, binding departmentBinding) ([]CostUsageDepartment, []CostDepartmentFlow, error) {
	rows, err := q.db.QueryContext(ctx, `
SELECT root.platform,COALESCE(NULLIF(contribution_department.value,''),'__unassigned__'),COALESCE(NULLIF(usage_department.value,''),'__unassigned__'),
       COUNT(*),COALESCE(SUM(ul.input_tokens+ul.cache_creation_tokens+ul.cache_read_tokens+ul.output_tokens),0),
       COALESCE(SUM(COALESCE(ul.account_stats_cost,ul.total_cost)*COALESCE(ul.account_rate_multiplier,1)),0)::text
FROM usage_logs ul
JOIN accounts source ON source.id=ul.account_id
JOIN accounts root ON root.id=COALESCE(source.parent_account_id,source.id)
JOIN LATERAL (SELECT registered,contributor_user_id FROM insights_cost_account_months value WHERE value.account_id=root.id AND value.month<=$1 ORDER BY value.month DESC LIMIT 1) cfg ON cfg.registered=TRUE
LEFT JOIN user_attribute_values contribution_department ON contribution_department.user_id=cfg.contributor_user_id AND contribution_department.attribute_id=$4
LEFT JOIN user_attribute_values usage_department ON usage_department.user_id=ul.user_id AND usage_department.attribute_id=$4
WHERE ul.created_at >= $2 AND ul.created_at < $3 AND (root.deleted_at IS NULL OR root.deleted_at >= $2)
		GROUP BY 1,2,3`, filter.Month.Format("2006-01-02"), filter.Month, filter.Month.AddDate(0, 1, 0), binding.ID)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	type usageAgg struct {
		requests, tokens int64
		cost             decimal.Decimal
	}
	usageMap := map[string]*usageAgg{}
	flowMap := map[string]*usageAgg{}
	for rows.Next() {
		var platform, contributionID, usageID, costRaw string
		var requests, tokens int64
		if err := rows.Scan(&platform, &contributionID, &usageID, &requests, &tokens, &costRaw); err != nil {
			return nil, nil, err
		}
		if filter.Platform != "" && platform != filter.Platform {
			continue
		}
		if filter.Department != "" && contributionID != filter.Department {
			continue
		}
		cost, e := decimalFromDB(costRaw)
		if e != nil {
			return nil, nil, e
		}
		u := usageMap[usageID]
		if u == nil {
			u = &usageAgg{}
			usageMap[usageID] = u
		}
		u.requests += requests
		u.tokens += tokens
		u.cost = u.cost.Add(cost)
		key := contributionID + "\x00" + usageID
		f := flowMap[key]
		if f == nil {
			f = &usageAgg{}
			flowMap[key] = f
		}
		f.cost = f.cost.Add(cost)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	usage := make([]CostUsageDepartment, 0, len(usageMap))
	for id, a := range usageMap {
		usage = append(usage, CostUsageDepartment{ID: id, Name: departmentLabel(binding, id), RequestCount: a.requests, Tokens: a.tokens, PlatformCost: money(a.cost)})
	}
	sort.Slice(usage, func(i, j int) bool {
		return moneyValue(usage[i].PlatformCost).GreaterThan(moneyValue(usage[j].PlatformCost))
	})
	flows := make([]CostDepartmentFlow, 0, len(flowMap))
	for key, a := range flowMap {
		parts := strings.SplitN(key, "\x00", 2)
		flows = append(flows, CostDepartmentFlow{ContributionDepartmentID: parts[0], ContributionDepartment: departmentLabel(binding, parts[0]), UsageDepartmentID: parts[1], UsageDepartment: departmentLabel(binding, parts[1]), PlatformCost: money(a.cost)})
	}
	sort.Slice(flows, func(i, j int) bool {
		return moneyValue(flows[i].PlatformCost).GreaterThan(moneyValue(flows[j].PlatformCost))
	})
	return usage, flows, nil
}

func (q *Query) loadCostDimensions(ctx context.Context, rows []CostAccountItem, binding departmentBinding, dimension DepartmentDimension) (CostDimensions, error) {
	userRows, err := q.db.QueryContext(ctx, `SELECT u.id,COALESCE(u.username,''),u.email,u.status,COALESCE(NULLIF(d.value,''),'__unassigned__') FROM users u LEFT JOIN user_attribute_values d ON d.user_id=u.id AND d.attribute_id=$1 WHERE u.deleted_at IS NULL ORDER BY LOWER(COALESCE(NULLIF(u.username,''),u.email)),u.id`, binding.ID)
	if err != nil {
		return CostDimensions{}, err
	}
	defer func() { _ = userRows.Close() }()
	contributors := make([]CostContributor, 0)
	for userRows.Next() {
		var item CostContributor
		if err := userRows.Scan(&item.ID, &item.Name, &item.Email, &item.Status, &item.DepartmentID); err != nil {
			return CostDimensions{}, err
		}
		item.Name = costPersonName(item.Name, item.Email)
		item.Department = departmentLabel(binding, item.DepartmentID)
		contributors = append(contributors, item)
	}
	if err := userRows.Err(); err != nil {
		return CostDimensions{}, err
	}
	platformSet, statusSet := map[string]struct{}{}, map[string]struct{}{}
	for _, row := range rows {
		platformSet[row.Platform] = struct{}{}
		if row.DeletedAt != nil {
			statusSet["deleted"] = struct{}{}
		} else {
			statusSet[row.Status] = struct{}{}
		}
	}
	platforms := mapKeys(platformSet)
	statuses := mapKeys(statusSet)
	return CostDimensions{Contributors: contributors, Departments: dimension.Options, Platforms: platforms, Statuses: statuses, PaymentMethods: []DimensionOption{{Value: "subscription", Label: "订阅"}, {Value: "payg", Label: "即用即付"}, {Value: "other", Label: "其他"}}}, nil
}

func mapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func (q *Query) SaveCostMonth(ctx context.Context, input SaveCostMonthInput) error {
	if err := q.validateCostAccountMonth(ctx, input.AccountID, input.Month); err != nil {
		return err
	}
	var exists bool
	if err := q.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL)`, input.ContributorID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrCostContributorNotFound
	}
	_, err := q.db.ExecContext(ctx, `INSERT INTO insights_cost_account_months(account_id,month,registered,contributor_user_id,payment_method,actual_cost,notes,updated_by) VALUES($1,$2,TRUE,$3,$4,$5,$6,$7) ON CONFLICT(account_id,month) DO UPDATE SET registered=TRUE,contributor_user_id=EXCLUDED.contributor_user_id,payment_method=EXCLUDED.payment_method,actual_cost=EXCLUDED.actual_cost,notes=EXCLUDED.notes,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, input.AccountID, input.Month.Format("2006-01-02"), input.ContributorID, input.PaymentMethod, input.ActualCost, input.Notes, input.UpdatedBy)
	return err
}

func (q *Query) StopCostAccountAfter(ctx context.Context, accountID int64, afterMonth time.Time, updatedBy int64) error {
	if err := q.validateCostAccountMonth(ctx, accountID, afterMonth); err != nil {
		return err
	}
	next := afterMonth.AddDate(0, 1, 0)
	result, err := q.db.ExecContext(ctx, `INSERT INTO insights_cost_account_months(account_id,month,registered,notes,updated_by) VALUES($1,$2,FALSE,'',$3) ON CONFLICT(account_id,month) DO UPDATE SET registered=FALSE,contributor_user_id=NULL,payment_method=NULL,actual_cost=NULL,notes='',updated_by=EXCLUDED.updated_by,updated_at=NOW() WHERE insights_cost_account_months.actual_cost IS NULL`, accountID, next.Format("2006-01-02"), updatedBy)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrCostStopConflict
	}
	return nil
}

func (q *Query) validateCostAccountMonth(ctx context.Context, accountID int64, month time.Time) error {
	var parent sql.NullInt64
	var deleted sql.NullTime
	err := q.db.QueryRowContext(ctx, `SELECT parent_account_id,deleted_at FROM accounts WHERE id=$1`, accountID).Scan(&parent, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCostAccountNotFound
	}
	if err != nil {
		return err
	}
	if parent.Valid {
		return ErrCostShadowAccount
	}
	if deleted.Valid {
		loc := mustLocation(q.timezone)
		deletedMonth := time.Date(deleted.Time.In(loc).Year(), deleted.Time.In(loc).Month(), 1, 0, 0, 0, 0, month.Location())
		if month.After(deletedMonth) {
			return ErrCostDeletedAccount
		}
	}
	return nil
}

func ValidateCostPaymentMethod(value string) bool {
	return value == "subscription" || value == "payg" || value == "other"
}
func NormalizeCostAmount(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !costAmountPattern.MatchString(value) {
		return "", ErrInvalidFilter
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil || parsed.IsNegative() || parsed.Exponent() < -2 {
		return "", ErrInvalidFilter
	}
	if len(parsed.Truncate(0).String()) > 18 {
		return "", ErrInvalidFilter
	}
	return parsed.StringFixed(2), nil
}
func moneyValue(value string) decimal.Decimal {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero
	}
	return parsed
}
