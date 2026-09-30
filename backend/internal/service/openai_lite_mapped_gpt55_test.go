package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMappedGPT55LiteCompressedBuildersOnWire(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "normal"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.5","stream":false,"input":[],"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true","keep":"yes","x-codex-turn-metadata":"{\"model\":\"gpt-6-astra\",\"reasoning_effort\":\"ultra\"}"}}`)
			c := newConvTestContext(t, body)
			c.Request.Header.Set(responsesLiteHeader, "true")
			account := wireProfileTestAccount(true)
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			var request *http.Request
			var err error
			if passthrough {
				request, err = svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "test-token")
			} else {
				request, err = svc.buildUpstreamRequest(context.Background(), c, account, body, "test-token", true, "", false)
			}
			require.NoError(t, err)
			// WS-to-HTTP invokes compatibility once more with the pre-builder body.
			require.NoError(t, applyMappedGPT55LiteCompatibility(request, account, body))
			var received []byte
			var readErr error
			var encoding, lite string
			var contentLength int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received, readErr = io.ReadAll(r.Body)
				encoding, lite, contentLength = r.Header.Get("Content-Encoding"), r.Header.Get(responsesLiteHeader), r.ContentLength
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			request.URL, err = url.Parse(server.URL)
			require.NoError(t, err)
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.NoError(t, readErr)
			require.Equal(t, "zstd", encoding)
			require.Empty(t, lite)
			require.Equal(t, int64(len(received)), contentLength)
			decoder, err := zstd.NewReader(nil)
			require.NoError(t, err)
			defer decoder.Close()
			decoded, err := decoder.DecodeAll(received, nil)
			require.NoError(t, err)
			require.False(t, isOpenAIResponsesLiteWebSocketPayload(decoded))
			require.Equal(t, "yes", gjson.GetBytes(decoded, "client_metadata.keep").String())
			metadata := gjson.GetBytes(decoded, "client_metadata.x-codex-turn-metadata").String()
			require.Equal(t, "gpt-5.5", gjson.Get(metadata, "model").String())
			require.Equal(t, "ultra", gjson.Get(metadata, "reasoning_effort").String())
			require.True(t, isOpenAIResponsesLiteWebSocketPayload(body), "failover must retain the ingress body")
			require.Equal(t, "true", c.GetHeader(responsesLiteHeader))
			replay, err := request.GetBody()
			require.NoError(t, err)
			again, err := io.ReadAll(replay)
			require.NoError(t, err)
			require.NoError(t, replay.Close())
			require.Equal(t, received, again)
		})
	}
}

func TestMappedGPT55LiteCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name     string
		id       int64
		model    string
		mapped   bool
		wantLite bool
	}{
		{"mapped", 12, "gpt-5.5", true, false},
		{"account13Mapped", 13, "gpt-5.5", true, false},
		{"account14Mapped", 14, "gpt-5.5", true, false},
		{"account13Native", 13, "gpt-6-astra", false, true},
		{"account14Native", 14, "gpt-6-astra", false, true},
		{"nativeSol", 12, "gpt-5.6-sol", true, true},
		{"noMapping", 12, "gpt-5.5", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Account{ID: tc.id, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{}}
			if tc.mapped {
				a.Credentials["model_mapping"] = map[string]any{"gpt-5.6-sol": "gpt-5.5"}
			}
			body := []byte(`{"model":"` + tc.model + `","input":[{"type":"additional_tools","role":"developer","tools":[]},{"type":"function_call_output","call_id":"call1","output":"12345678901234567890"}],"reasoning":{"context":"all_turns"},"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true","keep":"yes"}}`)
			r, err := http.NewRequest("POST", "http://localhost", bytes.NewReader(body))
			require.NoError(t, err)
			r.Header.Set(responsesLiteHeader, "true")
			require.NoError(t, applyMappedGPT55LiteCompatibility(r, a, body))
			require.Equal(t, tc.wantLite, isOpenAIResponsesLiteHeader(r.Header.Get(responsesLiteHeader)))
			got, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			if tc.wantLite {
				require.Equal(t, body, got)
			} else {
				require.False(t, isOpenAIResponsesLiteWebSocketPayload(got))
				require.Equal(t, gjson.GetBytes(body, "input").Raw, gjson.GetBytes(got, "input").Raw)
				require.Equal(t, "all_turns", gjson.GetBytes(got, "reasoning.context").String())
				require.Equal(t, "yes", gjson.GetBytes(got, "client_metadata.keep").String())
				require.Equal(t, int64(len(got)), r.ContentLength)
				replay, err := r.GetBody()
				require.NoError(t, err)
				again, err := io.ReadAll(replay)
				require.NoError(t, err)
				require.Equal(t, got, again)
			}
			require.True(t, isOpenAIResponsesLiteWebSocketPayload(body))
		})
	}
}

func TestMappedGPT55LiteBuildersPreserveIngressForFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		c.Request.Header.Set(responsesLiteHeader, "true")
		s := &OpenAIGatewayService{cfg: &config.Config{}}
		a := &Account{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.5"}}}
		body := []byte(`{"model":"gpt-5.5","stream":true,"input":[]}`)
		var r *http.Request
		var err error
		if passthrough {
			r, err = s.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, a, body, "test-token")
		} else {
			r, err = s.buildUpstreamRequest(context.Background(), c, a, body, "test-token", true, "", true)
		}
		require.NoError(t, err)
		require.Empty(t, r.Header.Get(responsesLiteHeader))
		require.Equal(t, "true", c.GetHeader(responsesLiteHeader))
		a.ID = 14
		body = []byte(`{"model":"gpt-6-astra","stream":true,"input":[]}`)
		if passthrough {
			r, err = s.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, a, body, "test-token")
		} else {
			r, err = s.buildUpstreamRequest(context.Background(), c, a, body, "test-token", true, "", true)
		}
		require.NoError(t, err)
		require.Equal(t, "true", r.Header.Get(responsesLiteHeader))
	}
}
