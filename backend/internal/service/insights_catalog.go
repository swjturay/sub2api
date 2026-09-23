package service

import (
	"context"
	"sort"
	"strings"
)

// InsightsCatalogModel is the deliberately small projection exposed to Insights.
// It contains no group, channel, account, multiplier, or routing information.
type InsightsCatalogModel struct {
	Platform        string
	Name            string
	DisplayName     string
	OfficialPricing *PlazaOfficialPricing
}

// ListInsightsCatalog returns the system-wide configured model catalog.
func (s *ModelPlazaService) ListInsightsCatalog(ctx context.Context) ([]InsightsCatalogModel, error) {
	groups, err := s.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	type key struct{ platform, name string }
	seen := make(map[key]int)
	out := make([]InsightsCatalogModel, 0)
	for _, group := range groups {
		for _, model := range group.Models {
			k := key{strings.ToLower(strings.TrimSpace(model.Platform)), strings.ToLower(strings.TrimSpace(model.Name))}
			if k.platform == "" || k.name == "" {
				continue
			}
			if at, ok := seen[k]; ok {
				if out[at].OfficialPricing == nil && model.OfficialPricing != nil {
					out[at].OfficialPricing = model.OfficialPricing
				}
				continue
			}
			seen[k] = len(out)
			out = append(out, InsightsCatalogModel{Platform: model.Platform, Name: model.Name, DisplayName: model.Name, OfficialPricing: model.OfficialPricing})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Platform, out[j].Platform) {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return strings.ToLower(out[i].Platform) < strings.ToLower(out[j].Platform)
	})
	return out, nil
}
