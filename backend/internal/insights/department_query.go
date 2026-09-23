package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/lib/pq"
)

type DimensionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DepartmentDimension struct {
	Status              string            `json:"status"`
	AttributeID         *int64            `json:"attribute_id"`
	Key                 *string           `json:"key"`
	Type                *string           `json:"type"`
	Options             []DimensionOption `json:"options"`
	InvalidBindingCount int64             `json:"invalid_binding_count"`
	Detail              string            `json:"detail,omitempty"`
}

type departmentBinding struct {
	ID      int64
	Key     string
	Kind    string
	Options []DimensionOption
}

func (q *Query) resolveDepartmentBinding(ctx context.Context) (departmentBinding, DepartmentDimension, error) {
	var configured sql.NullInt64
	err := q.db.QueryRowContext(ctx, `SELECT NULLIF(value #>> '{}','')::bigint FROM insights_settings WHERE key='department_attribute_id'`).Scan(&configured)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return departmentBinding{}, DepartmentDimension{}, err
	}
	type candidate struct {
		id        int64
		key, kind string
		options   []byte
		enabled   bool
	}
	candidates := make([]candidate, 0, 2)
	var rows *sql.Rows
	if configured.Valid {
		rows, err = q.db.QueryContext(ctx, `SELECT id,key,type,options,enabled FROM user_attribute_definitions WHERE id=$1 AND deleted_at IS NULL`, configured.Int64)
	} else {
		rows, err = q.db.QueryContext(ctx, `SELECT id,key,type,options,enabled FROM user_attribute_definitions WHERE deleted_at IS NULL AND lower(key) IN ('department','dept') ORDER BY id`)
	}
	if err != nil {
		return departmentBinding{}, DepartmentDimension{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.key, &c.kind, &c.options, &c.enabled); err != nil {
			return departmentBinding{}, DepartmentDimension{}, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return departmentBinding{}, DepartmentDimension{}, err
	}
	empty := DepartmentDimension{Status: "not_configured", Options: []DimensionOption{}}
	if len(candidates) == 0 {
		if configured.Valid {
			empty.Status, empty.Detail = "invalid", "configured department attribute does not exist"
		}
		return departmentBinding{}, empty, nil
	}
	if len(candidates) != 1 {
		empty.Status, empty.Detail = "invalid", "department attribute discovery is ambiguous"
		return departmentBinding{}, empty, nil
	}
	c := candidates[0]
	id, key, kind := c.id, c.key, c.kind
	dim := DepartmentDimension{Status: "configured", AttributeID: &id, Key: &key, Type: &kind, Options: []DimensionOption{}}
	if !c.enabled {
		dim.Status, dim.Detail = "invalid", "configured department attribute is disabled"
		return departmentBinding{}, dim, nil
	}
	if kind == "multi_select" {
		dim.Status, dim.Detail = "invalid", "multi_select department attributes are not supported"
		return departmentBinding{}, dim, nil
	}
	if kind != "text" && kind != "textarea" && kind != "select" {
		dim.Status, dim.Detail = "invalid", "unsupported department attribute type"
		return departmentBinding{}, dim, nil
	}
	binding := departmentBinding{ID: id, Key: key, Kind: kind}
	if kind == "select" {
		if err := json.Unmarshal(c.options, &binding.Options); err != nil {
			dim.Status, dim.Detail = "invalid", "invalid select options"
			return departmentBinding{}, dim, nil
		}
		seen := map[string]bool{}
		normalized := make([]DimensionOption, 0, len(binding.Options)+1)
		for _, option := range binding.Options {
			option.Value = strings.TrimSpace(option.Value)
			if option.Value == "" || seen[option.Value] {
				continue
			}
			seen[option.Value] = true
			if option.Label == "" {
				option.Label = option.Value
			}
			normalized = append(normalized, option)
		}
		binding.Options = normalized
		if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_attribute_values v JOIN users u ON u.id=v.user_id AND u.deleted_at IS NULL WHERE v.attribute_id=$1 AND NULLIF(v.value,'') IS NOT NULL AND NOT(v.value=ANY($2::text[]))`, id, pq.Array(valuesOf(normalized))).Scan(&dim.InvalidBindingCount); err != nil {
			return departmentBinding{}, DepartmentDimension{}, err
		}
	} else {
		valueRows, err := q.db.QueryContext(ctx, `SELECT DISTINCT v.value FROM user_attribute_values v JOIN users u ON u.id=v.user_id AND u.deleted_at IS NULL WHERE v.attribute_id=$1 AND NULLIF(v.value,'') IS NOT NULL ORDER BY v.value`, id)
		if err != nil {
			return departmentBinding{}, DepartmentDimension{}, err
		}
		for valueRows.Next() {
			var value string
			if err := valueRows.Scan(&value); err != nil {
				valueRows.Close()
				return departmentBinding{}, DepartmentDimension{}, err
			}
			binding.Options = append(binding.Options, DimensionOption{Value: value, Label: value})
		}
		if err := valueRows.Close(); err != nil {
			return departmentBinding{}, DepartmentDimension{}, err
		}
	}
	sort.SliceStable(binding.Options, func(i, j int) bool {
		return strings.ToLower(binding.Options[i].Label) < strings.ToLower(binding.Options[j].Label)
	})
	dim.Options = append([]DimensionOption{{Value: "__unassigned__", Label: "未分配"}}, binding.Options...)
	return binding, dim, nil
}

func valuesOf(options []DimensionOption) []string {
	out := make([]string, 0, len(options))
	for _, option := range options {
		out = append(out, option.Value)
	}
	return out
}

func (q *Query) DepartmentDimension(ctx context.Context) (DepartmentDimension, error) {
	_, dim, err := q.resolveDepartmentBinding(ctx)
	return dim, err
}

func departmentLabel(binding departmentBinding, value string) string {
	if value == "__unassigned__" || value == "" {
		return "未分配"
	}
	for _, option := range binding.Options {
		if option.Value == value && option.Label != "" {
			return option.Label
		}
	}
	return value
}
