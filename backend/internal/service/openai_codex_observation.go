package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// CodexObservation is passive telemetry, never a routing or billing input.
// HTTP observations describe one attempt. WS is explicitly unobserved: a pooled
// handshake cannot establish per-turn safety or serving-route information.
type CodexObservation struct {
	Transport string                  `json:"transport"`
	Safety    *CodexSafetyObservation `json:"safety,omitempty"`
	Route     *CodexRouteObservation  `json:"route,omitempty"`
}

type CodexSafetyObservation struct {
	EnabledPresent     bool   `json:"enabled_present"`
	FasterModelPresent bool   `json:"faster_model_present"`
	Enabled            *bool  `json:"enabled"`
	FasterModel        string `json:"faster_model,omitempty"`
}

type CodexRouteObservation struct {
	OutboundDigest      string `json:"outbound_digest,omitempty"`
	ResponseDigest      string `json:"response_digest,omitempty"`
	ResponseGatewayHint string `json:"response_gateway_hint,omitempty"`
}

type codexObservationContextKey struct{}

func codexHeaderPresent(h http.Header, name string) bool {
	for k := range h {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

func codexSafetyObservation(h http.Header) *CodexSafetyObservation {
	model := usageCodexSafetyBufferingFasterModelPtr(h)
	o := &CodexSafetyObservation{
		EnabledPresent:     codexHeaderPresent(h, openAICodexSafetyBufferingEnabledHeader),
		FasterModelPresent: codexHeaderPresent(h, openAICodexSafetyBufferingFasterModelHeader),
		Enabled:            usageCodexSafetyBufferingEnabledPtr(h),
	}
	if model != nil {
		o.FasterModel = *model
	}
	return o
}

// HMAC is scoped by account and domain-separated from JWT use. Missing config
// disables digests rather than falling back to a public/unkeyed cookie hash.
func (s *OpenAIGatewayService) codexRouteDigest(account *Account, cookies []*http.Cookie) string {
	if s.cfg == nil || s.cfg.JWT.Secret == "" || account == nil {
		return ""
	}
	values := map[string]string{}
	for _, c := range cookies {
		if c.Name == "__cflb" || c.Name == "__oailb" {
			values[c.Name] = c.Value
		}
	}
	if len(values) == 0 {
		return ""
	}
	canonical, _ := json.Marshal(values)
	mac := hmac.New(sha256.New, []byte(s.cfg.JWT.Secret))
	_, _ = mac.Write([]byte("codex-route-observation:v1:" + strconv.FormatInt(account.ID, 10) + ":"))
	_, _ = mac.Write(canonical)
	return "v1:" + hex.EncodeToString(mac.Sum(nil))
}

// This is an unverified hint from a response cookie, not the current serving
// gateway. Bound parsing and accept only the known host form; never store JWTs.
func codexResponseGatewayHint(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name != "__oailb" || len(c.Value) > 16384 {
			continue
		}
		parts := strings.Split(c.Value, ".")
		if len(parts) != 3 {
			continue
		}
		payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
		if err != nil {
			continue
		}
		var claims struct {
			Host string `json:"host"`
		}
		if json.Unmarshal(payload, &claims) != nil {
			continue
		}
		host := strings.TrimPrefix(claims.Host, "chat.gateway.")
		if host == claims.Host || !strings.HasSuffix(host, ".api.openai.com") {
			continue
		}
		gateway := strings.TrimSuffix(host, ".api.openai.com")
		id := strings.TrimPrefix(gateway, "unified-")
		if id == gateway || len(id) == 0 || len(id) > 16 {
			continue
		}
		valid := true
		for _, ch := range id {
			if ch < '0' || ch > '9' {
				valid = false
			}
		}
		if valid {
			return gateway
		}
	}
	return ""
}

func codexObservationFromResponse(resp *http.Response) *CodexObservation {
	if resp == nil || resp.Request == nil {
		return nil
	}
	o, _ := resp.Request.Context().Value(codexObservationContextKey{}).(*CodexObservation)
	return o
}

func (s *OpenAIGatewayService) observeCodexHTTPResponse(request *http.Request, account *Account, outbound string, resp *http.Response) {
	if resp == nil || account == nil || !account.UsesOpenAICodexProtocol() {
		return
	}
	cookies := resp.Cookies()
	o := &CodexObservation{
		Transport: "http",
		Safety:    codexSafetyObservation(resp.Header),
		Route: &CodexRouteObservation{OutboundDigest: outbound,
			ResponseDigest:      s.codexRouteDigest(account, cookies),
			ResponseGatewayHint: codexResponseGatewayHint(cookies)},
	}
	// The response owns its context snapshot; no shared jar or mutable Gin state
	// is read later when asynchronous usage recording runs.
	resp.Request = request.WithContext(context.WithValue(request.Context(), codexObservationContextKey{}, o))
}
