package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/google/uuid"
)

// batchImageItemFact is the durable identity of one actual provider generation
// unit. Polling, settlement retries, downloads and cancellation requests do not
// create additional calls.
func batchImageItemFact(job *BatchImageJob, account *Account, item CreateBatchImageItemParams, statisticalAt time.Time) insights.CallFact {
	if item.IndexedAt != nil && !item.IndexedAt.IsZero() {
		statisticalAt = item.IndexedAt.UTC()
	}
	if statisticalAt.IsZero() {
		statisticalAt = time.Now().UTC()
	}
	startedAt := job.CreatedAt
	if startedAt.IsZero() {
		startedAt = statisticalAt
	}
	uid := job.UserID
	var apiKeyID *int64
	if job.APIKeyID != nil {
		v := *job.APIKeyID
		apiKeyID = &v
	}
	platform := ""
	if account != nil {
		platform = strings.TrimSpace(account.Platform)
	}
	call := insights.NewCall(insights.Identity{
		CallID:    uuid.NewSHA1(uuid.NameSpaceOID, []byte("batch-image:"+job.BatchID+":"+item.CustomID)).String(),
		RequestID: "batch-image:" + job.BatchID + ":" + item.CustomID,
		UserID:    &uid,
		APIKeyID:  apiKeyID,
		Platform:  platform,
		Model:     strings.TrimSpace(job.Model),
		Transport: insights.TransportHTTPSync,
	}, startedAt, func() time.Time { return statisticalAt })
	if item.Status == BatchImageItemStatusSuccess {
		return call.FinishSuccess(nil)
	}
	outcome := insights.OutcomeError
	errorType := batchImageDerefString(item.ErrorCode)
	if item.Status == BatchImageItemStatusCancelled {
		outcome = insights.OutcomeCancelled
	}
	if errorType == "" {
		errorType = "batch_image_item_failed"
	}
	return call.FinishFailure(outcome, errorType, "")
}

func recordBatchImageItemFactsBestEffort(job *BatchImageJob, account *Account, items []CreateBatchImageItemParams, statisticalAt time.Time, recorders ...func(insights.CallFact)) {
	if job == nil {
		return
	}
	record := func(f insights.CallFact) { insights.RecordBestEffort(f) }
	if len(recorders) > 0 && recorders[0] != nil {
		record = recorders[0]
	}
	for _, item := range items {
		record(batchImageItemFact(job, account, item, statisticalAt))
	}
}

func recordTerminalBatchImageFactsBestEffort(ctx context.Context, repo BatchImageRepository, job *BatchImageJob, account *Account, status, errorCode string, statisticalAt time.Time, recorders ...func(insights.CallFact)) {
	if repo == nil || job == nil {
		return
	}
	for offset := 0; ; {
		items, err := repo.ListBatchImageItems(ctx, job.BatchID, BatchImageItemFilter{Limit: 500, Offset: offset})
		if err != nil {
			insights.ReportGapBestEffort("batch image terminal items unavailable: " + job.BatchID)
			return
		}
		recordBatchImageItemFactsBestEffort(job, account, batchImageItemsAsFactParams(items, status, errorCode, statisticalAt), statisticalAt, recorders...)
		if len(items) < 500 {
			return
		}
		offset += len(items)
	}
}

func batchImageItemsAsFactParams(items []*BatchImageItem, fallbackStatus, errorCode string, statisticalAt time.Time) []CreateBatchImageItemParams {
	out := make([]CreateBatchImageItemParams, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		status := item.Status
		if status == BatchImageItemStatusPending || strings.TrimSpace(status) == "" {
			status = fallbackStatus
		}
		code := item.ErrorCode
		if code == nil && errorCode != "" {
			code = batchImageStringPtr(errorCode)
		}
		at := item.IndexedAt
		if at == nil {
			at = &statisticalAt
		}
		out = append(out, CreateBatchImageItemParams{JobID: item.JobID, CustomID: item.CustomID, Status: status, ImageCount: item.ImageCount, ErrorCode: code, IndexedAt: at})
	}
	return out
}

func batchImageGenerationStatisticalAt(ctx context.Context, repo BatchImageRepository, job *BatchImageJob, fallback time.Time) time.Time {
	if repo == nil || job == nil {
		return fallback
	}
	latest := time.Time{}
	for offset := 0; ; {
		items, err := repo.ListBatchImageItems(ctx, job.BatchID, BatchImageItemFilter{Limit: 500, Offset: offset})
		if err != nil {
			return fallback
		}
		for _, item := range items {
			if item != nil && item.IndexedAt != nil && item.IndexedAt.After(latest) {
				latest = *item.IndexedAt
			}
		}
		if len(items) < 500 {
			break
		}
		offset += len(items)
	}
	if latest.IsZero() {
		return fallback
	}
	return latest
}
