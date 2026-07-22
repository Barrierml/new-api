package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// countTokensHTTPTimeout 限制上游 count_tokens 透传耗时，避免拖住客户端的上下文预算预检。
const countTokensHTTPTimeout = 15 * time.Second

// CountTokensClaude 实现 Anthropic /v1/messages/count_tokens。
//
// 策略（LiteLLM /count_tokens 模式）：上游原生支持就透传（精度最高，含 cache
// 拆分），上游不支持（4xx/5xx/超时/不可达）则回退本地 tokenizer 估算。
// count_tokens 只用于客户端预检（上下文窗口/成本预估），不参与计费，本地估算
// 略有偏差可接受——真正影响账单的是实际请求返回的 upstream usage。
func CountTokensClaude(c *gin.Context) {
	var claudeReq dto.ClaudeRequest
	if err := common.UnmarshalBodyReusable(c, &claudeReq); err != nil {
		respondCountTokensError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}
	model := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if model == "" {
		model = claudeReq.Model
	}

	// 1) 上游透传优先：Distribute 已选好渠道，base_url/key/type 在 context 里
	if status, respBody, ok := tryForwardUpstreamCountTokens(c); ok {
		c.Data(status, "application/json", respBody)
		return
	}

	// 2) 回退本地 tokenizer 估算
	localTokens := localCountClaudeTokens(&claudeReq, model)
	logger.LogInfo(c, fmt.Sprintf("count_tokens 回退本地估算 model=%s input_tokens=%d", model, localTokens))
	c.JSON(http.StatusOK, gin.H{
		"input_tokens": localTokens,
	})
}

// tryForwardUpstreamCountTokens 尝试把 count_tokens 透传给选定渠道的上游。
// 命中上游 2xx 时返回 (status, body, true)；其余情况（无渠道信息/无可用 body/
// 上游不可达/上游非 2xx）返回 (0, nil, false)，由调用方走本地回退。
func tryForwardUpstreamCountTokens(c *gin.Context) (int, []byte, bool) {
	baseURL := common.GetContextKeyString(c, constant.ContextKeyChannelBaseUrl)
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if baseURL == "" || apiKey == "" {
		return 0, nil, false
	}
	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		return 0, nil, false
	}
	if _, err := bodyStorage.Seek(0, io.SeekStart); err != nil {
		return 0, nil, false
	}
	bodyBytes, err := io.ReadAll(bodyStorage)
	if err != nil || len(bodyBytes) == 0 {
		return 0, nil, false
	}

	anthropicVersion := c.Request.Header.Get("anthropic-version")
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	anthropicBeta := c.Request.Header.Get("anthropic-beta")

	status, respBody, reached := fetchUpstreamCountTokens(
		c.Request.Context(), baseURL, apiKey, channelType, anthropicVersion, anthropicBeta, bodyBytes)
	if !reached {
		logger.LogDebug(c, "count_tokens 上游不可达，回退本地")
		return 0, nil, false
	}
	if status < 200 || status >= 300 {
		logger.LogDebug(c, "count_tokens 上游 %d，回退本地：%s",
			status, common.LocalLogPreview(string(respBody)))
		return 0, nil, false
	}
	return status, respBody, true
}

// fetchUpstreamCountTokens 向上游发起 count_tokens 请求。
// 返回 (statusCode, body, true) 表示收到上游响应（任意状态码）；
// 返回 (0, nil, false) 表示请求本身失败（网络错误/超时/构造失败）。
func fetchUpstreamCountTokens(ctx context.Context, baseURL, apiKey string, channelType int, anthropicVersion, anthropicBeta string, body []byte) (int, []byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, countTokensHTTPTimeout)
	defer cancel()

	upstreamURL := strings.TrimRight(baseURL, "/") + "/v1/messages/count_tokens"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicVersion)
	if anthropicBeta != "" {
		req.Header.Set("anthropic-beta", anthropicBeta)
	}
	// 鉴权按渠道类型：type=14 Anthropic 用 x-api-key；其余（sub2api 等多语种上游）用 Bearer
	if channelType == constant.ChannelTypeAnthropic {
		req.Header.Set("x-api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return 0, nil, false
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, true
}

// localCountClaudeTokens 用 Claude DTO 的 token 计数元数据 + 本地 tokenizer 估算 input tokens。
func localCountClaudeTokens(req *dto.ClaudeRequest, model string) int {
	if req == nil {
		return 0
	}
	meta := req.GetTokenCountMeta()
	if meta == nil {
		return 0
	}
	return service.CountTextToken(meta.CombineText, model)
}

func respondCountTokensError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
