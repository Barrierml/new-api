package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddTokenDefaultsToDisallowingUnsafeChannels(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	body := map[string]any{
		"name":                 "safe-only-key",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var token model.Token
	require.NoError(t, db.Where("user_id = ?", 1).First(&token).Error)
	require.NotNil(t, token.AllowUnsafeChannels)
	assert.False(t, *token.AllowUnsafeChannels)
}

func TestUpdateTokenPersistsAndPreservesAllowUnsafeChannels(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "editable-safety-key", "safety1234token5678")
	initialValue := true
	require.NoError(t, db.Model(token).Update("allow_unsafe_channels", initialValue).Error)

	body := map[string]any{
		"id":                    token.Id,
		"name":                  token.Name,
		"expired_time":          -1,
		"remain_quota":          100,
		"unlimited_quota":       true,
		"model_limits_enabled":  false,
		"model_limits":          "",
		"group":                 "default",
		"cross_group_retry":     false,
		"allow_unsafe_channels": false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)

	var updated model.Token
	require.NoError(t, db.First(&updated, token.Id).Error)
	require.NotNil(t, updated.AllowUnsafeChannels)
	assert.False(t, *updated.AllowUnsafeChannels)

	delete(body, "allow_unsafe_channels")
	body["name"] = "renamed-safety-key"
	ctx, recorder = newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)

	require.NoError(t, db.First(&updated, token.Id).Error)
	require.NotNil(t, updated.AllowUnsafeChannels)
	assert.False(t, *updated.AllowUnsafeChannels)
}

func TestMaskedLegacyTokenResponseAllowsUnsafeChannels(t *testing.T) {
	masked := buildMaskedTokenResponse(&model.Token{})

	require.NotNil(t, masked.AllowUnsafeChannels)
	assert.True(t, *masked.AllowUnsafeChannels)
}
