// 「自动补货」看板的类型,与后端 controller/catfk_replenish.go 响应对齐。

export interface ReplenisherStatus {
  enabled: boolean
  is_master: boolean
  interval_sec: number
  last_run_at: number // unix, 0=从未跑过
  low_water: number
  batch: number
}

export interface ReplenishStockRow {
  goods_key: string
  kind: string // "plan" | "quota"
  available: number // -1 = 查询失败
  low_water: number
}

export interface RedemptionHealth {
  valid: number
  used: number
  disabled: number
  expired: number
}

export interface ReplenishOverview {
  status: ReplenisherStatus
  stock: ReplenishStockRow[]
  health: RedemptionHealth
  orphan_count: number // -1 = 未统计
  generated_at: number
  catfk_error?: string
}

export interface ReplenishEvent {
  id: number
  goods_key: string
  kind: string
  value: number
  codes_generated: number
  catfk_upload_ok: boolean
  error_message: string
  trigger: string // "auto" | "purchase" | "manual"
  created_at: number
}

// 分页响应(new-api 通用 pageInfo 信封)
export interface PageInfo<T> {
  items?: T[]
  total?: number
  page?: number
  page_size?: number
}

export interface ReplenishConfig {
  replenish_enabled?: boolean
  low_water?: number
  batch?: number
}
