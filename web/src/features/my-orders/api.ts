import { api } from '@/lib/api'

import type { CatfkOrderItem, PageInfo } from './types'

// 当前用户的 catfk 订单列表
export async function getMyOrders(
  p = 1,
  page_size = 20
): Promise<{
  success: boolean
  message: string
  data?: PageInfo<CatfkOrderItem>
}> {
  const res = await api.get(`/api/user/orders?p=${p}&page_size=${page_size}`)
  return res.data
}

// 轮询单个订单状态(未发货 → 已发货);返回最新 granted 状态
export async function getOrderStatus(
  tradeNo: string
): Promise<{ success: boolean; message: string; data?: { status: string; kind: string } }> {
  const res = await api.get(
    `/api/user/checkout/status?trade_no=${encodeURIComponent(tradeNo)}`
  )
  return res.data
}
