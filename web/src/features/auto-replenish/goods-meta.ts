// CatFK 商品元数据 — 与后端 service/catfk/client.go Goods map 一一对应。
// 改这里前先看后端,保持同步。
import i18next from 'i18next'

export interface GoodsMeta {
  /** 商品显示名(中文,i18n t() 函数处理英文) */
  name: string
  /** 价格标签(人民币) */
  price: string
  /** plan | quota */
  kind: 'plan' | 'quota'
  /** 描述(如 ¥20 → $60) */
  description?: string
}

// 后端 Goods map 没有名称/价格字段,这里维护前端展示元数据
export const CATFK_GOODS_META: Record<string, GoodsMeta> = {
  vk898s: { name: 'Mini', price: '¥59', kind: 'plan' },
  e0b3y5: { name: 'Pro mini', price: '¥119', kind: 'plan' },
  r07y8g: { name: 'Pro x1', price: '¥199', kind: 'plan' },
  uhwx0f: { name: 'Pro x2', price: '¥329', kind: 'plan' },
  bx9j3s: { name: 'Pro x3', price: '¥499', kind: 'plan' },
  snae3x: { name: 'Pro x4', price: '¥749', kind: 'plan' },
  cbcg11: {
    name: 'Quota ¥5 (test)',
    price: '¥5',
    kind: 'quota',
    description: '$15',
  },
  r5ufqm: {
    name: 'Quota ¥20',
    price: '¥20',
    kind: 'quota',
    description: '$60',
  },
  ot5e6z: {
    name: 'Quota ¥50',
    price: '¥50',
    kind: 'quota',
    description: '$150',
  },
  jyq5ae: {
    name: 'Quota ¥100',
    price: '¥100',
    kind: 'quota',
    description: '$300',
  },
  paibsa: {
    name: 'Quota ¥200',
    price: '¥200',
    kind: 'quota',
    description: '$600',
  },
}

/** 取商品显示名(找不到就回退 goods_key) */
export function getGoodsDisplayName(goodsKey: string): string {
  const meta = CATFK_GOODS_META[goodsKey]
  if (!meta) return goodsKey
  // i18n:英文用户看到 "Mini / Pro x1",中文用户看到一样(名字本身是英文 SKU)
  return i18next.t(meta.name)
}

/** 取商品价格标签 */
export function getGoodsPriceLabel(goodsKey: string): string {
  return CATFK_GOODS_META[goodsKey]?.price ?? ''
}

/** 取商品完整描述(Mini · ¥59) */
export function getGoodsFullLabel(goodsKey: string): string {
  const meta = CATFK_GOODS_META[goodsKey]
  if (!meta) return goodsKey
  const desc = meta.description ? ` → ${meta.description}` : ''
  return `${i18next.t(meta.name)} · ${meta.price}${desc}`
}
