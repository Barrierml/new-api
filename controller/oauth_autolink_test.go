package controller

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOAuthAutoLinkTest(t *testing.T) *oauth.GenericOAuthProvider {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserOAuthBinding{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	return oauth.NewGenericOAuthProvider(&model.CustomOAuthProvider{
		Id: 1, Slug: "google", Enabled: true,
		UserInfoEndpoint: "https://openidconnect.googleapis.com/v1/userinfo",
	})
}

func createOAuthAutoLinkUser(t *testing.T, email string, status int) *model.User {
	t.Helper()
	user := &model.User{
		Username: "existing-user", Password: "password", Email: email,
		Role: common.RoleCommonUser, Status: status, Group: "vip", Quota: 12345,
		AffCode: "aff-existing",
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func TestLinkVerifiedOAuthEmailLinksExistingEnabledUser(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	user := createOAuthAutoLinkUser(t, "Existing@Example.com", common.UserStatusEnabled)

	linked, found, err := linkVerifiedOAuthEmail(provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-1", Email: " existing@example.COM ", EmailVerified: true,
	})

	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, linked)
	assert.Equal(t, user.Id, linked.Id)
	assert.Equal(t, 12345, linked.Quota)
	assert.Equal(t, "vip", linked.Group)

	binding, err := model.GetUserOAuthBinding(user.Id, provider.GetProviderId())
	require.NoError(t, err)
	assert.Equal(t, "google-user-1", binding.ProviderUserId)
}

func TestLinkVerifiedOAuthEmailDoesNotMatchMissingEmail(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	linked, found, err := linkVerifiedOAuthEmail(provider, &oauth.OAuthUser{ProviderUserID: "google-user-2"})
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, linked)
}

func TestLinkVerifiedOAuthEmailRejectsDisabledUser(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	createOAuthAutoLinkUser(t, "disabled@example.com", common.UserStatusDisabled)

	linked, found, err := linkVerifiedOAuthEmail(provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-3", Email: "disabled@example.com", EmailVerified: true,
	})

	assert.Error(t, err)
	assert.IsType(t, &OAuthUserDeletedError{}, err)
	assert.False(t, found)
	assert.Nil(t, linked)
}

func TestLinkVerifiedOAuthEmailRejectsConflictingBinding(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	user := createOAuthAutoLinkUser(t, "bound@example.com", common.UserStatusEnabled)
	require.NoError(t, model.DB.Create(&model.UserOAuthBinding{
		UserId: user.Id, ProviderId: provider.GetProviderId(), ProviderUserId: "other-google-user",
	}).Error)

	linked, found, err := linkVerifiedOAuthEmail(provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-4", Email: "bound@example.com", EmailVerified: true,
	})

	assert.Error(t, err)
	assert.IsType(t, &OAuthEmailAlreadyTakenError{}, err)
	assert.False(t, found)
	assert.Nil(t, linked)
}

func TestFindOrCreateOAuthUserDoesNotAutoLinkUnverifiedEmail(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	createOAuthAutoLinkUser(t, "unverified@example.com", common.UserStatusEnabled)
	previousRegisterEnabled := common.RegisterEnabled
	common.RegisterEnabled = true
	t.Cleanup(func() { common.RegisterEnabled = previousRegisterEnabled })

	_, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-5", Email: "unverified@example.com", EmailVerified: false,
	}, "")

	require.Error(t, err)
	assert.IsType(t, &OAuthEmailAlreadyTakenError{}, err)
	var count int64
	require.NoError(t, model.DB.Model(&model.UserOAuthBinding{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestLinkVerifiedOAuthEmailRejectsAmbiguousEmail(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	createOAuthAutoLinkUser(t, "duplicate@example.com", common.UserStatusEnabled)
	second := &model.User{
		Username: "second-user", Password: "password", Email: "DUPLICATE@example.com",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "aff-second",
	}
	require.NoError(t, model.DB.Create(second).Error)

	_, found, err := linkVerifiedOAuthEmail(provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-6", Email: "duplicate@example.com", EmailVerified: true,
	})

	assert.True(t, errors.Is(err, model.ErrEmailAmbiguous))
	assert.False(t, found)
}

func TestFindOrCreateOAuthUserAutoLinksVerifiedGoogleEmail(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	user := createOAuthAutoLinkUser(t, "verified@example.com", common.UserStatusEnabled)
	previousRegisterEnabled := common.RegisterEnabled
	common.RegisterEnabled = false
	t.Cleanup(func() { common.RegisterEnabled = previousRegisterEnabled })

	linked, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-7", Email: "verified@example.com", EmailVerified: true,
	}, "")

	require.NoError(t, err)
	require.NotNil(t, linked)
	assert.Equal(t, user.Id, linked.Id)
	binding, err := model.GetUserOAuthBinding(user.Id, provider.GetProviderId())
	require.NoError(t, err)
	assert.Equal(t, "google-user-7", binding.ProviderUserId)
}

func TestFindOrCreateOAuthUserDoesNotAutoLinkSoftDeletedUser(t *testing.T) {
	provider := setupOAuthAutoLinkTest(t)
	user := createOAuthAutoLinkUser(t, "deleted@example.com", common.UserStatusEnabled)
	require.NoError(t, model.DB.Delete(user).Error)
	previousRegisterEnabled := common.RegisterEnabled
	common.RegisterEnabled = true
	t.Cleanup(func() { common.RegisterEnabled = previousRegisterEnabled })

	linked, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "google-user-8", Email: "deleted@example.com", EmailVerified: true,
	}, "")

	require.Error(t, err)
	assert.IsType(t, &OAuthEmailAlreadyTakenError{}, err)
	assert.Nil(t, linked)
	var count int64
	require.NoError(t, model.DB.Model(&model.UserOAuthBinding{}).Count(&count).Error)
	assert.Zero(t, count)
}
