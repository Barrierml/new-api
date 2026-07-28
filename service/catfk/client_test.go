package catfk

import (
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyForReqFallsBackToEnvironment(t *testing.T) {
	originalURLs := setting.CatfkProxyURLs
	originalEnvironmentProxy := environmentProxy
	t.Cleanup(func() {
		setting.CatfkProxyURLs = originalURLs
		environmentProxy = originalEnvironmentProxy
	})

	expected, err := url.Parse("http://environment-proxy:3128")
	require.NoError(t, err)
	environmentProxy = func(*http.Request) (*url.URL, error) { return expected, nil }
	req, err := http.NewRequest(http.MethodGet, origin, nil)
	require.NoError(t, err)

	setting.CatfkProxyURLs = ""
	actual, err := proxyForReq(req)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)

	setting.CatfkProxyURLs = "http://dedicated-proxy:8080, "
	atomic.StoreUint64(&proxyIdx, 0) // first selection is the blank second entry
	actual, err = proxyForReq(req)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestProxyForReqPrefersDedicatedPool(t *testing.T) {
	originalURLs := setting.CatfkProxyURLs
	originalEnvironmentProxy := environmentProxy
	t.Cleanup(func() {
		setting.CatfkProxyURLs = originalURLs
		environmentProxy = originalEnvironmentProxy
	})

	environmentProxy = func(*http.Request) (*url.URL, error) {
		t.Fatal("environment proxy must not be used for a non-empty pool entry")
		return nil, nil
	}
	setting.CatfkProxyURLs = "http://proxy-a:8080,http://proxy-b:8080"
	atomic.StoreUint64(&proxyIdx, 0)
	req, err := http.NewRequest(http.MethodGet, origin, nil)
	require.NoError(t, err)

	actual, err := proxyForReq(req)
	require.NoError(t, err)
	assert.Equal(t, "http://proxy-b:8080", actual.String())
}

func TestParseLoginResponse(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		cookie, err := parseLoginResponse(http.StatusOK, "application/json", []byte(`{"code":1,"msg":"ok"}`), []*http.Cookie{
			{Name: "session", Value: "secret-value"},
		})
		require.NoError(t, err)
		assert.Equal(t, "session=secret-value", cookie)
	})

	t.Run("non JSON upstream error is secret safe", func(t *testing.T) {
		body := []byte("<html>private-upstream-response</html>")
		_, err := parseLoginResponse(http.StatusBadGateway, "text/html", body, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status=502")
		assert.Contains(t, err.Error(), "content_type=\"text/html\"")
		assert.Contains(t, err.Error(), "bytes=38")
		assert.NotContains(t, err.Error(), string(body))
		assert.NotContains(t, err.Error(), "private-upstream-response")
	})

	t.Run("successful HTTP with invalid JSON is secret safe", func(t *testing.T) {
		body := []byte("credential-like-private-body")
		_, err := parseLoginResponse(http.StatusOK, "text/plain", body, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "响应非JSON")
		assert.NotContains(t, err.Error(), string(body))
	})

	t.Run("provider rejection keeps message", func(t *testing.T) {
		_, err := parseLoginResponse(http.StatusOK, "application/json", []byte(`{"code":0,"msg":"账号或密码错误"}`), nil)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "code=0"))
		assert.True(t, strings.Contains(err.Error(), "账号或密码错误"))
	})
}
