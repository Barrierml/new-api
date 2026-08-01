package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// mockRequest 仅实现 ModelMappedHelper 用到的 SetModelName,其余走 BaseRequest 默认。
type mockRequest struct {
	dto.BaseRequest
	model string
}

func (m *mockRequest) SetModelName(modelName string) { m.model = modelName }

func setupModelMappedTest(t *testing.T, channelMapping string) (*gin.Context, *relaycommon.RelayInfo, *mockRequest) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, ratio_setting.UpdateGlobalModelMappingGroupsByJSONString("[]"))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGlobalModelMappingGroupsByJSONString("[]"))
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if channelMapping != "" {
		c.Set("model_mapping", channelMapping)
	}
	info := &relaycommon.RelayInfo{OriginModelName: "model-a"}
	req := &mockRequest{model: "model-a"}
	return c, info, req
}

func enableGlobalGroup(t *testing.T, json string) {
	t.Helper()
	require.NoError(t, ratio_setting.UpdateGlobalModelMappingGroupsByJSONString(json))
}

func TestModelMappedHelperGlobalMappingNoChannelMapping(t *testing.T) {
	c, info, req := setupModelMappedTest(t, "")
	enableGlobalGroup(t, `[{"id":"g1","name":"failover","enabled":true,"mappings":{"model-a":"model-b"}}]`)

	require.NoError(t, ModelMappedHelper(c, info, req))
	require.True(t, info.IsModelMapped)
	require.Equal(t, "model-b", info.UpstreamModelName)
	require.Equal(t, "model-a", info.OriginModelName, "计费/日志模型不变")
	require.Equal(t, "model-b", req.model, "请求体模型 = 链尾")
}

func TestModelMappedHelperGlobalOverridesChannelSameKey(t *testing.T) {
	c, info, req := setupModelMappedTest(t, `{"model-a":"channel-target"}`)
	enableGlobalGroup(t, `[{"id":"g1","name":"failover","enabled":true,"mappings":{"model-a":"global-target"}}]`)

	require.NoError(t, ModelMappedHelper(c, info, req))
	require.Equal(t, "global-target", info.UpstreamModelName, "同 key 全局优先")
	require.Equal(t, "global-target", req.model)
}

func TestModelMappedHelperGlobalChainIntoChannelMapping(t *testing.T) {
	c, info, req := setupModelMappedTest(t, `{"model-b":"model-c"}`)
	enableGlobalGroup(t, `[{"id":"g1","name":"failover","enabled":true,"mappings":{"model-a":"model-b"}}]`)

	require.NoError(t, ModelMappedHelper(c, info, req))
	require.Equal(t, "model-c", info.UpstreamModelName, "全局 A→B + 渠道 B→C = 打 C")
	require.Equal(t, "model-c", req.model)
}

func TestModelMappedHelperGlobalChannelCycleDetected(t *testing.T) {
	c, info, req := setupModelMappedTest(t, `{"model-b":"model-a"}`)
	enableGlobalGroup(t, `[{"id":"g1","name":"failover","enabled":true,"mappings":{"model-a":"model-b"}}]`)

	err := ModelMappedHelper(c, info, req)
	require.ErrorContains(t, err, "model_mapping_contains_cycle")
}

func TestModelMappedHelperDisabledGlobalGroupIgnored(t *testing.T) {
	c, info, req := setupModelMappedTest(t, "")
	enableGlobalGroup(t, `[{"id":"g1","name":"failover","enabled":false,"mappings":{"model-a":"model-b"}}]`)

	require.NoError(t, ModelMappedHelper(c, info, req))
	require.False(t, info.IsModelMapped)
	require.Equal(t, "", req.model, "禁用组不触发映射,UpstreamModelName 为空")
}

func TestModelMappedHelperGlobalSelfMappingTreatedAsUnmapped(t *testing.T) {
	// 全局 A→A(自映射):视为未映射,不打 upstream 标记
	c, info, req := setupModelMappedTest(t, "")
	enableGlobalGroup(t, `[{"id":"g1","name":"self","enabled":true,"mappings":{"model-a":"model-a"}}]`)

	require.NoError(t, ModelMappedHelper(c, info, req))
	require.False(t, info.IsModelMapped)
	require.Equal(t, "model-a", req.model)
}
