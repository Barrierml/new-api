import { api } from '@/lib/api'

import type { ModelMappingGroup } from './types'

export async function getModelMappingGroups(): Promise<{
  success: boolean
  message: string
  data?: ModelMappingGroup[]
}> {
  const res = await api.get('/api/model_mapping_groups/')
  return res.data
}

export async function createModelMappingGroup(payload: {
  name: string
  mappings: Record<string, string>
}): Promise<{ success: boolean; message: string; data?: ModelMappingGroup }> {
  const res = await api.post('/api/model_mapping_groups/', payload)
  return res.data
}

export async function updateModelMappingGroup(
  id: string,
  payload: { name: string; mappings: Record<string, string> }
): Promise<{ success: boolean; message: string }> {
  const res = await api.put(`/api/model_mapping_groups/${id}`, payload)
  return res.data
}

// 一键启停 — 快速切换主路径(故障时秒切,恢复时秒回滚)。
export async function setModelMappingGroupStatus(
  id: string,
  enabled: boolean
): Promise<{ success: boolean; message: string }> {
  const res = await api.put(`/api/model_mapping_groups/${id}/status`, { enabled })
  return res.data
}

export async function deleteModelMappingGroup(
  id: string
): Promise<{ success: boolean; message: string }> {
  const res = await api.delete(`/api/model_mapping_groups/${id}`)
  return res.data
}
