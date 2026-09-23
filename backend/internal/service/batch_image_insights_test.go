package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/stretchr/testify/require"
)

func TestBatchImageItemFactIsStableAcrossWorkerRetry(t *testing.T) {
	apiKeyID := int64(9)
	submittedAt := time.Date(2026, 9, 20, 1, 2, 3, 0, time.UTC)
	terminalAt := submittedAt.Add(2 * time.Hour)
	job := &BatchImageJob{BatchID: "imgbatch_stable", UserID: 7, APIKeyID: &apiKeyID, Model: "imagen-3", CreatedAt: submittedAt.Add(-time.Minute), SubmittedAt: &submittedAt}
	account := &Account{Platform: "gemini"}
	item := CreateBatchImageItemParams{JobID: job.BatchID, CustomID: "item-1", Status: BatchImageItemStatusSuccess, ImageCount: 1, IndexedAt: &terminalAt}

	first := batchImageItemFact(job, account, item, terminalAt)
	retry := batchImageItemFact(job, account, item, terminalAt)
	require.Equal(t, first.CallID, retry.CallID)
	require.Equal(t, first.RequestID, retry.RequestID)
	require.Equal(t, terminalAt, first.StatisticalAt)
	require.Equal(t, insights.OutcomeSuccess, first.Outcome)
	require.Equal(t, 0, first.AttemptCount)
	require.Nil(t, first.GatewayPreForward)
	require.Nil(t, first.ModelDuration)
	require.Equal(t, int64(7), *first.UserID)
	require.Equal(t, apiKeyID, *first.APIKeyID)
	require.Equal(t, "gemini", first.Platform)
	require.Equal(t, "imagen-3", first.Model)
}

func TestBatchImageItemFactPreservesFailedUnitOwnership(t *testing.T) {
	apiKeyID := int64(12)
	at := time.Date(2026, 9, 21, 4, 5, 6, 0, time.UTC)
	job := &BatchImageJob{BatchID: "imgbatch_failed", UserID: 11, APIKeyID: &apiKeyID, Model: "imagen", CreatedAt: at.Add(-time.Hour)}
	item := CreateBatchImageItemParams{JobID: job.BatchID, CustomID: "bad-item", Status: BatchImageItemStatusFailed, ErrorCode: batchImageStringPtr("SAFETY_BLOCKED")}
	fact := batchImageItemFact(job, &Account{Platform: "vertex"}, item, at)
	require.Equal(t, insights.OutcomeError, fact.Outcome)
	require.Equal(t, "SAFETY_BLOCKED", fact.ErrorType)
	require.Equal(t, int64(11), *fact.UserID)
	require.Equal(t, at, fact.StatisticalAt)
}

func TestBatchImageItemFactLateRetryKeepsPersistedIndexedDay(t *testing.T) {
	apiKeyID := int64(9)
	indexedAt := time.Date(2026, 9, 20, 23, 59, 0, 0, time.UTC)
	retryAt := indexedAt.Add(26 * time.Hour)
	job := &BatchImageJob{BatchID: "imgbatch_late", UserID: 7, APIKeyID: &apiKeyID, Model: "imagen", CreatedAt: indexedAt.Add(-time.Hour)}
	item := CreateBatchImageItemParams{JobID: job.BatchID, CustomID: "item", Status: BatchImageItemStatusSuccess, IndexedAt: &indexedAt}
	fact := batchImageItemFact(job, &Account{Platform: "vertex"}, item, retryAt)
	require.Equal(t, indexedAt, fact.StatisticalAt)
	require.Equal(t, 0, fact.AttemptCount)
	require.Nil(t, fact.GatewayPreForward)
	require.Nil(t, fact.ModelDuration)
}
