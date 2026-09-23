package insights

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrModelProfileConflict = errors.New("model profile version conflict")

type ModelDisplayIdentity struct {
	Platform    string `json:"platform"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type Capability string

const (
	CapabilitySupported   Capability = "supported"
	CapabilityUnsupported Capability = "unsupported"
	CapabilityUnknown     Capability = "unknown"
)

type ProfileSource struct {
	Label     string     `json:"label"`
	URL       string     `json:"url"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (s *ProfileSource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Label     string  `json:"label"`
		URL       string  `json:"url"`
		UpdatedAt *string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Label, s.URL, s.UpdatedAt = raw.Label, raw.URL, nil
	if raw.UpdatedAt == nil || strings.TrimSpace(*raw.UpdatedAt) == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		if parsed, err := time.Parse(layout, *raw.UpdatedAt); err == nil {
			s.UpdatedAt = &parsed
			return nil
		}
	}
	return fmt.Errorf("invalid source updated_at")
}

type ModelProfile struct {
	Description      *string         `json:"description"`
	UseCases         []string        `json:"use_cases"`
	ContextLimit     *int64          `json:"context_limit"`
	MaxOutput        *int64          `json:"max_output"`
	InputModalities  []string        `json:"input_modalities"`
	OutputModalities []string        `json:"output_modalities"`
	Reasoning        Capability      `json:"reasoning"`
	ToolCalling      Capability      `json:"tool_calling"`
	StructuredOutput Capability      `json:"structured_output"`
	Sources          []ProfileSource `json:"sources"`
	UpdatedAt        *time.Time      `json:"updated_at"`
	Version          int64           `json:"version"`
}

type ModelProfileInput struct {
	Description      *string         `json:"description"`
	UseCases         []string        `json:"use_cases"`
	ContextLimit     *int64          `json:"context_limit"`
	MaxOutput        *int64          `json:"max_output"`
	InputModalities  []string        `json:"input_modalities"`
	OutputModalities []string        `json:"output_modalities"`
	Reasoning        Capability      `json:"reasoning"`
	ToolCalling      Capability      `json:"tool_calling"`
	StructuredOutput Capability      `json:"structured_output"`
	Sources          []ProfileSource `json:"sources"`
	ExpectedVersion  int64           `json:"expected_version"`
}

type ReferencePriceItem struct {
	Kind      string  `json:"kind"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Unit      string  `json:"unit"`
	Condition string  `json:"condition,omitempty"`
}

type ReferencePricing struct {
	Status    string               `json:"status"`
	Source    string               `json:"source,omitempty"`
	UpdatedAt *time.Time           `json:"updated_at"`
	Items     []ReferencePriceItem `json:"items"`
}

type ModelPerformance struct {
	AverageTPM      *float64 `json:"average_tpm"`
	AverageRPM      *float64 `json:"average_rpm"`
	AverageTTFTMS   *float64 `json:"average_ttft_ms"`
	EstimatedTPOTMS *float64 `json:"estimated_tpot_ms"`
	TPMSamples      int64    `json:"tpm_samples"`
	RPMSamples      int64    `json:"rpm_samples"`
	TTFTSamples     int64    `json:"ttft_samples"`
	TPOTSamples     int64    `json:"tpot_samples"`
}

type ModelTrendPoint struct {
	At          time.Time        `json:"at"`
	Complete    bool             `json:"complete"`
	Performance ModelPerformance `json:"performance"`
}

type ModelView struct {
	Identity         ModelIdentity     `json:"identity"`
	Configured       bool              `json:"configured"`
	Profile          ModelProfile      `json:"profile"`
	ReferencePricing ReferencePricing  `json:"reference_pricing"`
	Performance      ModelPerformance  `json:"performance"`
	Trend            []ModelTrendPoint `json:"trend"`
}

func NormalizeModelKey(id ModelIdentity) string {
	return strings.ToLower(strings.TrimSpace(id.Platform)) + ":" + strings.ToLower(strings.TrimSpace(id.Name))
}
