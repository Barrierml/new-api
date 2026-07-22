package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 全局 HTTP client（service.GetHttpClient）只在服务器启动时由 InitHttpClient 初始化，
// 单元测试里需手动初始化，否则 fetchUpstreamCountTokens 会拿到 nil client。
func TestMain(m *testing.M) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestFetchUpstreamCountTokens(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5","messages":[{"role":"user","content":"hi"}]}`)

	t.Run("anthropic channel 2xx passes through cache fields and x-api-key auth", func(t *testing.T) {
		var (
			gotPath, gotAPIKey, gotBearer, gotVersion string
			gotBeta                                   string
			gotBody                                   []byte
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAPIKey = r.Header.Get("x-api-key")
			gotBearer = r.Header.Get("Authorization")
			gotVersion = r.Header.Get("anthropic-version")
			gotBeta = r.Header.Get("anthropic-beta")
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"input_tokens":431,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}`))
		}))
		defer srv.Close()

		status, respBody, reached := fetchUpstreamCountTokens(
			context.Background(), srv.URL, "test-key", constant.ChannelTypeAnthropic, "2023-06-01", "", body)
		require.True(t, reached)
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "/v1/messages/count_tokens", gotPath)
		assert.Equal(t, "test-key", gotAPIKey, "type=14 渠道必须用 x-api-key")
		assert.Empty(t, gotBearer, "type=14 渠道不应带 Bearer")
		assert.Equal(t, "2023-06-01", gotVersion)
		assert.Empty(t, gotBeta, "无 beta 时不转发")
		assert.Equal(t, body, gotBody)
		assert.Contains(t, string(respBody), "cache_read_input_tokens", "透传要保留 cache 拆分字段")
	})

	t.Run("openai-compatible channel uses Bearer auth and no x-api-key", func(t *testing.T) {
		var gotAPIKey, gotBearer string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAPIKey = r.Header.Get("x-api-key")
			gotBearer = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"input_tokens":10}`))
		}))
		defer srv.Close()

		_, _, reached := fetchUpstreamCountTokens(
			context.Background(), srv.URL, "gw-test", 1, "2023-06-01", "", body)
		require.True(t, reached)
		assert.Empty(t, gotAPIKey, "非 Anthropic 渠道不应带 x-api-key")
		assert.Equal(t, "Bearer gw-test", gotBearer, "sub2api 等多语种上游用 Bearer")
	})

	t.Run("anthropic-beta header forwarded when present", func(t *testing.T) {
		var gotBeta string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBeta = r.Header.Get("anthropic-beta")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"input_tokens":1}`))
		}))
		defer srv.Close()

		_, _, _ = fetchUpstreamCountTokens(
			context.Background(), srv.URL, "test-key", constant.ChannelTypeAnthropic, "2023-06-01", "prompt-caching-2024-07-31", body)
		assert.Equal(t, "prompt-caching-2024-07-31", gotBeta)
	})

	t.Run("upstream non-2xx returns reached=true so caller falls back to local", func(t *testing.T) {
		// 上游不支持 count_tokens（如 sub2api 部分组返回 400/404）→ 调用方据此走本地估算
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"not supported"}`))
		}))
		defer srv.Close()

		status, _, reached := fetchUpstreamCountTokens(
			context.Background(), srv.URL, "test-key", constant.ChannelTypeAnthropic, "2023-06-01", "", body)
		require.True(t, reached, "收到响应（即便非 2xx）也算 reachable，状态码交给调用方判断")
		assert.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("unreachable upstream returns reached=false", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		srv.Close() // 立即关闭，制造连接失败

		_, _, reached := fetchUpstreamCountTokens(
			context.Background(), srv.URL, "test-key", constant.ChannelTypeAnthropic, "2023-06-01", "", body)
		assert.False(t, reached, "网络不可达应回退本地，而不是报错给客户端")
	})
}

func TestLocalCountClaudeTokens(t *testing.T) {
	t.Run("nil request returns zero", func(t *testing.T) {
		assert.Equal(t, 0, localCountClaudeTokens(nil, "claude-fable-5"))
	})

	t.Run("non-empty prompt yields positive estimate", func(t *testing.T) {
		req := &dto.ClaudeRequest{
			Model: "claude-fable-5",
			Messages: []dto.ClaudeMessage{
				{Role: "user", Content: "Hello world, this is a counting test."},
			},
		}
		n := localCountClaudeTokens(req, "claude-fable-5")
		assert.Greater(t, n, 0, "回退路径必须给客户端一个可用的正数估算")
	})
}

// TestCountTokensClaudeHandler 覆盖 handler 的真实决策路径：透传 vs 回退。
// 用前置中间件手动注入 Distribute 本会填充的渠道 context，绕过整条中间件链。
func TestCountTokensClaudeHandler(t *testing.T) {
	// setChannel 注入渠道 context（base_url/key/type/model）；传 nil 表示未选到渠道。
	newEngine := func(setChannel func(c *gin.Context)) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if setChannel != nil {
				setChannel(c)
			}
			c.Next()
		})
		r.POST("/v1/messages/count_tokens", CountTokensClaude)
		return r
	}
	doRequest := func(r *gin.Engine, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	withAnthropicChannel := func(c *gin.Context, baseURL string) {
		common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, baseURL)
		common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
		common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	}

	t.Run("upstream 2xx passes through body with cache fields", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"input_tokens":433,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}`))
		}))
		defer srv.Close()

		r := newEngine(func(c *gin.Context) {
			withAnthropicChannel(c, srv.URL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-5")
		})
		rec := doRequest(r, `{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`)

		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, `"input_tokens":433`)
		assert.Contains(t, body, "cache_read_input_tokens", "透传要原样保留上游的 cache 拆分字段")
	})

	t.Run("upstream non-2xx falls back to local estimate (no cache fields)", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"not supported"}`))
		}))
		defer srv.Close()

		r := newEngine(func(c *gin.Context) {
			withAnthropicChannel(c, srv.URL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-fable-5")
		})
		rec := doRequest(r, `{"model":"claude-fable-5","messages":[{"role":"user","content":"hello world count me"}]}`)

		require.Equal(t, http.StatusOK, rec.Code, "回退路径仍返回 200,给客户端可用估算")
		body := rec.Body.String()
		assert.Contains(t, body, "input_tokens")
		assert.NotContains(t, body, "cache_read_input_tokens", "本地估算不带 cache 拆分")
	})

	t.Run("no channel selected falls back to local", func(t *testing.T) {
		r := newEngine(func(c *gin.Context) {
			// 模拟 Distribute 未注入任何渠道信息（base_url/key 为空）
		})
		rec := doRequest(r, `{"model":"claude-fable-5","messages":[{"role":"user","content":"hello"}]}`)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "input_tokens")
	})

	t.Run("invalid JSON body returns 400 Claude error without hitting upstream", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("解析失败不应触达上游")
		}))
		defer srv.Close()

		r := newEngine(func(c *gin.Context) {
			withAnthropicChannel(c, srv.URL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-5")
		})
		rec := doRequest(r, `{not valid json`)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, `"type":"error"`, "错误响应用 Claude error 格式")
		assert.Contains(t, body, "invalid_request_error")
	})
}
