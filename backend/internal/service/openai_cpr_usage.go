package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// CPRUsageObservation describes evidence, independently of request outcome.
// It is admin-only telemetry; billing always uses the separately captured Usage.
type CPRUsageObservation struct {
	Status        string `json:"status"` // complete, partial, unknown; nil is historical/unobserved
	TerminalEvent string `json:"terminal_event,omitempty"`
}

type cprUsageContextKey struct{}

type cprUsageCapture struct {
	mu            sync.Mutex
	started       bool
	streaming     bool
	usage         OpenAIUsage
	observed      bool
	terminal      string
	terminalUsage bool
	responseID    string
	requestID     string
	requestBody   []byte
	header        http.Header
	observation   *CodexObservation
}

func (o *cprUsageCapture) observe(payload []byte, event string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	event = effectiveOpenAISSEEventType(payload, event)
	terminal := openAIStreamEventTypeIsTerminal(event) || event == "error"
	if event == "" && !o.streaming {
		status := gjson.GetBytes(payload, "status").String()
		if status == "completed" || status == "failed" || status == "incomplete" {
			event, terminal = "response."+status, true
		}
		// Some Responses-compatible JSON gateways omit status; a finished JSON
		// response with its own usage is still a final measurement.
		if status == "" && gjson.GetBytes(payload, "id").String() != "" {
			terminal = true
		}
	}
	parsed, hasUsage, complete := extractCPRMeasuredUsage(payload)
	if hasUsage {
		o.observed = true
		if terminal && complete {
			o.usage = parsed
		} else {
			mergeOpenAIUsageNonZero(&o.usage, parsed)
		}
	}
	if terminal {
		if event == "" {
			event = "response.completed"
		}
		o.terminal = event
		o.terminalUsage = hasUsage && complete
	}
	if id := extractOpenAIResponseIDFromJSONBytes(payload); id != "" {
		o.responseID = id
	}
}

func (o *cprUsageCapture) snapshot() (OpenAIUsage, *CPRUsageObservation) {
	o.mu.Lock()
	defer o.mu.Unlock()
	status := "unknown"
	if o.observed {
		status = "partial"
	}
	if o.terminalUsage {
		status = "complete"
	}
	return o.usage, &CPRUsageObservation{Status: status, TerminalEvent: o.terminal}
}

func (s *OpenAIGatewayService) beginCPRForward(ctx context.Context, c *gin.Context, account *Account, body []byte) (context.Context, func(**OpenAIForwardResult, *error)) {
	if !account.IsCPR() {
		return ctx, func(**OpenAIForwardResult, *error) {}
	}
	start := time.Now()
	billingRequestID := "cpr:" + generateRequestID()
	capture := &cprUsageCapture{}
	ctx = context.WithValue(ctx, cprUsageContextKey{}, capture)
	return ctx, func(result **OpenAIForwardResult, returnErr *error) {
		capture.mu.Lock()
		started, requestID, responseID := capture.started, capture.requestID, capture.responseID
		sentBody, headers, observation := capture.requestBody, capture.header, capture.observation
		streaming := capture.streaming
		capture.mu.Unlock()
		if !started {
			return
		}
		usage, measured := capture.snapshot()
		if *result == nil {
			requested := gjson.GetBytes(body, "model").String()
			sent := gjson.GetBytes(sentBody, "model").String()
			*result = &OpenAIForwardResult{Model: requested, BillingModel: sent,
				Usage: usage, RequestID: requestID, ResponseID: responseID, UpstreamHeaders: headers,
				Stream: gjson.GetBytes(body, "stream").Bool(), Duration: time.Since(start),
				ServiceTier:     resolvedOpenAIUpstreamServiceTier(c, extractOpenAIServiceTierFromBody(sentBody)),
				ReasoningEffort: extractOpenAIReasoningEffortFromBody(sentBody, sent)}
			if sent != requested {
				(*result).UpstreamModel = sent
			}
		}
		res := *result
		res.BillingRequestID = billingRequestID
		if requestID != "" && res.RequestID == "" {
			res.RequestID = requestID
		}
		if res.RequestID == "" {
			res.RequestID = "generated:" + generateRequestID()
		}
		// Already-received measurements survive failed client writes and nil-result
		// compatibility error paths. No local token estimate enters this assignment.
		res.Usage = usage
		if observation == nil {
			observation = &CodexObservation{Transport: "http"}
		}
		copied := *observation
		copied.Usage = measured
		res.CodexObservation = &copied
		res.UpstreamTerminalEvent = measured.TerminalEvent
		if ctx.Err() != nil || errors.Is(*returnErr, context.Canceled) {
			res.ClientDisconnect = true
			if *returnErr == nil {
				*returnErr = ctx.Err()
			}
		}
		if *returnErr == nil && streaming && measured.TerminalEvent == "" {
			*returnErr = errors.New("CPR stream ended before a terminal event")
		}
		if *returnErr == nil && (measured.TerminalEvent == "response.failed" || measured.TerminalEvent == "error") {
			*returnErr = errors.New("CPR upstream response failed")
		}
	}
}

// A usage object alone is not evidence: missing and malformed counters must
// remain unknown, while explicit numeric zero is a real measurement.
func validCPRUsageCounter(v gjson.Result) bool {
	return v.Type == gjson.Number && v.Float() >= 0 && v.Float() == float64(v.Int())
}

// Usage counters must be JSON integers, not numeric strings or negative values.
// The bounded shape also prevents untrusted nested usage metadata from growing
// a recursive sanitizer. Standard token details fit within these four levels.
func cprNumericUsageObject(value gjson.Result, depth int) gjson.Result {
	if !value.IsObject() || depth == 0 {
		return gjson.Result{}
	}
	fields := make(map[string]any)
	value.ForEach(func(key, v gjson.Result) bool {
		if validCPRUsageCounter(v) {
			fields[key.String()] = v.Int()
		} else if v.IsObject() {
			if nested := cprNumericUsageObject(v, depth-1); nested.IsObject() {
				fields[key.String()] = json.RawMessage(nested.Raw)
			}
		}
		return true
	})
	raw, _ := json.Marshal(fields)
	return gjson.ParseBytes(raw)
}

func extractCPRMeasuredUsage(payload []byte) (OpenAIUsage, bool, bool) {
	if !bytes.Contains(payload, []byte(`"usage"`)) || !gjson.ValidBytes(payload) {
		return OpenAIUsage{}, false, false
	}
	for _, prefix := range []string{"", "response.", "data.", "data.response."} {
		u := gjson.GetBytes(payload, prefix+"usage")
		if !u.IsObject() {
			continue
		}
		parsed, _ := openAIUsageFromGJSON(cprNumericUsageObject(u, 4))
		mergeHostedImageGenToolUsage(cprNumericUsageObject(gjson.GetBytes(payload, prefix+"tool_usage.image_gen"), 4), &parsed)
		in, out := validCPRUsageCounter(u.Get("input_tokens")), validCPRUsageCounter(u.Get("output_tokens"))
		if in || out || openAIUsageHasTokens(&parsed) {
			return parsed, true, in && out
		}
	}
	return OpenAIUsage{}, false, false
}

// attachCPRUsageCapture observes received bytes before any downstream write.
// It neither changes the bytes nor depends on the downstream protocol adapter.
func (s *OpenAIGatewayService) attachCPRUsageCapture(request *http.Request, response *http.Response) {
	capture, _ := request.Context().Value(cprUsageContextKey{}).(*cprUsageCapture)
	if capture == nil {
		return
	}
	var sent []byte
	if request.GetBody != nil {
		if body, err := request.GetBody(); err == nil {
			clone := request.Clone(request.Context())
			clone.Body = body
			sent, _ = httputil.ReadRequestBodyWithPrealloc(clone)
			_ = body.Close()
		}
	}
	capture.mu.Lock()
	capture.started, capture.requestBody = true, sent
	if response != nil {
		capture.header = response.Header.Clone()
		capture.requestID = response.Header.Get("x-request-id")
		capture.streaming = isEventStreamResponse(response.Header)
		capture.observation = codexObservationFromResponse(response)
	}
	streaming := capture.streaming
	capture.mu.Unlock()
	if response != nil && response.Body != nil {
		response.Body = &cprObservedBody{ReadCloser: response.Body, capture: capture, streaming: streaming}
	}
}

// Buffer at most one bounded observation frame. Oversized/invalid observations
// remain unknown; the upstream payload continues unchanged.
const cprObservationMaxBytes = 16 << 20

type cprObservedBody struct {
	io.ReadCloser
	readMu         sync.Mutex
	closed         bool
	capture        *cprUsageCapture
	streaming      bool
	pending        []byte
	parser         openAICompatSSEFrameParser
	oversized      bool
	frameBytes     int
	lineHasContent bool
}

func (r *cprObservedBody) Close() error {
	err := r.ReadCloser.Close()
	// Close unblocks a pending transport read; join its observation before billing.
	r.readMu.Lock()
	r.closed = true
	r.readMu.Unlock()
	return err
}

func (r *cprObservedBody) Read(p []byte) (int, error) {
	r.readMu.Lock()
	defer r.readMu.Unlock()
	if r.closed {
		return 0, io.EOF
	}
	n, err := r.ReadCloser.Read(p)
	if !r.streaming {
		if !r.oversized && len(r.pending)+n <= cprObservationMaxBytes {
			r.pending = append(r.pending, p[:n]...)
		} else {
			r.oversized = true
			r.pending = nil
		}
		if err == io.EOF && !r.oversized {
			r.capture.observe(r.pending, "")
			r.pending = nil
		}
		return n, err
	}
	data := p[:n]
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		length := len(data)
		if i >= 0 {
			length = i + 1
		}
		part := data[:length]
		if len(bytes.Trim(part, "\r\n")) > 0 {
			r.lineHasContent = true
		}
		if !r.oversized && r.frameBytes+length <= cprObservationMaxBytes {
			r.pending = append(r.pending, part...)
			r.frameBytes += length
		} else {
			r.oversized = true
			r.pending = nil
			r.parser = openAICompatSSEFrameParser{}
		}
		data = data[length:]
		if i >= 0 {
			if !r.oversized {
				if frame, ok := r.parser.AddLine(strings.TrimRight(string(r.pending), "\r\n")); ok {
					r.capture.observe([]byte(frame.Data), frame.EventType)
				}
			}
			r.pending = nil
			if !r.lineHasContent {
				r.parser = openAICompatSSEFrameParser{}
				r.oversized = false
				r.frameBytes = 0
			}
			r.lineHasContent = false
		}
	}
	if err != nil {
		if len(r.pending) > 0 && !r.oversized {
			if frame, ok := r.parser.AddLine(string(r.pending)); ok {
				r.capture.observe([]byte(frame.Data), frame.EventType)
			}
		}
		if frame, ok := r.parser.Finish(); ok && !r.oversized {
			r.capture.observe([]byte(frame.Data), frame.EventType)
		}
		r.pending = nil
	}
	return n, err
}

func openAICompatExecutionContext(ctx context.Context, account *Account) (context.Context, func()) {
	if account.IsCPR() {
		return ctx, func() {}
	}
	return detachUpstreamContext(ctx)
}
