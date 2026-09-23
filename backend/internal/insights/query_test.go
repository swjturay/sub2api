package insights

import (
	"math"
	"testing"
	"time"
)

func TestTokenFormulas(t *testing.T) {
	tokens := NewTokens(100, 20, 30, 50)
	if tokens.Total != 200 {
		t.Fatalf("total=%d", tokens.Total)
	}
	if got := CacheHitRatio(tokens); got == nil || math.Abs(*got-0.2) > 1e-9 {
		t.Fatalf("cache ratio=%v", got)
	}
	if got := Ratio(tokens.Output, tokens.Total); got == nil || math.Abs(*got-0.25) > 1e-9 {
		t.Fatalf("output ratio=%v", got)
	}
	if CacheHitRatio(Tokens{}) != nil {
		t.Fatal("zero denominator must be nil")
	}
}

func TestEstimatedTPOT(t *testing.T) {
	got := EstimatedTPOT(1000, 100, 10)
	if got == nil || *got != 100 {
		t.Fatalf("tpot=%v", got)
	}
	for _, tc := range [][3]int64{{100, 100, 10}, {100, 10, 1}, {100, -1, 10}} {
		if EstimatedTPOT(tc[0], tc[1], tc[2]) != nil {
			t.Fatalf("invalid sample accepted: %v", tc)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 22, 1, 2, 3, 4, time.UTC)
	encoded := encodeCursor(at, "42")
	gotAt, gotID, err := decodeCursor(encoded)
	if err != nil || !gotAt.Equal(at) || gotID != "42" {
		t.Fatalf("roundtrip: %v %q %v", gotAt, gotID, err)
	}
	if _, _, err := decodeCursor("bad"); err == nil {
		t.Fatal("expected invalid cursor")
	}
}

func TestFrequencyBoundaries(t *testing.T) {
	cases := map[int64]string{0: "low", 19: "low", 20: "medium", 200: "medium", 201: "high"}
	for count, want := range cases {
		if got := ClassifyFrequency(count); got != want {
			t.Fatalf("count %d: got %s want %s", count, got, want)
		}
	}
}

func TestParseModelIdentityPreservesName(t *testing.T) {
	got, err := ParseModelIdentity("OpenAI:Org/Model:Preview-V1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != "OpenAI" || got.Name != "Org/Model:Preview-V1" {
		t.Fatalf("identity=%+v", got)
	}
	for _, value := range []string{"missing-colon", ":name", "platform:"} {
		if _, err := ParseModelIdentity(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestBucketEndHour(t *testing.T) {
	start := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	if got := bucketEnd(start, "hour"); !got.Equal(start.Add(time.Hour)) {
		t.Fatalf("hour end=%v", got)
	}
}

func TestCoverageWindowClampsNaturalDayEndToNow(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	since := now.Add(-24 * time.Hour)
	through := now
	q := &Query{now: func() time.Time { return now }}
	if got := q.coverageWindowStatus(Coverage{Status: CoverageComplete, TrustedSince: &since, ObservedThrough: &through}, since, time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)); got != "complete" {
		t.Fatalf("status=%s", got)
	}
	before := since.Add(-time.Second)
	if got := q.coverageWindowStatus(Coverage{Status: CoverageComplete, TrustedSince: &since, ObservedThrough: &through}, before, now); got != "partial" {
		t.Fatalf("pre-trust status=%s", got)
	}
}
