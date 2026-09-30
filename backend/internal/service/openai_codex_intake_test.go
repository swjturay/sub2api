//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type codexIntakeCapture struct {
	client *http.Client
	target *url.URL
	header http.Header
	body   []byte
}

func (s *codexIntakeCapture) Do(r *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.URL = s.target
	clone.Host = s.target.Host
	return s.client.Do(clone)
}
func (s *codexIntakeCapture) DoWithTLS(r *http.Request, p string, id int64, n int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(r, p, id, n)
}
func codexIntakeForward(t *testing.T, kind string, pass bool, body []byte, meta string, compact bool) ([]byte, http.Header) {
	t.Helper()
	cap := &codexIntakeCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		cap.body = codexTestDecodeUpstreamBody(r.Header, raw)
		cap.header = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(openAICodexSafetyBufferingEnabledHeader, "false")
		w.Header().Add("Set-Cookie", "__cflb=synthetic-route; Secure")
		if gjson.GetBytes(cap.body, "stream").Bool() {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"hello"}`+"\n\n")
			_, _ = fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"offline","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`+"\n\n")
		} else {
			_, _ = fmt.Fprint(w, `{"id":"offline","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)
		}
	}))
	defer server.Close()
	cap.client = server.Client()
	cap.target, _ = url.Parse(server.URL)
	c := newConvTestContext(t, body)
	if meta != "" {
		c.Request.Header.Set(openAIWSTurnMetadataHeader, meta)
	}
	if compact {
		c.Request.URL.Path = "/v1/responses/compact"
	}
	a := wireProfileTestAccount(kind != "off")
	a.Extra["openai_passthrough"] = pass
	switch kind {
	case "setup":
		a.Type = AccountTypeSetupToken
	case "third":
		c.Request.Header.Set("User-Agent", "OpenAI/Python")
		c.Request.Header.Del("originator")
	case "apikey":
		a.Type = AccountTypeAPIKey
		a.Credentials = map[string]any{"api_key": "offline"}
	case "cpr":
		a = newCPRTestAccount()
	}
	a.Credentials["model_mapping"] = map[string]any{"gpt-6-astra": "gpt-5.4"}
	svc, _ := wireProfileTestService()
	svc.httpUpstream = cap
	svc.cfg = cprTestConfig()
	svc.cfg.JWT.Secret = "synthetic-observation-key"
	result, forwardErr := svc.Forward(context.Background(), c, a, body)
	require.NotNil(t, cap.body, "request never reached server; status: %d err: %v", c.Writer.Status(), forwardErr)
	require.NoError(t, forwardErr)
	require.NotNil(t, result)
	if a.UsesOpenAICodexProtocol() {
		require.NotNil(t, result.CodexObservation)
		require.True(t, result.CodexObservation.Safety.EnabledPresent)
		require.NotNil(t, result.CodexObservation.Safety.Enabled)
		require.False(t, *result.CodexObservation.Safety.Enabled)
		require.NotEmpty(t, result.CodexObservation.Route.ResponseDigest)
	} else if a.IsCPR() {
		require.NotNil(t, result.CodexObservation)
		require.Nil(t, result.CodexObservation.Route)
		require.Equal(t, "complete", result.CodexObservation.Usage.Status)
	} else {
		require.Nil(t, result.CodexObservation)
	}
	return cap.body, cap.header
}
func TestCodexIntakeCalls(t *testing.T) {
	for _, kind := range []string{"oauth", "setup", "off", "third", "apikey", "cpr"} {
		for _, pass := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pass=%v", kind, pass), func(t *testing.T) {
				body := []byte(`{"model":"gpt-6-astra","instructions":"test","stream":false,"input":[{"type":"function_call","id":"fc_item","call_id":"call_upstream","name":"shell","arguments":"{}"},{"type":"function_call_output","call_id":"call_upstream","output":"ok"},{"type":"item_reference","id":"call_upstream"}]}`)
				out, _ := codexIntakeForward(t, kind, pass, body, "", false)
				ids := []string{gjson.GetBytes(out, "input.0.call_id").String(), gjson.GetBytes(out, "input.1.call_id").String(), gjson.GetBytes(out, "input.2.id").String()}
				t.Logf("call=%q output=%q reference=%q", ids[0], ids[1], ids[2])
				want := "fc_upstream"
				if pass || kind == "apikey" || kind == "cpr" || (kind == "oauth" || kind == "setup") {
					want = "call_upstream"
				}
				require.Equal(t, want, ids[0])
				require.Equal(t, ids[0], ids[1])
				require.Equal(t, ids[0], ids[2])
			})
		}
	}
}
func TestCodexIntakeMetadataHTTP(t *testing.T) {
	for _, kind := range []string{"oauth", "setup", "off", "apikey", "cpr"} {
		for _, pass := range []bool{false, true} {
			for _, compact := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/pass=%v/compact=%v", kind, pass, compact), func(t *testing.T) {
					meta := `{"turn_id":"turn","model":"gpt-6-astra","reasoning_effort":"ultra","unknown":9007199254740993}`
					body := []byte(`{"model":"gpt-6-astra","instructions":"test","stream":false,"input":[{"role":"user","content":"hi"}],"reasoning":{"effort":"minimal"}}`)
					body, err := sjson.SetBytes(body, "client_metadata.x-codex-turn-metadata", meta)
					require.NoError(t, err)
					out, h := codexIntakeForward(t, kind, pass, body, meta, compact)
					for name, raw := range map[string]string{"header": h.Get(openAIWSTurnMetadataHeader), "body": gjson.GetBytes(out, "client_metadata.x-codex-turn-metadata").String()} {
						if raw == "" {
							t.Logf("%s absent (existing projection)", name)
							continue
						}
						m := gjson.Parse(raw)
						want := "gpt-6-astra"
						if kind == "cpr" || (!compact && (kind == "oauth" || kind == "setup")) {
							want = gjson.GetBytes(out, "model").String()
						}
						t.Logf("%s: body model=%s metadata model=%s", name, gjson.GetBytes(out, "model").String(), m.Get("model").String())
						require.Equal(t, want, m.Get("model").String())
						require.Equal(t, "ultra", m.Get("reasoning_effort").String())
						require.Equal(t, "9007199254740993", m.Get("unknown").Raw)
					}
				})
			}
		}
	}
}
func TestCodexIntakeMetadataWS(t *testing.T) {
	for _, kind := range []string{"oauth", "setup", "off", "apikey", "cpr"} {
		for _, meta := range []string{`{"model":"old","reasoning_effort":"ultra"}`, `{"unknown":"keep"}`, `broken`} {
			t.Run(kind+"/"+meta, func(t *testing.T) {
				a := wireProfileTestAccount(kind != "off")
				if kind == "setup" {
					a.Type = AccountTypeSetupToken
				}
				if kind == "apikey" {
					a.Type = AccountTypeAPIKey
				}
				if kind == "cpr" {
					a = newCPRTestAccount()
				}
				frame := []byte(`{"type":"response.create","model":"gpt-5.4","input":[{"type":"function_call_output","call_id":"call_original","output":"ok"}]}`)
				frame, err := sjson.SetBytes(frame, "client_metadata.x-codex-turn-metadata", meta)
				require.NoError(t, err)
				c := newConvTestContext(t, frame)
				out := applyCodexWSFrameWireProfile(c, a, frame, "")
				got := gjson.GetBytes(out, "client_metadata.x-codex-turn-metadata").String()
				want := meta
				if (kind == "oauth" || kind == "setup") && gjson.Get(meta, "model").Exists() {
					want = `{"model":"gpt-5.4","reasoning_effort":"ultra"}`
				}
				require.Equal(t, want, got)
				require.Equal(t, "call_original", gjson.GetBytes(out, "input.0.call_id").String())
				t.Logf("metadata=%s call preserved", got)
			})
		}
	}
}
func TestCodexIntakeLegacyRelations(t *testing.T) {
	for _, id := range []string{"legacy123", "call_existing", "fc_existing"} {
		t.Run(id, func(t *testing.T) {
			input := []map[string]any{{"type": "function_call", "id": "fc_item", "call_id": id, "name": "shell", "arguments": "{}"}, {"type": "function_call_output", "call_id": id, "output": "ok"}, {"type": "item_reference", "id": id}}
			raw, _ := json.Marshal(map[string]any{"model": "gpt-6-astra", "instructions": "test", "input": input})
			out, _ := codexIntakeForward(t, "oauth", false, raw, "", false)
			require.Equal(t, gjson.GetBytes(out, "input.0.call_id").String(), gjson.GetBytes(out, "input.1.call_id").String())
			t.Logf("call=%s ref=%s", gjson.GetBytes(out, "input.0.call_id").String(), gjson.GetBytes(out, "input.2.id").String())
		})
	}
}
func TestCodexIntakeContinuationOnly(t *testing.T) {
	body := []byte(`{"model":"gpt-6-astra","instructions":"test","previous_response_id":"resp_prior","input":[{"type":"function_call_output","call_id":"call_server_previous","output":"ok"}]}`)
	out, _ := codexIntakeForward(t, "oauth", false, body, "", false)
	t.Logf("previous_response_id=%s output.call_id=%s", gjson.GetBytes(out, "previous_response_id").String(), gjson.GetBytes(out, "input.0.call_id").String())
	require.False(t, gjson.GetBytes(out, "previous_response_id").Exists(), "both HTTP paths are stateless")
	require.Empty(t, gjson.GetBytes(out, "input.0.call_id").String(), "orphan output removed by existing pipeline: this is not a proven call-ID regression")
}
