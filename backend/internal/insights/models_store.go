package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type ModelStore struct {
	db  *sql.DB
	loc *time.Location
}

func NewModelStore(db *sql.DB, timezone string) *ModelStore {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &ModelStore{db: db, loc: loc}
}

type profileCapabilities struct {
	ContextLimit     *int64          `json:"context_limit"`
	MaxOutput        *int64          `json:"max_output"`
	InputModalities  []string        `json:"input_modalities"`
	OutputModalities []string        `json:"output_modalities"`
	Reasoning        Capability      `json:"reasoning"`
	ToolCalling      Capability      `json:"tool_calling"`
	StructuredOutput Capability      `json:"structured_output"`
	Sources          []ProfileSource `json:"sources,omitempty"`
}

func emptyProfile() ModelProfile {
	return ModelProfile{UseCases: []string{}, InputModalities: []string{}, OutputModalities: []string{}, Sources: []ProfileSource{}, Reasoning: CapabilityUnknown, ToolCalling: CapabilityUnknown, StructuredOutput: CapabilityUnknown}
}

func (s *ModelStore) Profile(ctx context.Context, id ModelIdentity) (ModelProfile, error) {
	p := emptyProfile()
	var introduction, sourceURL, sourceLabel sql.NullString
	var useCases, capabilities []byte
	var sourceAt, updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT introduction,use_cases,capabilities,source_url,source_label,source_updated_at,expected_version,updated_at FROM insights_model_metadata WHERE lower(platform)=lower($1) AND lower(model)=lower($2)`, id.Platform, id.Name).Scan(&introduction, &useCases, &capabilities, &sourceURL, &sourceLabel, &sourceAt, &p.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if introduction.Valid {
		p.Description = &introduction.String
	}
	_ = json.Unmarshal(useCases, &p.UseCases)
	var caps profileCapabilities
	_ = json.Unmarshal(capabilities, &caps)
	p.ContextLimit, p.MaxOutput = caps.ContextLimit, caps.MaxOutput
	p.InputModalities, p.OutputModalities = caps.InputModalities, caps.OutputModalities
	p.Reasoning, p.ToolCalling, p.StructuredOutput = caps.Reasoning, caps.ToolCalling, caps.StructuredOutput
	p.Sources = caps.Sources
	if p.Reasoning == "" {
		p.Reasoning = CapabilityUnknown
	}
	if p.ToolCalling == "" {
		p.ToolCalling = CapabilityUnknown
	}
	if p.StructuredOutput == "" {
		p.StructuredOutput = CapabilityUnknown
	}
	if len(p.Sources) == 0 && (sourceURL.Valid || sourceLabel.Valid) {
		source := ProfileSource{Label: sourceLabel.String, URL: sourceURL.String}
		if sourceAt.Valid {
			source.UpdatedAt = &sourceAt.Time
		}
		p.Sources = []ProfileSource{source}
	}
	if updatedAt.Valid {
		p.UpdatedAt = &updatedAt.Time
	}
	return p, nil
}

func validCapability(v Capability) bool {
	return v == "" || v == CapabilitySupported || v == CapabilityUnsupported || v == CapabilityUnknown
}

func (s *ModelStore) PutProfile(ctx context.Context, id ModelIdentity, in ModelProfileInput, updatedBy int64) (ModelProfile, error) {
	id.Platform = strings.ToLower(strings.TrimSpace(id.Platform))
	id.Name = strings.ToLower(strings.TrimSpace(id.Name))
	if in.ExpectedVersion < 0 || !validCapability(in.Reasoning) || !validCapability(in.ToolCalling) || !validCapability(in.StructuredOutput) {
		return ModelProfile{}, ErrInvalidFilter
	}
	if in.Reasoning == "" {
		in.Reasoning = CapabilityUnknown
	}
	if in.ToolCalling == "" {
		in.ToolCalling = CapabilityUnknown
	}
	if in.StructuredOutput == "" {
		in.StructuredOutput = CapabilityUnknown
	}
	useCases, _ := json.Marshal(in.UseCases)
	capabilities, _ := json.Marshal(profileCapabilities{ContextLimit: in.ContextLimit, MaxOutput: in.MaxOutput, InputModalities: in.InputModalities, OutputModalities: in.OutputModalities, Reasoning: in.Reasoning, ToolCalling: in.ToolCalling, StructuredOutput: in.StructuredOutput, Sources: in.Sources})
	var source ProfileSource
	if len(in.Sources) > 0 {
		source = in.Sources[0]
	}
	var version int64
	var err error
	if in.ExpectedVersion == 0 {
		err = s.db.QueryRowContext(ctx, `INSERT INTO insights_model_metadata(platform,model,introduction,use_cases,capabilities,source_url,source_label,source_updated_at,expected_version,updated_by) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,1,$9) ON CONFLICT(platform,model) DO NOTHING RETURNING expected_version`, id.Platform, id.Name, in.Description, useCases, capabilities, source.URL, source.Label, source.UpdatedAt, updatedBy).Scan(&version)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE insights_model_metadata SET introduction=$3,use_cases=$4,capabilities=$5,source_url=NULLIF($6,''),source_label=NULLIF($7,''),source_updated_at=$8,expected_version=expected_version+1,updated_by=$9,updated_at=NOW() WHERE lower(platform)=lower($1) AND lower(model)=lower($2) AND expected_version=$10 RETURNING expected_version`, id.Platform, id.Name, in.Description, useCases, capabilities, source.URL, source.Label, source.UpdatedAt, updatedBy, in.ExpectedVersion).Scan(&version)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ModelProfile{}, ErrModelProfileConflict
	}
	if err != nil {
		return ModelProfile{}, err
	}
	return s.Profile(ctx, id)
}

func (s *ModelStore) DeleteProfile(ctx context.Context, id ModelIdentity, expectedVersion int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM insights_model_metadata WHERE lower(platform)=lower($1) AND lower(model)=lower($2) AND expected_version=$3`, id.Platform, id.Name, expectedVersion)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrModelProfileConflict
	}
	return nil
}
