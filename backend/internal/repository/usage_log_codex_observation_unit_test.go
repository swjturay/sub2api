//go:build unit

package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogCodexObservationSQLRoundTrip(t *testing.T) {
	disabled := false
	observation := &service.CodexObservation{Transport: "http", Safety: &service.CodexSafetyObservation{EnabledPresent: true, Enabled: &disabled}, Route: &service.CodexRouteObservation{OutboundDigest: "v1:out", ResponseDigest: "v1:response", ResponseGatewayHint: "unified-123"}}
	for _, o := range []*service.CodexObservation{nil, observation, {Transport: "websocket_unobserved"}} {
		log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, Model: "gpt-6-astra", CodexObservation: o}
		prepared := prepareUsageLogInsert(log)
		require.Len(t, prepared.args, len(usageLogInsertArgTypes))
		cols := strings.Split(usageLogSelectColumns, ", ")
		values := []driver.Value{int64(99)}
		for _, arg := range prepared.args {
			v, err := driver.DefaultParameterConverter.ConvertValue(arg)
			require.NoError(t, err)
			values = append(values, v)
		}
		require.Len(t, values, len(cols))
		db, mock := newSQLMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(cols).AddRow(values...))
		rows, err := db.QueryContext(context.Background(), "SELECT "+usageLogSelectColumns)
		require.NoError(t, err)
		require.True(t, rows.Next())
		got, err := scanUsageLog(rows)
		require.NoError(t, err)
		require.NoError(t, rows.Close())
		require.Equal(t, o, got.CodexObservation)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestUsageLogCodexObservationBatchWiring(t *testing.T) {
	prepared := prepareUsageLogInsert(&service.UsageLog{RequestID: "obs-batch", Model: "gpt-6-astra", CodexObservation: &service.CodexObservation{Transport: "http"}})
	key := usageLogBatchKey("obs-batch", 0)
	query, args := buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: prepared})
	require.Contains(t, query, "codex_observation")
	require.Len(t, args, len(prepared.args)+1)
	query, args = buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.Contains(t, query, "codex_observation")
	// Best-effort has no synthetic input_index.
	require.Len(t, args, len(prepared.args))
	found := false
	for _, v := range args {
		if raw, ok := v.(string); ok && strings.HasPrefix(raw, "{\"transport\"") {
			var o service.CodexObservation
			require.NoError(t, json.Unmarshal([]byte(raw), &o))
			require.Equal(t, "http", o.Transport)
			found = true
		}
	}
	require.True(t, found)
}
