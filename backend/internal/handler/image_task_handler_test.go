package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/insights"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type asyncImageMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*service.ImageTaskRecord
}

func (s *asyncImageMemoryStore) Save(_ context.Context, task *service.ImageTaskRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	s.tasks[task.ID] = &copy
	return nil
}

func (s *asyncImageMemoryStore) Get(_ context.Context, id string) (*service.ImageTaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task := s.tasks[id]
	if task == nil {
		return nil, service.ErrImageTaskNotFound
	}
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	return &copy, nil
}

func TestAsyncImageHandlerSubmitAndPoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	release := make(chan struct{})
	capturedCall := make(chan *insights.Call, 1)
	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		call, ok := insights.CallFromContext(c.Request.Context())
		require.True(t, ok)
		call.UpdateIdentity(insights.Identity{Model: "gpt-image-1", Platform: service.PlatformOpenAI})
		call.MarkUpstreamSend(time.Now())
		insightsFinishSuccess(c, "gpt-image-1", 0, nil)
		capturedCall <- call
		<-release
		c.JSON(http.StatusOK, gin.H{"created": 123, "data": []gin.H{{"url": "https://example.test/image.png"}}})
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`)).WithContext(requestCtx)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "async-client-id"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "3", w.Header().Get("Retry-After"))

	var accepted struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		PollURL string `json:"poll_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.Equal(t, service.ImageTaskStatusProcessing, accepted.Status)
	require.Equal(t, "/v1/images/tasks/"+accepted.TaskID, accepted.PollURL)
	require.Equal(t, accepted.PollURL, w.Header().Get("Location"))

	// The detached background request must survive completion of/cancellation
	// from the short submission request.
	cancelRequest()
	call := <-capturedCall
	close(release)
	require.Eventually(t, func() bool {
		got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		return err == nil && got.Status == service.ImageTaskStatusCompleted
	}, time.Second, 10*time.Millisecond)
	fact, finalized := call.Finalized()
	require.True(t, finalized)
	require.Equal(t, insights.OutcomeSuccess, fact.Outcome)
	require.Equal(t, 1, fact.AttemptCount)
	require.Equal(t, "client:async-client-id", fact.RequestID)
	require.Equal(t, "async-client-id", fact.ClientRequestID)
	require.Equal(t, asyncImageCallID(accepted.TaskID), fact.CallID)

	pollReq := httptest.NewRequest(http.MethodGet, accepted.PollURL, nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Equal(t, "no-store", pollWriter.Header().Get("Cache-Control"))
	require.Empty(t, pollWriter.Header().Get("Retry-After"))
	require.Contains(t, pollWriter.Body.String(), "https://example.test/image.png")
}

// When object storage is not configured the feature is fully disabled: the
// endpoints must return 404 without creating a task or writing to Redis.
func TestAsyncImageHandlerDisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithOptions(store, time.Hour, time.Minute) // enabled == false
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not enabled")

	pollReq := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/imgtask_missing", nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusNotFound, pollWriter.Code)

	// No task was created / persisted.
	require.Empty(t, store.tasks)
}

func TestAsyncImageRunMergesRetriesIntoOneFailedTaskCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	created, err := tasks.Create(context.Background(), service.ImageTaskOwner{UserID: 17, APIKeyID: 19})
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-1"}`))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	uid, kid := int64(17), int64(19)
	call := insights.NewCall(insights.Identity{CallID: asyncImageCallID(created.ID), RequestID: "local:task", UserID: &uid, APIKeyID: &kid, Model: "gpt-image-1", Platform: service.PlatformOpenAI, Transport: insights.TransportHTTPSync}, time.Unix(created.CreatedAt, 0), nil)
	c.Request = c.Request.WithContext(insights.WithCall(c.Request.Context(), call))

	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		current, ok := insights.CallFromContext(c.Request.Context())
		require.True(t, ok)
		current.MarkUpstreamSend(time.Now())
		current.MarkUpstreamSend(time.Now())
		insightsFinishFailure(c, insights.OutcomeError, "upstream_error")
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "upstream_error", "message": "failed"}})
	}
	_, cancel := context.WithCancel(context.Background())
	h.run(created.ID, service.PlatformOpenAI, c, recorder, cancel)

	fact, ok := call.Finalized()
	require.True(t, ok)
	require.Equal(t, insights.OutcomeError, fact.Outcome)
	require.Equal(t, 2, fact.AttemptCount)
	require.Equal(t, asyncImageCallID(created.ID), fact.CallID)
	got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 17, APIKeyID: 19}, created.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusFailed, got.Status)
}

func TestAsyncImageRunHTTP200WithoutProtocolTerminalMarksFactUnknownButKeepsTaskResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	created, err := tasks.Create(context.Background(), service.ImageTaskOwner{UserID: 27, APIKeyID: 29})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-1"}`))
	uid, kid := int64(27), int64(29)
	call := insights.NewCall(insights.Identity{CallID: asyncImageCallID(created.ID), UserID: &uid, APIKeyID: &kid, Model: "gpt-image-1", Transport: insights.TransportHTTPSync}, time.Unix(created.CreatedAt, 0), nil)
	c.Request = c.Request.WithContext(insights.WithCall(c.Request.Context(), call))

	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"error": gin.H{"type": "upstream_error", "message": "encoded behind HTTP 200"}})
	}
	_, cancel := context.WithCancel(context.Background())
	h.run(created.ID, service.PlatformOpenAI, c, recorder, cancel)

	fact, ok := call.Finalized()
	require.True(t, ok)
	require.Equal(t, insights.OutcomeError, fact.Outcome)
	require.Equal(t, "terminal_unobserved", fact.ErrorType)
	got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 27, APIKeyID: 29}, created.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusCompleted, got.Status, "collector uncertainty must not change the existing async task API behavior")
}
