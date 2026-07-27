import { api } from '@/lib/api'

import type {
  PageInfo,
  ReplenishConfig,
  ReplenishEvent,
  ReplenishOverview,
} from './types'

// 概览:daemon 状态 + 各商品库存 + 兑换码健康度 + 孤儿码数。后端缓存 5min 降频。
export async function getReplenishOverview(): Promise<{
  success: boolean
  message: string
  data?: ReplenishOverview
}> {
  const res = await api.get('/api/catfk-replenish/overview')
  return res.data
}

// 补货事件历史(分页)。
export async function getReplenishEvents(
  p = 1,
  page_size = 20
): Promise<{ success: boolean; message: string; data?: PageInfo<ReplenishEvent> }> {
  const res = await api.get(
    `/api/catfk-replenish/events?p=${p}&page_size=${page_size}`
  )
  return res.data
}

// 手动触发补货。goods_key 为空 → 全量巡检;否则对该商品强制补一批。
export async function runReplenish(
  goods_key = ''
): Promise<{ success: boolean; message: string; data?: { generated: number; goods_key: string } }> {
  const res = await api.post('/api/catfk-replenish/run', { goods_key })
  return res.data
}

// 热更新开关/水位/批量。
export async function updateReplenishConfig(
  config: ReplenishConfig
): Promise<{ success: boolean; message: string; data?: { updated: boolean } }> {
  const res = await api.put('/api/catfk-replenish/config', config)
  return res.data
}
