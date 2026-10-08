package service

import (
	"context"
	"errors"
	"strings"
)

const InsightsAccessKey = "insights_access"
const InsightsAccessDescription = "系统授权字段：允许查看全部 AI基础设施看板数据，不授予编辑或系统管理权限。"

// IsInsightsAccessKey reserves this attribute from identity synchronization.
func IsInsightsAccessKey(key string) bool { return strings.TrimSpace(key) == InsightsAccessKey }

func validInsightsAccessDefinition(def *UserAttributeDefinition) bool {
	if def == nil || def.Key != InsightsAccessKey || !def.Enabled || def.Required || def.Type != AttributeTypeSelect || def.Description != InsightsAccessDescription || len(def.Options) != 2 {
		return false
	}
	values := map[string]bool{}
	for _, option := range def.Options {
		values[option.Value] = true
	}
	return values["enabled"] && values["disabled"]
}

// CanViewInsights reads current authorization without the admin list's snapshot cache.
// System administrators retain their existing access; callers validate account status.
func (s *UserAttributeService) CanViewInsights(ctx context.Context, userID int64, admin bool) (bool, error) {
	if admin {
		return true, nil
	}
	if s == nil {
		return false, nil
	}
	def, err := s.defRepo.GetByKey(ctx, InsightsAccessKey)
	if errors.Is(err, ErrAttributeDefinitionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !validInsightsAccessDefinition(def) {
		return false, nil
	}
	values, err := s.valueRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, value := range values {
		if value.AttributeID == def.ID {
			return value.Value == "enabled", nil
		}
	}
	return false, nil
}
