package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type insightsDefinitions struct {
	UserAttributeDefinitionRepository
	def *UserAttributeDefinition
	err error
}

func (r *insightsDefinitions) GetByKey(context.Context, string) (*UserAttributeDefinition, error) {
	return r.def, r.err
}
func (r *insightsDefinitions) GetByID(context.Context, int64) (*UserAttributeDefinition, error) {
	return r.def, r.err
}
func (r *insightsDefinitions) List(context.Context, bool) ([]UserAttributeDefinition, error) {
	return []UserAttributeDefinition{*r.def}, nil
}

type insightsValues struct {
	UserAttributeValueRepository
	value string
	err   error
	calls int
}

func (r *insightsValues) GetByUserID(_ context.Context, id int64) ([]UserAttributeValue, error) {
	r.calls++
	if id != 7 {
		return nil, errors.New("unexpected subject")
	}
	return []UserAttributeValue{{UserID: id, AttributeID: 41, Value: r.value}}, r.err
}
func (r *insightsValues) UpsertBatch(_ context.Context, _ int64, inputs []UpdateUserAttributeInput) error {
	r.value = inputs[0].Value
	return nil
}

func insightsDefinition() *UserAttributeDefinition {
	return &UserAttributeDefinition{ID: 41, Key: InsightsAccessKey, Description: InsightsAccessDescription, Type: AttributeTypeSelect, Enabled: true, Options: []UserAttributeOption{{Value: "enabled"}, {Value: "disabled"}}}
}
func TestInsightsAccessCurrentValueAndFailClosed(t *testing.T) {
	ctx := context.Background()
	defs := &insightsDefinitions{def: insightsDefinition()}
	values := &insightsValues{value: "enabled"}
	s := NewUserAttributeService(defs, values)
	allowed, err := s.CanViewInsights(ctx, 7, false)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, s.UpdateUserAttributes(ctx, 7, []UpdateUserAttributeInput{{AttributeID: 41, Value: "disabled"}}))
	allowed, err = s.CanViewInsights(ctx, 7, false)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 2, values.calls)
	for _, value := range []string{"", "true", "Enabled", " enabled ", "invalid"} {
		values.value = value
		allowed, err = s.CanViewInsights(ctx, 7, false)
		require.NoError(t, err)
		require.False(t, allowed, value)
	}
	values.value = "enabled"
	for _, change := range []func(*UserAttributeDefinition){func(d *UserAttributeDefinition) { d.Enabled = false }, func(d *UserAttributeDefinition) { d.Type = AttributeTypeText }, func(d *UserAttributeDefinition) { d.Description = "custom field" }, func(d *UserAttributeDefinition) { d.Options = nil }} {
		defs.def = insightsDefinition()
		change(defs.def)
		allowed, err = s.CanViewInsights(ctx, 7, false)
		require.NoError(t, err)
		require.False(t, allowed)
	}
	defs.err = ErrAttributeDefinitionNotFound
	allowed, err = s.CanViewInsights(ctx, 7, false)
	require.NoError(t, err)
	require.False(t, allowed)
	defs.err = errors.New("database unavailable")
	allowed, err = s.CanViewInsights(ctx, 7, false)
	require.Error(t, err)
	require.False(t, allowed)
	allowed, err = s.CanViewInsights(ctx, 7, true)
	require.NoError(t, err)
	require.True(t, allowed)
	defs.err = nil
	defs.def = insightsDefinition()
	values.err = errors.New("database unavailable")
	allowed, err = s.CanViewInsights(ctx, 7, false)
	require.Error(t, err)
	require.False(t, allowed)
}
func TestInsightsAccessDefinitionProtectedButUserGrantEditable(t *testing.T) {
	ctx := context.Background()
	defs := &insightsDefinitions{def: insightsDefinition()}
	values := &insightsValues{}
	s := NewUserAttributeService(defs, values)
	_, err := s.CreateDefinition(ctx, CreateAttributeDefinitionInput{Key: InsightsAccessKey, Type: AttributeTypeText})
	require.Error(t, err)
	_, err = s.UpdateDefinition(ctx, 41, UpdateAttributeDefinitionInput{})
	require.Error(t, err)
	require.Error(t, s.DeleteDefinition(ctx, 41))
	require.NoError(t, s.UpdateUserAttributes(ctx, 7, []UpdateUserAttributeInput{{AttributeID: 41, Value: "enabled"}}))
	require.Equal(t, "enabled", values.value)
	require.Error(t, s.UpdateUserAttributes(ctx, 7, []UpdateUserAttributeInput{{AttributeID: 41, Value: "true"}}))
}
