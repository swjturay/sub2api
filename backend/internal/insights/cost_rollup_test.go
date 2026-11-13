package insights

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCostRollupRangeUsesCalendarMonths(t *testing.T) {
	from, to, err := CostRollupRange(time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if from.Format("2006-01-02") != "2025-11-01" || to.Format("2006-01-02") != "2026-12-01" {
		t.Fatalf("range=%s..%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
}

func TestRefreshCostMonthlyUsageSkipsWhenAnotherReplicaOwnsLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectQuery("pg_try_advisory_xact_lock").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectRollback()
	store := NewStore(db)
	err = store.RefreshCostMonthlyUsage(context.Background(), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
