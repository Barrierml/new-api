// 我的订单 — 类型定义,与后端 model.CatfkOrder + controller.ListMyCatfkOrders 响应对齐。

export interface CatfkOrderItem {
  id: number
  trade_no: string
  user_id: number
  goods_key: string
  kind: string // "plan" | "quota"
  value: number // plan_id 或 quota 数
  pay: string // "alipay" | "wechat"
  payurl: string
  granted: boolean
  cards: string // JSON 数组字符串,已作废的兑换码
  created_at: number // unix 秒
}

// 分页信封
export interface PageInfo<T> {
  items?: T[]
  total?: number
  page?: number
  page_size?: number
}
