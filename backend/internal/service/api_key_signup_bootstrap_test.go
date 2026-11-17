//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type signupBootstrapAPIKeyRepoStub struct {
	*apiKeyRepoStub
	activeCount int64
	created     []*APIKey
}

func (s *signupBootstrapAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	return s.activeCount, nil
}

func (s *signupBootstrapAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	s.created = append(s.created, key)
	return nil
}

type signupBootstrapUserRepoStub struct {
	UserRepository
	user       *User
	allowedIDs []int64
}

func (s *signupBootstrapUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

func (s *signupBootstrapUserRepoStub) AddGroupToAllowedGroups(_ context.Context, _ int64, groupID int64) error {
	s.allowedIDs = append(s.allowedIDs, groupID)
	return nil
}

type signupBootstrapGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (s *signupBootstrapGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	return append([]Group(nil), s.groups...), nil
}

func (s *signupBootstrapGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range s.groups {
		if s.groups[i].ID == id {
			group := s.groups[i]
			return &group, nil
		}
	}
	return nil, ErrGroupNotFound
}

type signupBootstrapSubscriptionRepoStub struct {
	UserSubscriptionRepository
}

func (s *signupBootstrapSubscriptionRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

type signupBootstrapAssignerStub struct {
	userIDs []int64
}

func (s *signupBootstrapAssignerStub) EnsureDefaultAPIKey(_ context.Context, userID int64) error {
	s.userIDs = append(s.userIDs, userID)
	return nil
}

func TestAuthServiceRegisterInitializesDefaultAPIKey(t *testing.T) {
	repo := &userRepoStub{nextID: 42}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:                 "true",
		SettingKeyAuthSourceDefaultEmailGrantOnSignup: "false",
	}, nil, nil)
	assigner := &signupBootstrapAssignerStub{}
	svc.SetDefaultAPIKeyAssigner(assigner)

	_, user, err := svc.Register(context.Background(), "default-key@example.com", "password")
	require.NoError(t, err)
	require.Equal(t, int64(42), user.ID)
	require.Equal(t, []int64{42}, assigner.userIDs)
}

func TestAuthServiceEmailOAuthRegistrationInitializesDefaultAPIKey(t *testing.T) {
	repo := &userRepoStub{nextID: 43}
	svc := newEmailOAuthAutoAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil)
	assigner := &signupBootstrapAssignerStub{}
	svc.SetDefaultAPIKeyAssigner(assigner)

	user, err := svc.createEmailOAuthUser(
		context.Background(),
		"default-oauth-key@example.com",
		"default-oauth-key",
		"github",
		"",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, int64(43), user.ID)
	require.Equal(t, []int64{43}, assigner.userIDs)
}

func newSignupBootstrapAPIKeyService(
	apiKeyRepo APIKeyRepository,
	userRepo UserRepository,
	groupRepo GroupRepository,
	subscriptionRepo UserSubscriptionRepository,
) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo:  apiKeyRepo,
		userRepo:    userRepo,
		groupRepo:   groupRepo,
		userSubRepo: subscriptionRepo,
		cfg:         &config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}},
	}
}

func TestAPIKeyServiceEnsureDefaultAPIKeyCreatesGeneratedKeyForFirstAvailableGroup(t *testing.T) {
	apiKeys := &signupBootstrapAPIKeyRepoStub{apiKeyRepoStub: &apiKeyRepoStub{}}
	users := &signupBootstrapUserRepoStub{user: &User{ID: 42}}
	groups := &signupBootstrapGroupRepoStub{groups: []Group{
		{ID: 9, Name: "later", SortOrder: 10, Status: StatusActive},
		{ID: 7, Name: "first", SortOrder: 1, Status: StatusActive},
	}}
	svc := newSignupBootstrapAPIKeyService(apiKeys, users, groups, &signupBootstrapSubscriptionRepoStub{})

	require.NoError(t, svc.EnsureDefaultAPIKey(context.Background(), 42))
	require.Len(t, apiKeys.created, 1)
	require.Equal(t, "default-42", apiKeys.created[0].Name)
	require.NotNil(t, apiKeys.created[0].GroupID)
	require.Equal(t, int64(7), *apiKeys.created[0].GroupID)
	require.Equal(t, []int64{7}, users.allowedIDs)
	require.True(t, strings.HasPrefix(apiKeys.created[0].Key, "sk-"))
}

func TestAPIKeyServiceEnsureDefaultAPIKeySkipsWhenUserHasAnyAPIKey(t *testing.T) {
	apiKeys := &signupBootstrapAPIKeyRepoStub{
		apiKeyRepoStub: &apiKeyRepoStub{},
		activeCount:    1,
	}
	groups := &signupBootstrapGroupRepoStub{}
	users := &signupBootstrapUserRepoStub{user: &User{ID: 42}}
	svc := newSignupBootstrapAPIKeyService(apiKeys, users, groups, &signupBootstrapSubscriptionRepoStub{})

	require.NoError(t, svc.EnsureDefaultAPIKey(context.Background(), 42))
	require.Empty(t, apiKeys.created)
	require.Empty(t, users.allowedIDs)
}

func TestAPIKeyServiceEnsureDefaultAPIKeyCreatesUnboundKeyWithoutAvailableGroup(t *testing.T) {
	apiKeys := &signupBootstrapAPIKeyRepoStub{apiKeyRepoStub: &apiKeyRepoStub{}}
	users := &signupBootstrapUserRepoStub{user: &User{ID: 42}}
	svc := newSignupBootstrapAPIKeyService(
		apiKeys,
		users,
		&signupBootstrapGroupRepoStub{},
		&signupBootstrapSubscriptionRepoStub{},
	)

	require.NoError(t, svc.EnsureDefaultAPIKey(context.Background(), 42))
	require.Len(t, apiKeys.created, 1)
	require.Nil(t, apiKeys.created[0].GroupID)
	require.Empty(t, users.allowedIDs)
}
