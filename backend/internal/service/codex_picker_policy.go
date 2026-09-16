package service

import "encoding/json"

// Picker preferences do not rewrite inference IDs or remove resume metadata.
// Only hide a duplicate when its preferred entry is already visible to this group.
var codexPickerPreferredModels = map[string]string{
	"gpt-6":                  "gpt-6-astra",
	"gpt-5.6":                "gpt-5.6-sol",
	"gemini-3.1-pro-high":    "gemini-3.1-pro",
	"gemini-3.1-pro-preview": "gemini-3.1-pro",
}

var codexPickerDedicatedModels = map[string]bool{
	"chat_20706":                  true,
	"chat_23310":                  true,
	"tab_flash_lite_preview":      true,
	"tab_jump_flash_lite_preview": true,
}

// FilterCodexPickerManifest applies presentation policy after group filtering,
// never to the shared upstream cache. Unknown fields and model order survive.
func FilterCodexPickerManifest(body []byte) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	var models []json.RawMessage
	if err := json.Unmarshal(envelope["models"], &models); err != nil {
		return nil, err
	}
	type entry struct {
		Slug       string `json:"slug"`
		Visibility string `json:"visibility"`
		Supported  bool   `json:"supported_in_api"`
	}
	entries := make([]entry, len(models))
	visible := make(map[string]bool, len(models))
	for i, raw := range models {
		if err := json.Unmarshal(raw, &entries[i]); err != nil {
			continue
		}
		e := entries[i]
		visible[e.Slug] = visible[e.Slug] || (e.Visibility == "list" && e.Supported)
	}
	changed := false
	for i, e := range entries {
		if e.Visibility != "list" || (!codexPickerDedicatedModels[e.Slug] && !visible[codexPickerPreferredModels[e.Slug]]) {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(models[i], &fields); err != nil {
			return nil, err
		}
		fields["visibility"] = json.RawMessage(`"hide"`)
		raw, err := json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		models[i] = raw
		changed = true
	}
	if !changed {
		return body, nil
	}
	raw, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	envelope["models"] = raw
	return json.Marshal(envelope)
}
