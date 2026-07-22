// CatFK checkout 闭环:下单 → 支付链接(支付宝/微信) → 轮询 → 自动兑现。
// 后端桥接服务见 scripts/catfk-checkout.py(默认 :8390)。

import { catfkGoodsKeyForPrice } from './catfk-plans'

const CHECKOUT_BASE = 'http://localhost:8390'

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
  jwt: string,
  pay: CatfkPayMethod = 'alipay'
): Promise<CheckoutStart> {
  const resp = await fetch(`${CHECKOUT_BASE}/checkout`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jwt, goods_key: goodsKey, pay }),
  })
  const data = await resp.json()
  if (!resp.ok || data.error) {
    throw new Error(data.error || `checkout failed (${resp.status})`)
  }
  return data
}

export async function getCheckoutStatus(
  tradeNo: string
): Promise<{ status: CheckoutStatus; kind?: string }> {
  const resp = await fetch(
    `${CHECKOUT_BASE}/checkout/status?trade_no=${encodeURIComponent(tradeNo)}`
  )
  const data = await resp.json()
  if (!resp.ok) return { status: 'unknown' }
  return { status: data.status || 'unknown', kind: data.kind }
}

/**
 * 发起一次云猫购买:下单、打开支付页、轮询直到自动兑现。
 * onStatus 回调各阶段状态用于 UI 展示。
 */
export async function runCatfkCheckout(options: {
  price: number
  jwt: string
  pay?: CatfkPayMethod
  onStatus?: (status: CheckoutStatus) => void
  pollIntervalMs?: number
  timeoutMs?: number
}): Promise<CheckoutStatus> {
  const goodsKey = catfkGoodsKeyForPrice(options.price)
  if (!goodsKey) throw new Error(`no catfk goods for price ${options.price}`)

  const { trade_no, payurl } = await startCheckout(
    goodsKey,
    options.jwt,
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
