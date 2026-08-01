// 全局临时模型映射组(运维区独立功能页)。
// 后端:options 表 key=GlobalModelMappingGroups,enabled 组合并进内存 map,
// distributor 选渠道前 + ModelMappedHelper 协议层共用同一份配置。
export interface ModelMappingGroup {
  id: string
  name: string
  enabled: boolean
  mappings: Record<string, string>
  created_at: number
  updated_at: number
}
