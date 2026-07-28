package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// CodexFormClientVersion is the client_version query value tako-cli sends to opt
// into the Codex-form /v1/models response — the only shape that carries
// context_window. See tako-cli src/models/tako.ts and docs/models/DESIGN.md.
const CodexFormClientVersion = "tako-cli"

// codexFormModel is the entry tako-cli expects in the Codex-form list. Its model
// picker reads these fields; in particular context_window drives the non-GPT
// `model_context_window` written to ~/.codex/config.toml, and model_category is
// used to filter non-chat models out of the picker. Field names are snake_case
// to match tako-cli's CodexModelDTO (parseCodexResponse).
type codexFormModel struct {
	Slug          string `json:"slug"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	ContextWindow int    `json:"context_window"`
	Priority      int    `json:"priority"`
	ModelCategory string `json:"model_category"`
}

// ListCodexFormModels serves GET /v1/models?client_version=tako-cli, returning
// the available model list in the Codex form tako-cli expects:
// {"models":[{slug, context_window, ...}]}. It reuses collectUserModelNames so
// the returned set is identical to the OpenAI/Anthropic/Gemini ListModels
// responses; only the rendering differs.
//
// context_window is looked up from the curated metadata for this tako instance
// (codexFormContextWindows); models not present report 0, which tako-cli treats
// as "unknown" and falls back to its own bundled catalog for.
//
// api_type (openai|claude) is accepted by tako-cli but intentionally not used to
// filter here: the existing OpenAI /v1/models returns the full available set and
// tako-cli filters non-chat models client-side by model_category, so returning
// the same set keeps the two shapes consistent.
func ListCodexFormModels(c *gin.Context) {
	userModelNames, _, ok := collectUserModelNames(c)
	if !ok {
		return
	}
	models := make([]codexFormModel, 0, len(userModelNames))
	for _, name := range userModelNames {
		models = append(models, codexFormModel{
			Slug:          name,
			DisplayName:   name,
			ContextWindow: codexFormContextWindows[name],
			ModelCategory: codexFormModelCategory(name),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"models": models,
	})
}

// codexFormModelCategory classifies a model for the picker from the endpoint
// types it supports: chat-capable types (openai chat/responses, anthropic,
// gemini) or unknown → "chat"; otherwise the specific non-chat category. This
// keeps embedding/image/video models out of the Codex/Claude Code model picker
// (which only does chat), so it does not regress vs tako-cli's bundled catalog.
func codexFormModelCategory(name string) string {
	types := model.GetModelSupportEndpointTypes(name)
	for _, t := range types {
		switch t {
		case constant.EndpointTypeEmbeddings:
			return "embedding"
		case constant.EndpointTypeImageGeneration:
			return "image"
		case constant.EndpointTypeOpenAIVideo:
			return "video"
		}
	}

	// The endpoint registry currently has no dedicated audio/realtime/ASR types.
	// Conservatively classify known non-chat naming forms so they cannot enter a
	// Claude Code or Codex chat picker merely because their endpoint is generic.
	lowerName := strings.ToLower(name)
	switch {
	case strings.Contains(lowerName, "realtime"):
		return "realtime"
	case strings.Contains(lowerName, "audio"),
		strings.Contains(lowerName, "-asr"),
		strings.Contains(lowerName, "speech"),
		strings.HasPrefix(lowerName, "tts-"):
		return "audio"
	case strings.Contains(lowerName, "image"):
		return "image"
	case strings.Contains(lowerName, "video"):
		return "video"
	}
	return "chat"
}
