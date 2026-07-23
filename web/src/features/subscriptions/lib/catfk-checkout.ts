// CatFK checkout 闭环:下单 → 支付链接(支付宝/微信) → 轮询 → 自动兑现。
// 后端为 new-api 原生端点 /api/user/checkout(见 controller/checkout.go),
// 同域调用,凭据走 api 实例的 Authorization header,不再依赖本地 :8390 服务。

import { api } from '@/lib/api'

import { catfkGoodsKeyForPrice } from './catfk-plans'

export type CatfkPayMethod = 'alipay' | 'wechat'

export type CheckoutStart = {
  trade_no: string
  payurl: string
}

export type CheckoutStatus =
  | 'pending'
  | 'paid'
  | 'granted'
  | 'error'
  | 'unknown'

export async function startCheckout(
  goodsKey: string,
  pay: CatfkPayMethod = 'alipay'
): Promise<CheckoutStart> {
  const res = await api.post('/api/user/checkout', {
    goods_key: goodsKey,
    pay,
  })
  const body = res.data
  if (!body?.success) {
    throw new Error(body?.message || 'checkout failed')
  }
  return body.data as CheckoutStart
}

export async function getCheckoutStatus(
  tradeNo: string
): Promise<{ status: CheckoutStatus; kind?: string }> {
  const res = await api.get(
    `/api/user/checkout/status?trade_no=${encodeURIComponent(tradeNo)}`
  )
  const body = res.data
  if (!body?.success) return { status: 'unknown' }
  return { status: body.data?.status || 'unknown', kind: body.data?.kind }
}

/**
 * 发起一次云猫购买:下单、打开支付页、轮询直到自动兑现。
 * onStatus 回调各阶段状态用于 UI 展示。
 */
export async function runCatfkCheckout(options: {
  price: number
  pay?: CatfkPayMethod
  onStatus?: (status: CheckoutStatus) => void
  pollIntervalMs?: number
  timeoutMs?: number
}): Promise<CheckoutStatus> {
  const goodsKey = catfkGoodsKeyForPrice(options.price)
  if (!goodsKey) throw new Error(`no catfk goods for price ${options.price}`)

  const { trade_no, payurl } = await startCheckout(
    goodsKey,
    options.pay ?? 'alipay'
  )
  window.open(payurl, '_blank', 'noopener,noreferrer')

  const interval = options.pollIntervalMs ?? 3000
  const timeout = options.timeoutMs ?? 10 * 60 * 1000
  const start = Date.now()
  options.onStatus?.('pending')
  while (Date.now() - start < timeout) {
    await new Promise((r) => setTimeout(r, interval))
    const { status } = await getCheckoutStatus(trade_no)
    if (status !== 'pending') {
      options.onStatus?.(status)
      return status
    }
  }
  options.onStatus?.('error')
  return 'error'
}
