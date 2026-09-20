package service

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// These adapters keep the KLN non-turn-state wire-profile call sites source-compatible
// with the official v0.2.7 turn-state implementation. They deliberately preserve
// the official WS behavior: no provenance capture, cross-account guard, or frame rewrite.
func (s *OpenAIGatewayService) noteOpenAICodexTurnStateOrigin(_ *gin.Context, _ *Account, _ string) {
}

func (s *OpenAIGatewayService) guardOpenAICodexTurnStateValue(_ *gin.Context, _ *Account, state string) string {
	return strings.TrimSpace(state)
}

func (s *OpenAIGatewayService) noteOpenAICodexTurnStateFromWSEvent(_ *gin.Context, _ *Account, _ []byte) {
}

func (s *OpenAIGatewayService) guardOpenAICodexWSFrameTurnState(_ *gin.Context, _ *Account, payload []byte) []byte {
	return payload
}
