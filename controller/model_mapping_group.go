package controller

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// 全局临时模型映射组管理 API(运维页「模型映射」)。
//
// 存储:options 表 key=GlobalModelMappingGroups(JSON 组列表),mutation =
// 改内存态 → 写 options 表(与 updateOption 同语义:先 DB 成功再更内存,
// 这里反过来 — 先 validate+暂存,DB 写成功才更新内存,失败整体不落)。
// 运行时:enabled 组 mappings 合并进内存 map,distributor/relay 直接查。

const globalModelMappingGroupsOptionKey = "GlobalModelMappingGroups"

func newGlobalModelMappingGroupId() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "grp_" + hex.EncodeToString(b)
}

// persistGlobalModelMappingGroups 校验 + 写 options 表 + 更内存态。
// model.UpdateOption 内部先写 DB,成功才 dispatch updateOptionMap(其
// case GlobalModelMappingGroups 调 validate + 更内存),DB 失败整体不落。
func persistGlobalModelMappingGroups(c *gin.Context, groups []ratio_setting.GlobalModelMappingGroup) bool {
	raw, err := common.Marshal(groups)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if err := model.UpdateOption(globalModelMappingGroupsOptionKey, string(raw)); err != nil {
		common.ApiError(c, err)
		return false
	}
	return true
}

// GetGlobalModelMappingGroups GET /api/model_mapping_groups
func GetGlobalModelMappingGroups(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    ratio_setting.GetGlobalModelMappingGroups(),
	})
}

type globalModelMappingGroupRequest struct {
	Name     string            `json:"name" binding:"required"`
	Mappings map[string]string `json:"mappings" binding:"required"`
}

// AddGlobalModelMappingGroup POST /api/model_mapping_groups
func AddGlobalModelMappingGroup(c *gin.Context) {
	var req globalModelMappingGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	now := time.Now().Unix()
	group := ratio_setting.GlobalModelMappingGroup{
		Id:        newGlobalModelMappingGroupId(),
		Name:      req.Name,
		Enabled:   false, // 新建默认禁用,显式启用才生效
		Mappings:  req.Mappings,
		CreatedAt: now,
		UpdatedAt: now,
	}
	groups := append(ratio_setting.GetGlobalModelMappingGroups(), group)
	if !persistGlobalModelMappingGroups(c, groups) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": group})
}

// UpdateGlobalModelMappingGroup PUT /api/model_mapping_groups/:id 改 name/mappings
func UpdateGlobalModelMappingGroup(c *gin.Context) {
	id := c.Param("id")
	var req globalModelMappingGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	groups := ratio_setting.GetGlobalModelMappingGroups()
	found := false
	for i := range groups {
		if groups[i].Id == id {
			groups[i].Name = req.Name
			groups[i].Mappings = req.Mappings
			groups[i].UpdatedAt = time.Now().Unix()
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "mapping group not found"})
		return
	}
	if !persistGlobalModelMappingGroups(c, groups) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// UpdateGlobalModelMappingGroupStatus PUT /api/model_mapping_groups/:id/status {enabled}
// 一键启停 — 快速切换主路径。
func UpdateGlobalModelMappingGroupStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	groups := ratio_setting.GetGlobalModelMappingGroups()
	found := false
	for i := range groups {
		if groups[i].Id == id {
			groups[i].Enabled = req.Enabled
			groups[i].UpdatedAt = time.Now().Unix()
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "mapping group not found"})
		return
	}
	if !persistGlobalModelMappingGroups(c, groups) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// DeleteGlobalModelMappingGroup DELETE /api/model_mapping_groups/:id
func DeleteGlobalModelMappingGroup(c *gin.Context) {
	id := c.Param("id")
	groups := ratio_setting.GetGlobalModelMappingGroups()
	out := make([]ratio_setting.GlobalModelMappingGroup, 0, len(groups))
	found := false
	for _, g := range groups {
		if g.Id == id {
			found = true
			continue
		}
		out = append(out, g)
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "mapping group not found"})
		return
	}
	if !persistGlobalModelMappingGroups(c, out) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
