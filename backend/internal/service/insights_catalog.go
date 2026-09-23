package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/insights"
)

// InsightsCatalogModel is the deliberately small projection exposed to Insights.
// It contains no group, channel, account, multiplier, or routing information.
type InsightsCatalogModel struct {
	Platform        string
	Name            string
	DisplayName     string
	Profile         insights.ModelProfile
	OfficialPricing *PlazaOfficialPricing
}

// ListInsightsCatalog returns the manually curated production model catalog.
// Account mappings, channel configuration, and request history never add names
// to this list; they may only supply reference pricing for an allowed entry.
func (s *ModelPlazaService) ListInsightsCatalog(ctx context.Context) ([]InsightsCatalogModel, error) {
	official := insights.OfficialModelCatalog()
	out := make([]InsightsCatalogModel, 0, len(official))
	pricingMemo := make(map[string]*PlazaOfficialPricing, len(official))
	for _, model := range official {
		out = append(out, InsightsCatalogModel{
			Platform:        model.Platform,
			Name:            model.Name,
			DisplayName:     model.DisplayName,
			Profile:         model.Profile,
			OfficialPricing: s.lookupOfficialPricing(ctx, model.Name, pricingMemo),
		})
	}
	return out, nil
}
