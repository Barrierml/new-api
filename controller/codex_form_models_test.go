package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codexFormListResponse struct {
	Models []codexFormModel `json:"models"`
}

func decodeCodexFormResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]codexFormModel {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload codexFormListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	bySlug := make(map[string]codexFormModel, len(payload.Models))
	for _, m := range payload.Models {
		bySlug[m.Slug] = m
	}
	return bySlug
}

// TestListCodexFormModelsReturnsContextWindowAndCategory verifies the Codex-form
// /v1/models response shape: each entry carries context_window looked up from
// the curated metadata for this tako instance (0 when absent — tako-cli then falls back to
// its own bundled catalog), and model_category classifies chat vs non-chat so
// the picker is not polluted by embedding/image models. Disabled abilities must
// be excluded, mirroring ListModels.
func TestListCodexFormModelsReturnsContextWindowAndCategory(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1004,
		Username: "codex-form-model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "claude-opus-4-8", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "gpt-5.5", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "gpt-5.6-sol", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-not-in-map-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-disabled-model", ChannelId: 1, Enabled: false},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version=tako-cli", nil)
	ctx.Set("id", 1004)

	ListCodexFormModels(ctx)

	bySlug := decodeCodexFormResponse(t, recorder)
	assert.Len(t, bySlug, 4, "disabled ability must be excluded")
	assert.Contains(t, bySlug, "claude-opus-4-8")
	assert.Contains(t, bySlug, "gpt-5.5")
	assert.Contains(t, bySlug, "gpt-5.6-sol")
	assert.Contains(t, bySlug, "zz-not-in-map-model")
	assert.NotContains(t, bySlug, "zz-disabled-model")

	opus := bySlug["claude-opus-4-8"]
	assert.Equal(t, 950000, opus.ContextWindow, "instance-specific Opus limit")
	assert.Equal(t, "claude-opus-4-8", opus.DisplayName)
	assert.Equal(t, "chat", opus.ModelCategory)

	assert.Equal(t, 272000, bySlug["gpt-5.5"].ContextWindow)
	assert.Equal(t, 272000, bySlug["gpt-5.6-sol"].ContextWindow)
	assert.Equal(t, "chat", bySlug["gpt-5.6-sol"].ModelCategory)

	unknown := bySlug["zz-not-in-map-model"]
	assert.Zero(t, unknown.ContextWindow, "model absent from map reports 0; tako-cli falls back to its bundled catalog")
	assert.Equal(t, "chat", unknown.ModelCategory)
}

func TestCodexFormModelCategoryNameFallbacks(t *testing.T) {
	assert.Equal(t, "audio", codexFormModelCategory("gpt-4o-audio-preview"))
	assert.Equal(t, "realtime", codexFormModelCategory("gpt-4o-realtime-preview"))
	assert.Equal(t, "audio", codexFormModelCategory("mimo-v2.5-asr"))
	assert.Equal(t, "image", codexFormModelCategory("grok-imagine-image-quality"))
	assert.Equal(t, "chat", codexFormModelCategory("gpt-5.6"))
}

func TestCodexFormContextWindowInstanceOverrides(t *testing.T) {
	for _, name := range []string{"gpt-5.5", "gpt-5.6", "gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.6-terra"} {
		assert.Equal(t, 272000, codexFormContextWindows[name], name)
	}
	for _, name := range []string{"claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8", "claude-opus-5", "full-claude-opus-4-8"} {
		assert.Equal(t, 950000, codexFormContextWindows[name], name)
	}
	assert.Zero(t, codexFormContextWindows["codex-auto-review"], "unknown internal aliases must not be guessed")
	assert.Zero(t, codexFormContextWindows["composer-2.5"], "unknown internal aliases must not be guessed")
}

// TestListCodexFormModelsMatchesListModelsSet locks the core consistency
// guarantee: the Codex-form response exposes exactly the same available model
// set as the OpenAI ListModels response for the same user — only the rendering
// differs. This is what lets tako-cli's model picker and the OpenAI /v1/models
// list agree, so the statusline context-window denominator and the picker stay
// in sync.
func TestListCodexFormModelsMatchesListModelsSet(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1005,
		Username: "codex-form-set-parity-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "claude-haiku-4-5", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "gpt-4o", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "claude-3-sonnet-20240229", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-disabled-model", ChannelId: 1, Enabled: false},
	}).Error)

	// Codex form
	codexRecorder := httptest.NewRecorder()
	codexCtx, _ := gin.CreateTestContext(codexRecorder)
	codexCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version=tako-cli", nil)
	codexCtx.Set("id", 1005)
	ListCodexFormModels(codexCtx)
	codexBySlug := decodeCodexFormResponse(t, codexRecorder)

	// OpenAI form, same user / same DB
	openaiRecorder := httptest.NewRecorder()
	openaiCtx, _ := gin.CreateTestContext(openaiRecorder)
	openaiCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	openaiCtx.Set("id", 1005)
	ListModels(openaiCtx, constant.ChannelTypeOpenAI)
	openaiIds := decodeListModelsResponse(t, openaiRecorder)

	assert.Equal(t, len(openaiIds), len(codexBySlug), "both shapes must expose the same model count")
	for slug := range codexBySlug {
		assert.Contains(t, openaiIds, slug, "every Codex-form slug must also appear in the OpenAI list")
	}
}
