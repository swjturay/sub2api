package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 三条 WS 路径（passthrough / ctx_pool ingress / HTTP→WS v2）都直接驱动生产入口，
// 断言真正出站的握手头与帧字节，不在测试里复刻生产判断再调 helper。

func codexWSWireProfileConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.IngressModeDefault = OpenAIWSIngressModeCtxPool
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	return cfg
}

// codexWSStagedDialer 每次拨号交出下一条预置连接，并记录每次握手头与握手响应头。
type codexWSStagedDialer struct {
	mu        sync.Mutex
	conns     []openAIWSClientConn
	handshake http.Header
	headers   []http.Header
}

func (d *codexWSStagedDialer) Dial(_ context.Context, _ string, headers http.Header, _ string) (openAIWSClientConn, int, http.Header, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.headers = append(d.headers, cloneHeader(headers))
	if len(d.conns) == 0 {
		return nil, 0, nil, errors.New("no staged upstream connection left")
	}
	conn := d.conns[0]
	d.conns = d.conns[1:]
	return conn, 0, cloneHeader(d.handshake), nil
}

func (d *codexWSStagedDialer) Headers() []http.Header {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]http.Header(nil), d.headers...)
}

func codexWSCompletedEvent(id string) []byte {
	return []byte(`{"type":"response.completed","response":{"id":"` + id + `","model":"gpt-5.5","usage":{"input_tokens":1,"output_tokens":1}}}`)
}

func codexWSWireProfileService(cfg *config.Config) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}
}

// runCodexWSIngress 起一个入站 WS 服务端，把连接交给 ProxyResponsesWebSocketFromClient；
// 客户端逐帧发送并等待每轮的终态事件。inbound 是网关看到的入站握手头。
func runCodexWSIngress(t *testing.T, svc *OpenAIGatewayService, account *Account, inbound http.Header, frames []string) {
	t.Helper()
	serverErrCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			serverErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		req := r.Clone(r.Context())
		req.Header = req.Header.Clone()
		for name, values := range inbound {
			req.Header[name] = values
		}
		ginCtx.Request = req
		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		msgType, first, readErr := conn.Read(readCtx)
		cancel()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if msgType != coderws.MessageText {
			serverErrCh <- errors.New("unexpected first message type")
			return
		}
		serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "offline-token", first, nil)
	}))
	defer server.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	client, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	for i, frame := range frames {
		writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
		err = client.Write(writeCtx, coderws.MessageText, []byte(frame))
		cancelWrite()
		require.NoError(t, err, "turn %d", i+1)
		// 终态之前可能还有 response.metadata 等事件，读到终态为止。
		var event []byte
		for read := 0; read < 8; read++ {
			readCtx, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
			_, next, readErr := client.Read(readCtx)
			cancelRead()
			require.NoError(t, readErr, "turn %d", i+1)
			event = next
			if gjson.GetBytes(event, "type").String() == "response.completed" {
				break
			}
		}
		require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String(), "turn %d: %s", i+1, event)
	}
	_ = client.Close(coderws.StatusNormalClosure, "done")
	select {
	case serverErr := <-serverErrCh:
		if serverErr != nil {
			require.Contains(t, serverErr.Error(), "StatusNormalClosure")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("websocket ingress did not finish")
	}
}

func codexWSIngressInbound() http.Header {
	inbound := http.Header{}
	inbound.Set("User-Agent", "codex_cli_rs/0.98.0")
	inbound.Set("session-id", "S")
	inbound.Set(openAIWSTurnMetadataHeader, `{"session_id":"S","thread_id":"S"}`)
	return inbound
}

const codexWSTestFrame = `{"client_metadata":{"session_id":"S"},"type":"response.create","model":"gpt-5.5","stream":false,"input":[{"type":"message","role":"user","content":"hi"}]}`

// requireCodexWSStreamRequestStart 断言帧带 x-codex-ws-stream-request-start-ms：want 为空时
// 要求是网关在发送前盖的 unix 毫秒（十进制字符串，落在测试时间窗内），否则必须原样等于 want。
func requireCodexWSStreamRequestStart(t *testing.T, frame []byte, want string) {
	t.Helper()
	got := gjson.GetBytes(frame, "client_metadata."+codexWSStreamRequestStartKey)
	require.Equal(t, gjson.String, got.Type, "时间戳必须是字符串（HashMap<String,String>）：%s", frame)
	if want != "" {
		require.Equal(t, want, got.Str, "自带的时间戳不得改写：%s", frame)
		return
	}
	ms, err := strconv.ParseInt(got.Str, 10, 64)
	require.NoError(t, err, "十进制毫秒：%s", got.Str)
	now := time.Now().UnixMilli()
	require.True(t, ms > now-60_000 && ms <= now+1_000, "时间戳不在当前时间窗内：%d vs %d", ms, now)
}

// codexWSRealUpstream 是一个真正的 coder/websocket 上游：记录每条连接收到的原始文本帧字节，
// 每帧回一个 response.completed。只有在这里才能看到 wsjson/json.Encoder 之类编码层的效果。
type codexWSRealUpstream struct {
	mu     sync.Mutex
	frames [][]byte
	server *httptest.Server
}

func newCodexWSRealUpstream(t *testing.T) *codexWSRealUpstream {
	t.Helper()
	u := &codexWSRealUpstream{}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		for i := 1; ; i++ {
			msgType, payload, readErr := conn.Read(r.Context())
			if readErr != nil {
				return
			}
			if msgType != coderws.MessageText {
				return
			}
			u.mu.Lock()
			u.frames = append(u.frames, append([]byte(nil), payload...))
			u.mu.Unlock()
			if writeErr := conn.Write(r.Context(), coderws.MessageText, codexWSCompletedEvent(fmt.Sprintf("resp_%d", i))); writeErr != nil {
				return
			}
		}
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (u *codexWSRealUpstream) Frames() [][]byte {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([][]byte(nil), u.frames...)
}

// Dial 用生产拨号器连到真上游（忽略网关算出的 URL），握手头照常记录在 headers 里。
type codexWSRealDialer struct {
	upstream *codexWSRealUpstream
	inner    coderOpenAIWSClientDialer
	mu       sync.Mutex
	headers  []http.Header
}

func (d *codexWSRealDialer) Dial(ctx context.Context, _ string, headers http.Header, proxyURL string) (openAIWSClientConn, int, http.Header, error) {
	d.mu.Lock()
	d.headers = append(d.headers, cloneHeader(headers))
	d.mu.Unlock()
	return d.inner.Dial(ctx, "ws"+strings.TrimPrefix(d.upstream.server.URL, "http"), headers, proxyURL)
}

func requireCodexWSWireBytes(t *testing.T, frame []byte, enabled bool, marker string) {
	t.Helper()
	require.True(t, gjson.ValidBytes(frame), "%s", frame)
	if enabled {
		require.Contains(t, string(frame), marker, "双开：<>& 原字节上线（serde_json 不转义）")
		require.NotContains(t, string(frame), `\u003c`)
		require.NotContains(t, string(frame), `\u0026`)
		require.False(t, strings.HasSuffix(string(frame), "\n"), "双开：帧尾不带 json.Encoder 的换行")
		keys := topLevelKeys(t, frame)
		require.Equal(t, "type", keys[0], "%v", keys)
		requireCodexFieldOrder(t, keys, codexWantWSCreateOrder)
		requireCodexWSStreamRequestStart(t, frame, "")
		return
	}
	// 非双开钉住既有形态：wsjson 的 json.Encoder 转义 + 尾部换行，字节不变。
	require.Contains(t, string(frame), `\u003c`, "未开投影维持既有编码：%s", frame)
	require.True(t, strings.HasSuffix(string(frame), "\n"), "未开投影维持既有换行：%q", frame)
	require.False(t, gjson.GetBytes(frame, "client_metadata."+codexWSStreamRequestStartKey).Exists())
}

// 真上游连接上的帧字节：ctx_pool ingress 与 HTTP→WS v2（含预热帧）都经 lease 写出，
// 双开必须绕开 wsjson 的 json.Encoder（HTML 转义 + 换行），与 passthrough 的原始写同形。
func TestCodexDeviceWireProfileWSFrameBytesOnTheWire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const marker = "a <b> & c"
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("ingress/enabled=%v", enabled), func(t *testing.T) {
			upstream := newCodexWSRealUpstream(t)
			dialer := &codexWSRealDialer{upstream: upstream}
			cfg := codexWSWireProfileConfig()
			svc := codexWSWireProfileService(cfg)
			pool := newOpenAIWSConnPool(cfg)
			pool.setClientDialerForTest(dialer)
			svc.openaiWSPool = pool
			account := wireProfileTestAccount(enabled)
			account.Extra["openai_oauth_responses_websockets_v2_mode"] = OpenAIWSIngressModeCtxPool
			frame, err := sjson.Set(codexWSTestFrame, "instructions", marker)
			require.NoError(t, err)
			runCodexWSIngress(t, svc, account, codexWSIngressInbound(), []string{frame})
			frames := upstream.Frames()
			require.Len(t, frames, 1)
			requireCodexWSWireBytes(t, frames[0], enabled, marker)
		})
		t.Run(fmt.Sprintf("v2+prewarm/enabled=%v", enabled), func(t *testing.T) {
			upstream := newCodexWSRealUpstream(t)
			dialer := &codexWSRealDialer{upstream: upstream}
			cfg := codexWSWireProfileConfig()
			cfg.Gateway.OpenAIWS.PrewarmGenerateEnabled = true
			svc := codexWSWireProfileService(cfg)
			pool := newOpenAIWSConnPool(cfg)
			pool.setClientDialerForTest(dialer)
			svc.openaiWSPool = pool
			account := wireProfileTestAccount(enabled)
			account.Extra["openai_oauth_responses_websockets_v2_enabled"] = true
			body, err := sjson.SetBytes(wireProfileTestBody(t), "instructions", marker)
			require.NoError(t, err)
			c := newConvTestContext(t, body)
			c.Request.URL.Path = "/v1/responses"
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			frames := upstream.Frames()
			require.Len(t, frames, 2, "预热帧 + 正式帧")
			require.Equal(t, "false", gjson.GetBytes(frames[0], "generate").Raw, "首帧是 generate=false 的预热帧：%s", frames[0])
			require.False(t, gjson.GetBytes(frames[1], "generate").Exists())
			for _, raw := range frames {
				requireCodexWSWireBytes(t, raw, enabled, marker)
			}
		})
	}
}

// 补验投影完成后的发送边界；真实 ingress/v2/预热接线仍由上面的入口测试覆盖。
// 使用合法工具 schema 保留非规范空白、数字和转义，不能只比较反序列化后的对象。
func TestCodexDeviceWireProfileWSProjectedFrameBytesOnTheWire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const frameTimeout = 2 * time.Second
	const schema = `{
  "type" : "object", "properties" : {
    "value" : { "type" : "number", "default" : 1.2300e+02,
      "minimum" : -0, "maximum" : 9007199254740993,
      "description" : "\u0061\/\\\n<>&" }
  }
}`
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			account := wireProfileTestAccount(enabled)
			payload := []byte(`{"type":"response.create","model":"gpt-5.3-codex","tools":[{"type":"function","name":"wire_probe","parameters":` + schema + `}]}`)
			c := newConvTestContext(t, payload)
			payload = applyCodexWSFrameWireProfile(c, account, payload, "")
			require.Equal(t, schema, gjson.GetBytes(payload, "tools.0.parameters").Raw,
				"夹具必须在投影后仍包含待验的空白、转义与数字原文")
			expected := bytes.Clone(payload)
			if !enabled {
				var encoded bytes.Buffer
				require.NoError(t, json.NewEncoder(&encoded).Encode(json.RawMessage(payload)))
				expected = encoded.Bytes()
			}

			upstream := newCodexWSRealUpstream(t)
			ctx, cancel := context.WithTimeout(context.Background(), frameTimeout)
			t.Cleanup(cancel)
			dialer := &codexWSRealDialer{upstream: upstream}
			client, _, _, err := dialer.Dial(ctx, "", http.Header{}, "")
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })
			lease := &openAIWSConnLease{conn: newOpenAIWSConn("wire_bytes", account.ID, client, nil)}
			require.NoError(t, writeCodexWSFrame(ctx, c, account, lease, payload, frameTimeout))
			// 不能直接读 client：newOpenAIWSConn 会为 coder 连接常驻读循环（上游 0.2.5，
			// 空闲连接也要应答 ping），读权已归它，再读会得到
			// "previous message not read to completion"。改为等上游把帧记下来。
			var frames [][]byte
			require.Eventually(t, func() bool {
				frames = upstream.Frames()
				return len(frames) == 1
			}, frameTimeout, 2*time.Millisecond, "上游未在超时内记录到帧")
			require.Equal(t, expected, frames[0], "双开逐字节写出投影结果；关闭时保留原编码器行为")
		})
	}
}
