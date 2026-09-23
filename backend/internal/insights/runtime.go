package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"
)

type RuntimeOptions struct {
	TrustedCollection       bool
	TrustedUsageHistoryFrom *time.Time
}

type runtimeRecorder struct {
	store             *Store
	db                *sql.DB
	timezone          string
	queue             chan CallFact
	gaps              chan string
	stop              chan struct{}
	done              chan struct{}
	maintenanceDone   chan struct{}
	cancelMaintenance context.CancelFunc
	once              sync.Once
	mu                sync.Mutex
	coverage          Coverage
	usageCoverage     Coverage
	pendingUsage      int
	dirtyUsers        map[int64]struct{}
}

var defaultRuntime struct {
	sync.RWMutex
	recorder *runtimeRecorder
}

func ConfigureRuntime(db *sql.DB, timezone string, queueSize int) {
	ConfigureRuntimeWithOptions(db, timezone, queueSize, RuntimeOptions{})
}

// TrustedCollection is an explicit operator rollout gate, not an assertion
// inferred from process startup. It applies only from this process's activation.
func ConfigureRuntimeWithOptions(db *sql.DB, timezone string, queueSize int, options RuntimeOptions) {
	if db == nil {
		return
	}
	if queueSize <= 0 {
		queueSize = 1024
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &runtimeRecorder{db: db, store: NewStore(db), timezone: timezone, queue: make(chan CallFact, queueSize), gaps: make(chan string, 16), stop: make(chan struct{}), done: make(chan struct{}), maintenanceDone: make(chan struct{}), cancelMaintenance: cancel, coverage: Coverage{Status: CoverageUnknown}, usageCoverage: Coverage{Status: CoverageUnknown}, dirtyUsers: make(map[int64]struct{})}
	defaultRuntime.Lock()
	old := defaultRuntime.recorder
	defaultRuntime.recorder = r
	defaultRuntime.Unlock()
	if old != nil {
		old.once.Do(func() { old.cancelMaintenance(); close(old.stop) })
	}
	if options.TrustedCollection {
		// The operator gate covers the serving fleet. Preserve a previously
		// trusted interval; a recorded gap starts a new interval instead.
		loadCtx, done := context.WithTimeout(context.Background(), 2*time.Second)
		var raw []byte
		if err := db.QueryRowContext(loadCtx, `SELECT value->'call_facts' FROM insights_settings WHERE key='coverage'`).Scan(&raw); err == nil {
			var previous Coverage
			if json.Unmarshal(raw, &previous) == nil && previous.Status == CoverageComplete && previous.TrustedSince != nil {
				r.coverage = previous
			}
		}
		done()
		r.coverage = r.coverage.Activate(time.Now())
		loadUsageCtx, doneUsage := context.WithTimeout(context.Background(), 2*time.Second)
		var usageRaw []byte
		if db.QueryRowContext(loadUsageCtx, `SELECT value->'usage' FROM insights_settings WHERE key='coverage'`).Scan(&usageRaw) == nil {
			var previous Coverage
			if json.Unmarshal(usageRaw, &previous) == nil && previous.Status == CoverageComplete && previous.TrustedSince != nil {
				r.usageCoverage = previous
			}
		}
		doneUsage()
		r.usageCoverage = r.usageCoverage.Activate(time.Now())
		if options.TrustedUsageHistoryFrom != nil {
			historyCtx, historyDone := context.WithTimeout(context.Background(), 5*time.Second)
			extended, extendErr := r.store.ExtendTrustedUsageHistory(historyCtx, r.usageCoverage, *options.TrustedUsageHistoryFrom, timezone)
			historyDone()
			if extendErr != nil {
				log.Printf("[Insights] trusted usage history: %v", extendErr)
			} else {
				r.usageCoverage = extended
			}
		}
	}
	go r.run()
	go r.maintain(ctx)
}

func ActivateTrustedCoverage(ctx context.Context, at time.Time) error {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	defaultRuntime.RUnlock()
	if r == nil {
		return errors.New("insights runtime is not configured")
	}
	if at.IsZero() {
		at = time.Now()
	}
	r.mu.Lock()
	r.coverage = r.coverage.Activate(at)
	coverage := r.coverage
	r.mu.Unlock()
	return r.store.SaveCoverage(ctx, map[string]Coverage{"call_facts": coverage, "errors": coverage})
}
func RecordBestEffort(f CallFact) bool {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	if r == nil {
		defaultRuntime.RUnlock()
		return false
	}
	select {
	case r.queue <- f:
		defaultRuntime.RUnlock()
		return true
	default:
		defaultRuntime.RUnlock()
		ReportGapBestEffort("collection queue full")
		return false
	}
}
func ReportGapBestEffort(reason string) {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	defer defaultRuntime.RUnlock()
	if r != nil {
		r.mu.Lock()
		r.coverage = r.coverage.MarkGap(time.Now(), reason)
		r.mu.Unlock()
		select {
		case r.gaps <- reason:
		default:
		}
	}
}
func (r *runtimeRecorder) persist(f CallFact) {
	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	err := r.store.StoreCall(ctx, f)
	c()
	if err != nil {
		r.markGap("call fact persistence failed")
		return
	}
	r.mu.Lock()
	if f.UserID != nil && len(r.dirtyUsers) < 4096 {
		r.dirtyUsers[*f.UserID] = struct{}{}
	}
	if r.coverage.Status == CoverageUnknown {
		r.coverage = Coverage{Status: CoveragePartial, ObservedThrough: &f.StatisticalAt, Reason: "observed before trusted activation"}
	} else {
		r.coverage = r.coverage.Observe(f.StatisticalAt)
	}
	r.mu.Unlock()
}
func (r *runtimeRecorder) run() {
	defer close(r.done)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case f := <-r.queue:
			r.persist(f)
		case reason := <-r.gaps:
			r.markGap(reason)
		case <-ticker.C:
			r.flushCoverage()
		case <-r.stop:
			for {
				select {
				case f := <-r.queue:
					r.persist(f)
				case reason := <-r.gaps:
					r.markGap(reason)
				default:
					r.flushCoverage()
					return
				}
			}
		}
	}
}
func (r *runtimeRecorder) flushCoverage() {
	r.mu.Lock()
	coverage := r.coverage
	// Idle intervals are observed too; a heartbeat never repairs a reported gap.
	if coverage.Status == CoverageComplete {
		coverage = coverage.Observe(time.Now())
		r.coverage = coverage
	}
	usageCoverage := r.usageCoverage
	if usageCoverage.Status == CoverageComplete && r.pendingUsage == 0 {
		usageCoverage = usageCoverage.Observe(time.Now())
		r.usageCoverage = usageCoverage
	}
	r.mu.Unlock()
	ctx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	if err := r.store.SaveCoverage(ctx, map[string]Coverage{"call_facts": coverage, "errors": coverage, "usage": usageCoverage}); err != nil {
		r.mu.Lock()
		r.coverage = r.coverage.MarkGap(time.Now(), "coverage heartbeat persistence failed")
		r.usageCoverage = r.usageCoverage.MarkGap(time.Now(), "coverage heartbeat persistence failed")
		r.mu.Unlock()
	}
}
func (r *runtimeRecorder) markGap(reason string) {
	r.mu.Lock()
	r.coverage = r.coverage.MarkGap(time.Now(), reason)
	coverage := r.coverage
	r.mu.Unlock()
	ctx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	_ = r.store.SaveCoverage(ctx, map[string]Coverage{"call_facts": coverage, "errors": coverage})
}
func (r *runtimeRecorder) maintain(ctx context.Context) {
	defer close(r.maintenanceDone)
	loc, err := time.LoadLocation(r.timezone)
	if err != nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		now := time.Now().In(loc)
		today := DayAt(now, loc)
		work, cancel := context.WithTimeout(ctx, 50*time.Second)
		r.maintainBatch(work, today)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *runtimeRecorder) maintainBatch(ctx context.Context, today time.Time) {
	var existingRaw []byte
	var existing AggregationConfig
	if err := r.db.QueryRowContext(ctx, `SELECT value FROM insights_settings WHERE key='aggregation_config'`).Scan(&existingRaw); err == nil {
		_ = json.Unmarshal(existingRaw, &existing)
	}
	cfg := existing.Configure(1, r.timezone)
	if existing.Timezone != "" && existing.Timezone != r.timezone {
		cfg.RebuildRequired = true
		r.markGap("aggregation timezone changed; retained history rebuild required")
	}
	if err := r.store.SaveAggregationConfig(ctx, cfg); err != nil {
		log.Printf("[Insights] aggregation config: %v", err)
		return
	}
	// Keep the live three-day window fresh; history initializes in bounded day
	// batches, never as an unbounded startup scan of the usage table.
	if err := r.store.RollupRange(ctx, today.AddDate(0, 0, -2), today.AddDate(0, 0, 1), r.timezone); err != nil {
		log.Printf("[Insights] live rollup: %v", err)
		return
	}
	var state struct {
		NextDate   string `json:"next_date"`
		UserCursor int64  `json:"user_cursor"`
	}
	var raw []byte
	if err := r.db.QueryRowContext(ctx, `SELECT value FROM insights_settings WHERE key='maintenance_state'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &state)
	}
	next, err := time.ParseInLocation("2006-01-02", state.NextDate, today.Location())
	if err != nil || next.Before(today.AddDate(0, 0, -365)) {
		next = today.AddDate(0, 0, -365)
	}
	for i := 0; i < 2 && next.Before(today.AddDate(0, 0, -2)); i++ {
		end := next.AddDate(0, 0, 1)
		if err := r.store.RollupRange(ctx, next, end, r.timezone); err != nil {
			log.Printf("[Insights] historical rollup: %v", err)
			return
		}
		next = end
	}
	state.NextDate = next.Format("2006-01-02")
	if !next.Before(today.AddDate(0, 0, -2)) && cfg.PreviousTimezone == "" && (existing.Timezone == "" || existing.Timezone == r.timezone) {
		cfg.RebuildRequired = false
		_ = r.store.SaveAggregationConfig(ctx, cfg)
	}
	r.mu.Lock()
	coverage := r.coverage
	ids := make([]int64, 0, 500)
	for id := range r.dirtyUsers {
		if len(ids) >= 300 {
			break
		}
		ids = append(ids, id)
		delete(r.dirtyUsers, id)
	}
	r.mu.Unlock()
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM users WHERE id>$1 AND deleted_at IS NULL ORDER BY id LIMIT 200`, state.UserCursor)
	if err != nil {
		log.Printf("[Insights] lifecycle scan: %v", err)
		return
	}
	n := 0
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return
		}
		ids = append(ids, id)
		state.UserCursor = id
		n++
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return
	}
	if n < 200 {
		state.UserCursor = 0
	}
	if err := r.store.RebuildLifecycleUsers(ctx, r.timezone, coverage, ids); err != nil {
		log.Printf("[Insights] lifecycle batch: %v", err)
		return
	}
	if err := r.store.DeleteExpiredDetails(ctx, time.Now(), r.timezone); err != nil {
		log.Printf("[Insights] detail retention: %v", err)
		return
	}
	encoded, _ := json.Marshal(state)
	if _, err := r.db.ExecContext(ctx, `INSERT INTO insights_settings(key,value,updated_at) VALUES('maintenance_state',$1::jsonb,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, encoded); err != nil {
		log.Printf("[Insights] maintenance checkpoint: %v", err)
	}
}
func ShutdownRuntime(ctx context.Context) error {
	defaultRuntime.Lock()
	r := defaultRuntime.recorder
	defaultRuntime.recorder = nil
	defaultRuntime.Unlock()
	if r == nil {
		return nil
	}
	r.once.Do(func() { r.cancelMaintenance(); close(r.stop) })
	for _, done := range []chan struct{}{r.done, r.maintenanceDone} {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func PrepareUsageCleanup(ctx context.Context, from, to time.Time, timezone string) error {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	defaultRuntime.RUnlock()
	if r == nil {
		return errors.New("insights runtime is not configured")
	}
	if err := r.store.RollupRange(ctx, from, to, timezone); err != nil {
		ReportGapBestEffort("usage cleanup rollup failed")
		return err
	}
	return nil
}

// PrepareUsageRetention is exclusively for whole-system automatic retention.
// User-filtered manual deletion archives its rows and must not advance this mark.
func PrepareUsageRetention(ctx context.Context, to time.Time, timezone string) error {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	defaultRuntime.RUnlock()
	if r == nil {
		return errors.New("insights runtime is not configured")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	cutoff := DayAt(to, loc)
	now := time.Now().In(loc)
	from := time.Date(now.Year()-2, time.January, 1, 0, 0, 0, 0, loc)
	if from.Before(cutoff) {
		if err = r.store.RollupRange(ctx, from, cutoff, timezone); err != nil {
			return err
		}
	}
	return r.store.FreezeSourceBefore(ctx, "usage", cutoff, timezone)
}

// BeginUsageTask covers queueing and execution; completing a task never repairs
// a previously reported usage gap. It does not block on database I/O.
func BeginUsageTask() func() {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	defaultRuntime.RUnlock()
	if r == nil {
		return func() {}
	}
	r.mu.Lock()
	r.pendingUsage++
	r.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { r.mu.Lock(); r.pendingUsage--; r.mu.Unlock() }) }
}
func ReportUsageGapBestEffort(reason string) {
	defaultRuntime.RLock()
	r := defaultRuntime.recorder
	defaultRuntime.RUnlock()
	if r == nil {
		return
	}
	r.mu.Lock()
	r.usageCoverage = r.usageCoverage.MarkGap(time.Now(), reason)
	r.mu.Unlock()
	// The independent heartbeat retries persistence; no lossy notification channel
	// sits between a dropped task and the in-memory completeness downgrade.
}
