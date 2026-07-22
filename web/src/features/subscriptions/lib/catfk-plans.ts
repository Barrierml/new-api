// 云猫寄售(catfk.com)购买页映射,沿用 PAR 的商品映射(projects/par/web/src/lib/catfkPlans.ts)。
//
// 套餐卡片的购买按钮引流到云猫完成交易;付款后云猫自动发卡(兑换码)到买家联系方式,
// 买家凭码回本站「钱包 → 兑换码」兑换。两类商品均走云猫:
// - Pro 月付套餐:按 plan.price_amount(CNY)映射到云猫月付 goods。
// - 按量付费(买量):4 个固定档位 ¥20/50/100/200。

export const CATFK_PLATFORM = '云猫寄售'

// --- Pro 月付 ---
// 月付套餐价格与云猫在售档位一致(¥59/119/199/329/499/749)。
// 若云猫 goods 上下架变动,同步更新此表。
export const CATFK_MONTHLY_LINK_BY_PRICE: Record<number, string> = {
  59: 'https://catfk.com/item/vk898s', // Mini
  119: 'https://catfk.com/item/e0b3y5', // Pro mini
  199: 'https://catfk.com/item/r07y8g', // Pro x1
  329: 'https://catfk.com/item/uhwx0f', // Pro x2
  499: 'https://catfk.com/item/bx9j3s', // Pro x3
  749: 'https://catfk.com/item/snae3x', // Pro x4
}

// 按月付套餐价格查云猫购买页;未命中返回 undefined。
export function catfkLinkForPrice(price: number): string | undefined {
  return CATFK_MONTHLY_LINK_BY_PRICE[Math.round(price)]
}

// 套餐价格 -> 云猫 goods_key(自动 checkout 用)。
const CATFK_GOODS_KEY_BY_PRICE: Record<number, string> = {
  59: 'vk898s',
  119: 'e0b3y5',
  199: 'r07y8g',
  329: 'uhwx0f',
  499: 'bx9j3s',
  749: 'snae3x',
  20: 'r5ufqm',
  50: 'ot5e6z',
  100: 'jyq5ae',
  200: 'paibsa',
}

export function catfkGoodsKeyForPrice(price: number): string | undefined {
  return CATFK_GOODS_KEY_BY_PRICE[Math.round(price)]
}

// --- 按量付费(买量)档位 ---
export const CATFK_TOPUP_LINK_BY_PRICE: Record<number, string> = {
  20: 'https://catfk.com/item/r5ufqm',
  50: 'https://catfk.com/item/ot5e6z',
  100: 'https://catfk.com/item/jyq5ae',
  200: 'https://catfk.com/item/paibsa',
}

// 档位价格升序,用于渲染金额选择器。
export const CATFK_TOPUP_PRICES: number[] = Object.keys(
  CATFK_TOPUP_LINK_BY_PRICE
)
  .map(Number)
  .sort((a, b) => a - b)
