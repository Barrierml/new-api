package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func resetGlobalModelMappingForTest(t *testing.T) {
	t.Helper()
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString("[]"))
	t.Cleanup(func() {
		require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString("[]"))
	})
}

func TestUpdateGlobalModelMappingGroupsValidation(t *testing.T) {
	resetGlobalModelMappingForTest(t)

	// 空串等价空列表
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(""))
	require.Empty(t, GetGlobalModelMappingGroups())

	// 重复组 id → 报错
	err := UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"a","enabled":false,"mappings":{"m1":"m2"}},
		{"id":"g1","name":"b","enabled":false,"mappings":{"m3":"m4"}}
	]`)
	require.ErrorContains(t, err, "duplicate group id")

	// 空 name → 报错
	err = UpdateGlobalModelMappingGroupsByJSONString(`[{"id":"g1","name":" ","enabled":false,"mappings":{"m1":"m2"}}]`)
	require.ErrorContains(t, err, "name is empty")

	// 空 mappings → 报错
	err = UpdateGlobalModelMappingGroupsByJSONString(`[{"id":"g1","name":"a","enabled":false,"mappings":{}}]`)
	require.ErrorContains(t, err, "mappings is empty")

	// key/value 空 → 报错
	err = UpdateGlobalModelMappingGroupsByJSONString(`[{"id":"g1","name":"a","enabled":false,"mappings":{" ":"m2"}}]`)
	require.ErrorContains(t, err, "empty key or value")

	// enabled 组间源 key 冲突 → 报错并带冲突信息
	err = UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"g1-name","enabled":true,"mappings":{"m1":"m2"}},
		{"id":"g2","name":"g2-name","enabled":true,"mappings":{"m1":"m9"}}
	]`)
	require.ErrorContains(t, err, "conflict")
	require.ErrorContains(t, err, "m1")

	// 同一 key 但一个组禁用 → 允许
	err = UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"g1-name","enabled":true,"mappings":{"m1":"m2"}},
		{"id":"g2","name":"g2-name","enabled":false,"mappings":{"m1":"m9"}}
	]`)
	require.NoError(t, err)
}

func TestResolveGlobalMappedModelOnlyEnabledGroups(t *testing.T) {
	resetGlobalModelMappingForTest(t)

	// 禁用组不进 active map
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"disabled","enabled":false,"mappings":{"model-a":"model-b"}}
	]`))
	require.Equal(t, "model-a", ResolveGlobalMappedModel("model-a"))
	require.Nil(t, GetActiveGlobalModelMapping())

	// 启用后生效;无映射模型原样返回
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"enabled","enabled":true,"mappings":{"model-a":"model-b"}}
	]`))
	require.Equal(t, "model-b", ResolveGlobalMappedModel("model-a"))
	require.Equal(t, "model-c", ResolveGlobalMappedModel("model-c"))
	active := GetActiveGlobalModelMapping()
	require.Equal(t, map[string]string{"model-a": "model-b"}, active)

	// 再禁用 → 恢复原样
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"enabled","enabled":false,"mappings":{"model-a":"model-b"}}
	]`))
	require.Equal(t, "model-a", ResolveGlobalMappedModel("model-a"))
}

func TestGlobalModelMappingMergeAndChainAcrossGroups(t *testing.T) {
	resetGlobalModelMappingForTest(t)

	// 多个 enabled 组合并;跨组链式 key 允许(A→B + B→C)
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"first","enabled":true,"mappings":{"a":"b"}},
		{"id":"g2","name":"second","enabled":true,"mappings":{"b":"c"}}
	]`))
	require.Equal(t, "b", ResolveGlobalMappedModel("a"))
	require.Equal(t, "c", ResolveGlobalMappedModel("b"))
	require.Equal(t, map[string]string{"a": "b", "b": "c"}, GetActiveGlobalModelMapping())
}

func TestGetGlobalModelMappingGroupsReturnsCopy(t *testing.T) {
	resetGlobalModelMappingForTest(t)
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"a","enabled":true,"mappings":{"x":"y"},"created_at":1,"updated_at":2}
	]`))
	groups := GetGlobalModelMappingGroups()
	require.Len(t, groups, 1)
	require.Equal(t, "g1", groups[0].Id)
	require.Equal(t, int64(1), groups[0].CreatedAt)
	groups[0].Name = "mutated"
	// 副本:改动不影响内部状态
	require.Equal(t, "a", GetGlobalModelMappingGroups()[0].Name)
}

func TestUpdateGlobalModelMappingGroupsInvalidJSON(t *testing.T) {
	resetGlobalModelMappingForTest(t)
	require.NoError(t, UpdateGlobalModelMappingGroupsByJSONString(`[
		{"id":"g1","name":"keep","enabled":true,"mappings":{"x":"y"}}
	]`))
	require.Error(t, UpdateGlobalModelMappingGroupsByJSONString(`{not json`))
	// 失败不落:旧状态保留
	require.Equal(t, "y", ResolveGlobalMappedModel("x"))
	require.Len(t, GetGlobalModelMappingGroups(), 1)
}
