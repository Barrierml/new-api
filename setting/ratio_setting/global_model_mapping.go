package ratio_setting

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/types"
)

// 全局临时模型映射(组):把请求模型名全局重定向到另一个模型,优先级高于
// 渠道级 model_mapping。典型场景:某模型上游挂了,快速全局切到替代模型,
// 不用逐个渠道改配置。
//
// 两层生效(同一份组配置):
//   - 路由层(选渠道前):middleware/distributor.go + controller/relay.go 用
//     ResolveGlobalMappedModel 决定选渠道/重试的模型名 → 兜底落到目标模型渠道池
//   - 协议层(选渠道后):relay/helper/model_mapped.go 用
//     GetActiveGlobalModelMapping 在链式重定向里优先查全局 map
//
// 存 options 表(key=GlobalModelMappingGroups,JSON 组列表),内存态 = 全量组
// (atomic.Value) + enabled 组合并 map(RWMap,relay 零 JSON 解析)。

// GlobalModelMappingGroup 一组可独立启停的模型映射。
type GlobalModelMappingGroup struct {
	Id        string            `json:"id"`
	Name      string            `json:"name"`
	Enabled   bool              `json:"enabled"`
	Mappings  map[string]string `json:"mappings"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
}

var (
	globalModelMappingGroupsValue atomic.Value // []GlobalModelMappingGroup
	globalModelMappingActive      = types.NewRWMap[string, string]()
)

func init() {
	globalModelMappingGroupsValue.Store([]GlobalModelMappingGroup{})
}

// rebuildActiveGlobalModelMapping 把 enabled 组的 mappings 合并进 active map。
// 调用前必须已通过 validateGlobalModelMappingGroups(保证 enabled 组间 key 不重叠)。
func rebuildActiveGlobalModelMapping(groups []GlobalModelMappingGroup) {
	merged := make(map[string]string)
	for _, g := range groups {
		if !g.Enabled {
			continue
		}
		for k, v := range g.Mappings {
			merged[k] = v
		}
	}
	raw, _ := json.Marshal(merged)
	_ = types.LoadFromJsonString(globalModelMappingActive, string(raw))
}

// validateGlobalModelMappingGroups 校验组列表:
//   - 组 id 唯一、name/mappings 非空、mappings key/value 非空
//   - enabled 组之间源模型 key 不重叠(同一模型全局只能有一个目标)
func validateGlobalModelMappingGroups(groups []GlobalModelMappingGroup) error {
	seenIds := make(map[string]string, len(groups))
	enabledKeys := make(map[string]string) // key -> 占用组名
	for _, g := range groups {
		if strings.TrimSpace(g.Id) == "" {
			return fmt.Errorf("global model mapping: group id is empty")
		}
		if prev, dup := seenIds[g.Id]; dup {
			return fmt.Errorf("global model mapping: duplicate group id %s (%s vs %s)", g.Id, prev, g.Name)
		}
		seenIds[g.Id] = g.Name
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("global model mapping: group %s name is empty", g.Id)
		}
		if len(g.Mappings) == 0 {
			return fmt.Errorf("global model mapping: group %s(%s) mappings is empty", g.Name, g.Id)
		}
		for k, v := range g.Mappings {
			if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
				return fmt.Errorf("global model mapping: group %s has empty key or value", g.Name)
			}
		}
		if !g.Enabled {
			continue
		}
		for k := range g.Mappings {
			if prev, conflict := enabledKeys[k]; conflict {
				return fmt.Errorf("global model mapping: enabled groups conflict on %s (%s vs %s)", k, prev, g.Name)
			}
			enabledKeys[k] = g.Name
		}
	}
	return nil
}

// ValidateGlobalModelMappingGroupsJSONString 只校验不落态(写库前预检用)。
func ValidateGlobalModelMappingGroupsJSONString(jsonStr string) error {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		jsonStr = "[]"
	}
	var groups []GlobalModelMappingGroup
	if err := json.Unmarshal([]byte(jsonStr), &groups); err != nil {
		return fmt.Errorf("invalid global model mapping groups json: %w", err)
	}
	return validateGlobalModelMappingGroups(groups)
}

// UpdateGlobalModelMappingGroupsByJSONString 全量替换组列表(option 更新回调用)。
func UpdateGlobalModelMappingGroupsByJSONString(jsonStr string) error {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		jsonStr = "[]"
	}
	var groups []GlobalModelMappingGroup
	if err := json.Unmarshal([]byte(jsonStr), &groups); err != nil {
		return fmt.Errorf("invalid global model mapping groups json: %w", err)
	}
	if groups == nil {
		groups = []GlobalModelMappingGroup{}
	}
	if err := validateGlobalModelMappingGroups(groups); err != nil {
		return err
	}
	globalModelMappingGroupsValue.Store(groups)
	rebuildActiveGlobalModelMapping(groups)
	return nil
}

// GetGlobalModelMappingGroups 返回组列表副本(API/UI 用)。
func GetGlobalModelMappingGroups() []GlobalModelMappingGroup {
	groups, _ := globalModelMappingGroupsValue.Load().([]GlobalModelMappingGroup)
	out := make([]GlobalModelMappingGroup, len(groups))
	copy(out, groups)
	return out
}

// GetActiveGlobalModelMapping 返回 enabled 组合并后的映射副本(协议层链式用),空返回 nil。
func GetActiveGlobalModelMapping() map[string]string {
	all := globalModelMappingActive.ReadAll()
	if len(all) == 0 {
		return nil
	}
	out := make(map[string]string, len(all))
	for k, v := range all {
		out[k] = v
	}
	return out
}

// ResolveGlobalMappedModel 路由层用:返回全局映射后的模型名(无映射原样返回)。
// 单次查找,不收敛链 — 链式收敛留给协议层 ModelMappedHelper。
func ResolveGlobalMappedModel(name string) string {
	if mapped, ok := globalModelMappingActive.Get(name); ok && mapped != "" {
		return mapped
	}
	return name
}
