package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// The account mapping changes the model, not the upstream capability header.
// Apply at the final HTTP request boundary, after account/model resolution.
// Never mutate the ingress headers/body: failover may select a native account.
func applyMappedGPT55LiteCompatibility(req *http.Request, account *Account, body []byte) error {
	if req == nil || account == nil || !account.IsOpenAIOAuthLike() {
		return nil
	}
	if strings.TrimSpace(gjson.GetBytes(body, "model").String()) != "gpt-5.5" {
		return nil
	}
	if !isOpenAIResponsesLiteHeader(req.Header.Get(responsesLiteHeader)) && !isOpenAIResponsesLiteWebSocketPayload(body) {
		return nil
	}
	// The Codex non-Lite endpoint accepts additional_tools, namespaces and
	// reasoning.context=all_turns. Preserve them and all history/tool results.
	if isOpenAIResponsesLiteWebSocketPayload(body) {
		// Builders may already have aligned metadata and compressed the body.
		// Repeated bridge application must edit that finalized body, not the
		// caller's earlier projection (which can carry the original model).
		var err error
		if req.GetBody != nil {
			replay, replayErr := req.GetBody()
			if replayErr != nil {
				return replayErr
			}
			body, err = io.ReadAll(replay)
			_ = replay.Close()
			if err != nil {
				return err
			}
			if req.Header.Get("Content-Encoding") == codexRequestZstdContentEncoding {
				decoder, decodeErr := zstd.NewReader(nil)
				if decodeErr != nil {
					return decodeErr
				}
				body, err = decoder.DecodeAll(body, nil)
				decoder.Close()
				if err != nil {
					return err
				}
			}
		}
		body, err = sjson.DeleteBytes(body, "client_metadata."+responsesLiteWSMetadataKey)
		if err != nil {
			return fmt.Errorf("remove mapped GPT-5.5 Lite metadata: %w", err)
		}
		// 双开账号的请求体此时已按真客户端压缩（Content-Encoding: zstd），改完要同样压回去。
		if req.Header.Get("Content-Encoding") == codexRequestZstdContentEncoding {
			if body, err = encodeCodexZstdRequestBody(body); err != nil {
				return err
			}
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		savedBody := append([]byte(nil), body...)
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(savedBody)), nil }
	}
	req.Header.Del(responsesLiteHeader)
	return nil
}
