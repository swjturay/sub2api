package service

import (
	"net/http"
	"strconv"
	"strings"
)

// Preserve upstream safety hints. Header presence (even enabled=false) is
// distinct from the parsed value; a faster-model hint is not a serving model.
const (
	openAICodexSafetyBufferingEnabledHeader     = "x-codex-safety-buffering-enabled"
	openAICodexSafetyBufferingFasterModelHeader = "x-codex-safety-buffering-faster-model"
	maxUsageSafetyBufferingModelLen             = 128
)

var openAICodexSafetyBufferingHeaders = [...]string{
	openAICodexSafetyBufferingEnabledHeader,
	openAICodexSafetyBufferingFasterModelHeader,
}

// codexHeaderValuesFold 取 h 里与 name 大小写无关匹配的全部值；上游响应头经 net/http 已规范化，
// 手工构造的 http.Header 可能不是，两种都认。
func codexHeaderValuesFold(h http.Header, name string) []string {
	var out []string
	for key, values := range h {
		if strings.EqualFold(key, name) {
			out = append(out, values...)
		}
	}
	return out
}

// relayOpenAICodexSafetyBufferingHeaders 把上游的两个头原样写到 dst；上游缺失时清除 dst 上
// 可能残留的上一 failover attempt 的值（与 turn-state 同一套理由）。
func relayOpenAICodexSafetyBufferingHeaders(dst http.Header, upstream http.Header) {
	if dst == nil {
		return
	}
	for _, name := range openAICodexSafetyBufferingHeaders {
		key := http.CanonicalHeaderKey(name)
		dst.Del(key)
		for _, v := range codexHeaderValuesFold(upstream, name) {
			dst.Add(key, v)
		}
	}
}

// stageOpenAICodexSafetyBufferingHeaders 与 stageOpenAICodexTurnState 同型：暂存到首输出守卫
// 延迟提交的响应头集合。既有限制同 turn-state：keepalive（默认 10s）先于首个语义输出写出
// 注释帧时响应头已提交，暂存值不再发给客户端；落库不受影响。
func stageOpenAICodexSafetyBufferingHeaders(dst *http.Header, upstream http.Header) {
	if dst == nil {
		return
	}
	if *dst == nil {
		present := false
		for _, name := range openAICodexSafetyBufferingHeaders {
			if len(codexHeaderValuesFold(upstream, name)) > 0 {
				present = true
				break
			}
		}
		if !present {
			return
		}
		*dst = http.Header{}
	}
	relayOpenAICodexSafetyBufferingHeaders(*dst, upstream)
}

// usageCodexSafetyBufferingEnabledPtr 从上游响应头取 enabled 读数写进使用记录；取不到或
// 不是布尔字面量返回 nil（列保持 NULL）。
func usageCodexSafetyBufferingEnabledPtr(h http.Header) *bool {
	values := codexHeaderValuesFold(h, openAICodexSafetyBufferingEnabledHeader)
	if len(values) == 0 {
		return nil
	}
	v, err := strconv.ParseBool(strings.TrimSpace(values[0]))
	if err != nil {
		return nil
	}
	return &v
}

// usageCodexSafetyBufferingFasterModelPtr 从上游响应头取 faster-model 读数；空值返回 nil。
func usageCodexSafetyBufferingFasterModelPtr(h http.Header) *string {
	values := codexHeaderValuesFold(h, openAICodexSafetyBufferingFasterModelHeader)
	if len(values) == 0 {
		return nil
	}
	v := strings.TrimSpace(values[0])
	if v == "" {
		return nil
	}
	// 列是 TEXT：截断不切多字节字符，短值里的非法字节也剔掉，否则整行 INSERT 被 PostgreSQL 拒绝。
	v = strings.ReplaceAll(strings.ToValidUTF8(truncateUTF8(v, maxUsageSafetyBufferingModelLen), ""), "\x00", "")
	return &v
}
