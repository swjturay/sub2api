package insights

import (
	"testing"
	"time"
)

func TestDepartmentBucketBreakdownsAndHybridMerge(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	q := &Query{timezone: "UTC", now: func() time.Time { return from.AddDate(0, 1, 0) }}
	binding := departmentBinding{Options: []DimensionOption{{Value: "a", Label: "研发"}, {Value: "b", Label: "研发"}}}
	for _, granularity := range []string{"day", "week", "month"} {
		t.Run(granularity, func(t *testing.T) {
			finish := func(aggs []departmentAggregate) []DepartmentBucket {
				t.Helper()
				result := q.finishDepartments(from, from.AddDate(0, 0, 2), granularity, nil, 2, aggs, ModelPerformance{}, "usage_detail", binding)
				data, ok := result.Data.(map[string]any)
				if !ok {
					t.Fatalf("unexpected department response type: %T", result.Data)
				}
				buckets, ok := data["buckets"].([]DepartmentBucket)
				if !ok {
					t.Fatalf("unexpected department buckets type: %T", data["buckets"])
				}
				return buckets
			}
			old := finish([]departmentAggregate{
				{bucket: from, userID: 1, department: "a", model: "openai:x", requests: 2, tokens: NewTokens(10, 0, 0, 2)},
				{bucket: from, userID: 2, department: "b", model: "anthropic:x", requests: 1, tokens: NewTokens(20, 0, 0, 3)},
			})
			recent := finish([]departmentAggregate{
				{bucket: from, userID: 1, department: "a", model: "openai:x", requests: 3, tokens: NewTokens(30, 0, 0, 4)},
				{bucket: from.AddDate(0, 1, 0), userID: 2, department: "b", model: "openai:x", requests: 1, tokens: NewTokens(5, 0, 0, 1)},
			})
			merged := mergeDepartmentBuckets(old, recent)
			if len(merged) != 2 {
				t.Fatalf("buckets=%d", len(merged))
			}
			first := merged[0]
			if first.Tokens.Total != 69 || first.RequestCount != 6 || len(first.Models) != 2 || len(first.Departments) != 2 || len(first.Members) != 2 {
				t.Fatalf("merged=%+v", first)
			}
			if first.Models[0].Model != "openai:x" || first.Models[0].Tokens.Total != 46 || first.Models[0].RequestCount != 5 {
				t.Fatalf("models=%+v", first.Models)
			}
			if first.Departments[0].ID != "a" || first.Departments[0].Tokens.Total != 46 {
				t.Fatalf("departments=%+v", first.Departments)
			}
			if first.Members[0].UserID != 1 || first.Members[0].TotalTokens != 46 || first.Members[1].UserID != 2 || first.Members[1].TotalTokens != 23 {
				t.Fatalf("members=%+v", first.Members)
			}
			for _, bucket := range merged {
				var models, departments Tokens
				for _, item := range bucket.Models {
					addTokens(&models, item.Tokens)
				}
				for _, item := range bucket.Departments {
					addTokens(&departments, item.Tokens)
				}
				if models != bucket.Tokens || departments != bucket.Tokens {
					t.Fatalf("category totals differ: %+v", bucket)
				}
			}
			if old[0].Tokens.Total != 35 || len(old[0].Models) != 2 {
				t.Fatal("merge mutated historical input")
			}
		})
	}
}

func TestDepartmentOverviewCoalescesEquivalentBucketInstants(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	databaseZone := time.FixedZone("CST", 8*60*60)
	monthFromDaily := time.Date(2026, 11, 1, 0, 0, 0, 0, shanghai)
	monthFromRaw := time.Date(2026, 11, 1, 0, 0, 0, 0, databaseZone)
	if !monthFromDaily.Equal(monthFromRaw) || monthFromDaily == monthFromRaw {
		t.Fatalf("fixture must represent one instant with distinct location identities: daily=%#v raw=%#v", monthFromDaily, monthFromRaw)
	}

	q := &Query{timezone: "Asia/Shanghai", now: func() time.Time {
		return time.Date(2026, 11, 12, 12, 0, 0, 0, shanghai)
	}}
	result := q.finishDepartmentOverview(
		time.Date(2026, 7, 1, 0, 0, 0, 0, shanghai),
		time.Date(2026, 11, 13, 0, 0, 0, 0, shanghai),
		"month",
		1,
		[]DepartmentMemberCount{{ID: "研发", Label: "研发", Count: 1}},
		[]departmentOverviewAggregate{
			{bucket: monthFromDaily, userID: 1, department: "研发", requests: 1, tokens: NewTokens(10, 0, 0, 1)},
			{bucket: monthFromRaw, userID: 1, department: "研发", requests: 1, tokens: NewTokens(20, 0, 0, 2)},
		},
		departmentBinding{},
		nil,
	)
	buckets := result.Data.(map[string]any)["buckets"].([]DepartmentBucket)
	distribution := result.Data.(map[string]any)["member_distribution"].([]DepartmentMemberCount)
	if len(buckets) != 1 {
		t.Fatalf("equivalent November bucket was split: got %d buckets: %+v", len(buckets), buckets)
	}
	if buckets[0].Tokens.Total != 33 || buckets[0].RequestCount != 2 {
		t.Fatalf("equivalent November bucket totals were not merged: %+v", buckets[0])
	}
	if len(distribution) != 1 || distribution[0].Count != 1 {
		t.Fatalf("member distribution=%+v", distribution)
	}
}
