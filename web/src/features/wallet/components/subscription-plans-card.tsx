/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Crown, RefreshCw, Sparkles, Check } from 'lucide-react'
import { useState, useEffect, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StatusBadge,
  dotColorMap,
  textColorMap,
} from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
  updateBillingPreference,
} from '@/features/subscriptions/api'
import {
  runCatfkCheckout,
  type CatfkPayMethod,
} from '@/features/subscriptions/lib/catfk-checkout'
import { catfkGoodsKeyForPrice } from '@/features/subscriptions/lib/catfk-plans'
import { formatDuration, formatResetPeriod } from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  SubQuotaUsage,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { formatPlanPrice, formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import type { TopupInfo } from '../types'

interface SubscriptionPlansCardProps {
  topupInfo: TopupInfo | null
  onAvailabilityChange?: (available: boolean) => void
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

function getBillingPreferenceLabel(
  preference: string,
  t: (key: string) => string
): string {
  switch (preference) {
    case 'subscription_first':
      return t('Subscription First')
    case 'wallet_first':
      return t('Wallet First')
    case 'subscription_only':
      return t('Subscription Only')
    case 'wallet_only':
      return t('Wallet Only')
    default:
      return preference
  }
}

interface SubQuotaLimitItem {
  name?: string
  period_unit?: string
  period_value?: number
  limit_usd?: number
}

function parseSubQuotaLimits(raw?: string): SubQuotaLimitItem[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function SubQuotaUsageList({ usages }: { usages: SubQuotaUsage[] }) {
  const { t } = useTranslation()
  return (
    <div className='mt-2 space-y-2'>
      {usages.map((u) => {
        const usagePct = Math.min(100, Math.round(u.percent || 0))
        const key = `${u.name || 'sub'}-${u.window_start || 0}-${u.window_end || 0}`
        return (
          <div key={key} className='rounded-md border p-2'>
            <div className='flex items-center justify-between text-xs'>
              <span className='font-medium'>{u.name || t('Sub Limit')}</span>
              {u.exceeded && (
                <span className='text-destructive font-medium'>
                  {t('Exceeded')}
                </span>
              )}
            </div>
            <div className='text-muted-foreground mt-1 text-xs'>
              {t('Used')} ${(u.used_usd || 0).toFixed(2)} / $
              {(u.limit_usd || 0).toFixed(2)} · {t('Remaining')} $
              {(u.remaining_usd || 0).toFixed(2)} ({usagePct}%)
              {u.reset_time > 0 &&
                ` · ${t('Next reset')}: ${new Date(u.reset_time * 1000).toLocaleString()}`}
            </div>
            <Progress value={usagePct} className='mt-1.5 h-1' />
          </div>
        )
      })}
    </div>
  )
}

export function SubscriptionPlansCard({
  onAvailabilityChange,
  onPurchaseSuccess,
}: SubscriptionPlansCardProps) {
  const { t } = useTranslation()

  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [activeSubscriptions, setActiveSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [billingPreference, setBillingPreference] =
    useState('subscription_first')
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  // `${planId}:${pay}` 标记正在结算的套餐 + 支付方式
  const [checkoutKey, setCheckoutKey] = useState<string | null>(null)
  // 当前打开购买弹窗的套餐
  const [purchasePlan, setPurchasePlan] = useState<PlanRecord['plan'] | null>(
    null
  )
  const accessToken = useAuthStore((s) => s.auth.accessToken)

  const fetchPlans = useCallback(async () => {
    try {
      const res = await getPublicPlans()
      if (res.success) {
        setPlans(res.data || [])
      }
    } catch {
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = await getSelfSubscriptionFull()
      if (res.success && res.data) {
        setBillingPreference(
          res.data.billing_preference || 'subscription_first'
        )
        setActiveSubscriptions(res.data.subscriptions || [])
        setAllSubscriptions(res.data.all_subscriptions || [])
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await fetchSelfSubscription()
    } finally {
      setRefreshing(false)
    }
  }

  const handleCatfkCheckout = useCallback(
    async (plan: PlanRecord['plan'], pay: CatfkPayMethod) => {
      if (!plan || checkoutKey !== null) return
      if (!accessToken) {
        toast.error(t('Please log in first'))
        return
      }
      setCheckoutKey(`${plan.id}:${pay}`)
      try {
        toast.info(t('Opening payment page, please complete payment'))
        const status = await runCatfkCheckout({
          price: Number(plan.price_amount || 0),
          jwt: accessToken,
          pay,
        })
        if (status === 'granted') {
          toast.success(t('Payment received, subscription activated!'))
          setPurchasePlan(null)
          fetchSelfSubscription()
          onPurchaseSuccess?.()
        } else {
          toast.warning(
            t(
              'Payment not detected yet, it will activate automatically once confirmed'
            )
          )
        }
      } catch (e) {
        toast.error(e instanceof Error ? e.message : t('Checkout failed'))
      } finally {
        setCheckoutKey(null)
      }
    },
    [accessToken, checkoutKey, fetchSelfSubscription, onPurchaseSuccess, t]
  )

  const handlePreferenceChange = async (pref: string) => {
    const previous = billingPreference
    setBillingPreference(pref)
    try {
      const res = await updateBillingPreference(pref)
      if (res.success) {
        toast.success(t('Updated successfully'))
        const normalized = res.data?.billing_preference || pref
        setBillingPreference(normalized)
      } else {
        toast.error(res.message || t('Update failed'))
        setBillingPreference(previous)
      }
    } catch {
      toast.error(t('Request failed'))
      setBillingPreference(previous)
    }
  }

  const hasActive = activeSubscriptions.length > 0
  const hasAny = allSubscriptions.length > 0
  const isAvailable = loading || plans.length > 0 || hasAny
  const disablePref = !hasActive
  const isSubPref =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only'
  const displayPref =
    disablePref && isSubPref ? 'wallet_first' : billingPreference

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const sub of allSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (!planId) continue
      map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [allSubscriptions])

  useEffect(() => {
    onAvailabilityChange?.(isAvailable)
  }, [isAvailable, onAvailabilityChange])

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id) {
        map.set(p.plan.id, p.plan.title || '')
      }
    }
    return map
  }, [plans])

  // 价格降序(高 → 低)
  const sortedPlans = useMemo(
    () =>
      [...plans].sort(
        (a, b) =>
          Number(b?.plan?.price_amount || 0) - Number(a?.plan?.price_amount || 0)
      ),
    [plans]
  )

  const buildBenefits = useCallback(
    (plan: NonNullable<PlanRecord['plan']>): string[] => {
      const totalAmount = Number(plan.total_amount || 0)
      const limit = Number(plan.max_purchase_per_user || 0)
      const subLimits = parseSubQuotaLimits(plan.sub_quota_limits)
      return [
        `${t('Validity Period')}: ${formatDuration(plan, t)}`,
        formatResetPeriod(plan, t) !== t('No Reset')
          ? `${t('Quota Reset')}: ${formatResetPeriod(plan, t)}`
          : null,
        totalAmount > 0
          ? `${t('Total Quota')}: ${formatQuota(totalAmount)}`
          : `${t('Total Quota')}: ${t('Unlimited')}`,
        ...subLimits.map(
          (s) =>
            `${s.name || t('Sub Limit')}: $${Number(s.limit_usd || 0).toFixed(0)}`
        ),
        limit > 0 ? `${t('Purchase Limit')}: ${limit}` : null,
      ].filter(Boolean) as string[]
    },
    [t]
  )

  const getRemainingDays = (sub: UserSubscriptionRecord) => {
    const endTime = sub?.subscription?.end_time || 0
    if (!endTime) return 0
    const now = Date.now() / 1000
    return Math.max(0, Math.ceil((endTime - now) / 86400))
  }

  const getUsagePercent = (sub: UserSubscriptionRecord) => {
    const total = Number(sub?.subscription?.amount_total || 0)
    const used = Number(sub?.subscription?.amount_used || 0)
    if (total <= 0) return 0
    return Math.round((used / total) * 100)
  }

  if (loading) {
    return (
      <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
          <Skeleton className='h-6 w-32' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:p-5'>
          <Skeleton className='h-20 w-full' />
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
            {['first', 'second', 'third'].map((key) => (
              <Skeleton key={key} className='h-48 w-full' />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (plans.length === 0 && !hasAny) {
    return null
  }

  return (
    <TitledCard
      title={t('Subscription Plans')}
      description={t('Subscribe to a plan for model access')}
      icon={<Crown className='h-4 w-4' />}
      iconTone='warning'
      disableHoverEffect
      contentClassName='space-y-4 sm:space-y-5'
    >
      {/* 我的订阅 & 计费偏好 */}
      <div className='rounded-xl border p-3 sm:p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2.5 sm:gap-3'>
          <div className='flex min-w-0 flex-wrap items-center gap-2'>
            <span className='text-sm font-medium'>{t('My Subscriptions')}</span>
            <span className='flex items-center gap-1.5 text-xs font-medium'>
              <span
                className={cn(
                  'size-1.5 shrink-0 rounded-full',
                  hasActive ? dotColorMap.success : dotColorMap.neutral
                )}
                aria-hidden='true'
              />
              {hasActive ? (
                <span className={cn(textColorMap.success)}>
                  {activeSubscriptions.length} {t('active')}
                </span>
              ) : (
                <span className='text-muted-foreground'>{t('No Active')}</span>
              )}
              {allSubscriptions.length > activeSubscriptions.length && (
                <>
                  <span className='text-muted-foreground/30'>·</span>
                  <span className='text-muted-foreground'>
                    {allSubscriptions.length - activeSubscriptions.length}{' '}
                    {t('expired')}
                  </span>
                </>
              )}
            </span>
          </div>
          <div className='flex w-full items-center gap-2 sm:w-auto'>
            <Select
              items={[
                {
                  value: 'subscription_first',
                  label: (
                    <>
                      {getBillingPreferenceLabel('subscription_first', t)}
                      {disablePref ? ` (${t('No Active')})` : ''}
                    </>
                  ),
                },
                {
                  value: 'wallet_first',
                  label: getBillingPreferenceLabel('wallet_first', t),
                },
                {
                  value: 'subscription_only',
                  label: (
                    <>
                      {getBillingPreferenceLabel('subscription_only', t)}
                      {disablePref ? ` (${t('No Active')})` : ''}
                    </>
                  ),
                },
                {
                  value: 'wallet_only',
                  label: getBillingPreferenceLabel('wallet_only', t),
                },
              ]}
              value={displayPref}
              onValueChange={(v) => v !== null && handlePreferenceChange(v)}
            >
              <SelectTrigger className='h-8 flex-1 text-xs sm:w-[140px] sm:flex-none'>
                <SelectValue>
                  {getBillingPreferenceLabel(displayPref, t)}
                </SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='subscription_first' disabled={disablePref}>
                    {getBillingPreferenceLabel('subscription_first', t)}
                    {disablePref ? ` (${t('No Active')})` : ''}
                  </SelectItem>
                  <SelectItem value='wallet_first'>
                    {getBillingPreferenceLabel('wallet_first', t)}
                  </SelectItem>
                  <SelectItem value='subscription_only' disabled={disablePref}>
                    {getBillingPreferenceLabel('subscription_only', t)}
                    {disablePref ? ` (${t('No Active')})` : ''}
                  </SelectItem>
                  <SelectItem value='wallet_only'>
                    {getBillingPreferenceLabel('wallet_only', t)}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8'
              onClick={handleRefresh}
              disabled={refreshing}
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
              />
            </Button>
          </div>
        </div>

        {disablePref && isSubPref && (
          <p className='text-muted-foreground mt-2 text-xs'>
            {t(
              'Preference saved as {{pref}}, but no active subscription. Wallet will be used automatically.',
              {
                pref:
                  billingPreference === 'subscription_only'
                    ? t('Subscription Only')
                    : t('Subscription First'),
              }
            )}
          </p>
        )}

        {hasAny && (
          <>
            <Separator className='my-3' />
            <div className='max-h-64 space-y-3 overflow-y-auto pr-1'>
              {allSubscriptions.map((sub) => {
                const subscription = sub.subscription
                const totalAmount = Number(subscription?.amount_total || 0)
                const usedAmount = Number(subscription?.amount_used || 0)
                const remainAmount =
                  totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0
                const planTitle = planTitleMap.get(subscription?.plan_id) || ''
                const remainDays = getRemainingDays(sub)
                const usagePercent = getUsagePercent(sub)
                const now = Date.now() / 1000
                const isExpired = (subscription?.end_time || 0) < now
                const isCancelled = subscription?.status === 'cancelled'
                const isActive =
                  subscription?.status === 'active' && !isExpired
                const nextResetTime = subscription?.next_reset_time ?? 0
                let statusBadge = (
                  <StatusBadge
                    label={t('Expired')}
                    variant='neutral'
                    copyable={false}
                  />
                )
                if (isActive) {
                  statusBadge = (
                    <StatusBadge
                      label={t('Active')}
                      variant='success'
                      copyable={false}
                    />
                  )
                } else if (isCancelled) {
                  statusBadge = (
                    <StatusBadge
                      label={t('Cancelled')}
                      variant='neutral'
                      copyable={false}
                    />
                  )
                }

                let endTimeLabel = t('Expired at')
                if (isActive) {
                  endTimeLabel = t('Until')
                } else if (isCancelled) {
                  endTimeLabel = t('Cancelled at')
                }

                return (
                  <div
                    key={subscription?.id}
                    className='bg-background rounded-md border p-3 text-xs'
                  >
                    <div className='flex items-center justify-between'>
                      <div className='flex items-center gap-2'>
                        <span className='font-medium'>
                          {planTitle
                            ? `${planTitle} · ${t('Subscription')} #${subscription?.id}`
                            : `${t('Subscription')} #${subscription?.id}`}
                        </span>
                        {statusBadge}
                      </div>
                      {isActive && (
                        <span className='text-muted-foreground'>
                          {t('{{count}} days remaining', {
                            count: remainDays,
                          })}
                        </span>
                      )}
                    </div>
                    <div className='text-muted-foreground mt-1.5'>
                      {endTimeLabel}{' '}
                      {new Date(
                        (subscription?.end_time || 0) * 1000
                      ).toLocaleString()}
                    </div>
                    {isActive && nextResetTime > 0 && (
                      <div className='text-muted-foreground mt-1'>
                        {t('Next reset')}:{' '}
                        {new Date(nextResetTime * 1000).toLocaleString()}
                      </div>
                    )}
                    <div className='text-muted-foreground mt-1'>
                      {t('Total Quota')}:{' '}
                      {totalAmount > 0 ? (
                        <Tooltip>
                          <TooltipTrigger
                            render={<span className='cursor-help' />}
                          >
                            {formatQuota(usedAmount)}/
                            {formatQuota(totalAmount)} · {t('Remaining')}{' '}
                            {formatQuota(remainAmount)}
                          </TooltipTrigger>
                          <TooltipContent>
                            {t('Raw Quota')}: {usedAmount}/{totalAmount} ·{' '}
                            {t('Remaining')} {remainAmount}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        t('Unlimited')
                      )}
                      {totalAmount > 0 && (
                        <span className='ml-2'>
                          {t('Used')} {usagePercent}%
                        </span>
                      )}
                    </div>
                    {totalAmount > 0 && isActive && (
                      <Progress value={usagePercent} className='mt-2 h-1.5' />
                    )}
                    {isActive &&
                      sub?.sub_quota_usage &&
                      sub.sub_quota_usage.length > 0 && (
                        <SubQuotaUsageList usages={sub.sub_quota_usage} />
                      )}
                  </div>
                )
              })}
            </div>
          </>
        )}

        {!hasAny && (
          <p className='text-muted-foreground mt-2 text-xs'>
            {t('Subscribe to a plan for model access')}
          </p>
        )}
      </div>

      {/* 可购买套餐(价格降序) */}
      {sortedPlans.length > 0 ? (
        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:gap-4'>
          {sortedPlans.map((p) => {
            const plan = p?.plan
            if (!plan) return null
            const isPopular = plan.title === 'Pro x1'
            const limit = Number(plan.max_purchase_per_user || 0)
            const count = planPurchaseCountMap.get(plan.id) || 0
            const reached = limit > 0 && count >= limit
            const purchasable = Boolean(
              catfkGoodsKeyForPrice(Number(plan.price_amount || 0))
            )
            const benefits = buildBenefits(plan)

            return (
              <Card
                key={plan.id}
                data-card-hover='false'
                className={cn(isPopular && 'border-primary/70 shadow-sm')}
              >
                <CardContent className='flex h-full flex-col p-3.5 sm:p-4'>
                  <div className='mb-2 flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <h4 className='truncate font-semibold'>
                        {plan.title || t('Subscription Plans')}
                      </h4>
                      {plan.subtitle && (
                        <p className='text-muted-foreground truncate text-xs'>
                          {plan.subtitle}
                        </p>
                      )}
                    </div>
                    {isPopular && (
                      <StatusBadge
                        variant='info'
                        copyable={false}
                        className='shrink-0'
                      >
                        <Sparkles className='h-3 w-3' />
                        {t('Recommended')}
                      </StatusBadge>
                    )}
                  </div>

                  <div className='flex items-baseline gap-1 py-2'>
                    <span className='text-primary text-2xl font-bold'>
                      {formatPlanPrice(
                        Number(plan.price_amount || 0),
                        plan.currency
                      )}
                    </span>
                    <span className='text-muted-foreground text-xs'>
                      /{t('month')}
                    </span>
                  </div>

                  <div className='flex-1 space-y-1.5 pb-3'>
                    {benefits.map((label) => (
                      <div
                        key={label}
                        className='text-muted-foreground flex items-center gap-2 text-xs'
                      >
                        <Check className='text-primary h-3 w-3 shrink-0' />
                        <span>{label}</span>
                      </div>
                    ))}
                  </div>

                  <Separator className='mb-3' />

                  {reached ? (
                    <Tooltip>
                      <TooltipTrigger render={<div />}>
                        <Button variant='outline' className='w-full' disabled>
                          {t('Limit Reached')}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>
                        {t('Purchase limit reached')} ({count}/{limit})
                      </TooltipContent>
                    </Tooltip>
                  ) : purchasable ? (
                    <Button
                      className='w-full'
                      onClick={() => setPurchasePlan(plan)}
                    >
                      {t('Buy Now')}
                    </Button>
                  ) : (
                    <Tooltip>
                      <TooltipTrigger render={<div />}>
                        <Button variant='outline' className='w-full' disabled>
                          {t('Coming Soon')}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>
                        {t('This plan is not available for purchase yet.')}
                      </TooltipContent>
                    </Tooltip>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </div>
      ) : (
        <p className='text-muted-foreground py-4 text-center text-sm'>
          {t('No plans available')}
        </p>
      )}
      {plans.length > 0 && (
        <p className='text-muted-foreground mt-3 text-xs'>
          {t(
            'After payment succeeds, your plan activates automatically — no redemption code needed. Payments are processed by CatFK.'
          )}
        </p>
      )}

      {/* 购买弹窗:套餐介绍 + 支付方式选择 */}
      <Dialog
        open={purchasePlan !== null}
        onOpenChange={(open) => {
          if (!open && checkoutKey === null) setPurchasePlan(null)
        }}
      >
        <DialogContent className='sm:max-w-md'>
          {purchasePlan && (
            <>
              <DialogHeader>
                <DialogTitle>{purchasePlan.title}</DialogTitle>
                {purchasePlan.subtitle && (
                  <DialogDescription>
                    {purchasePlan.subtitle}
                  </DialogDescription>
                )}
              </DialogHeader>

              <div className='flex items-baseline gap-1 py-1'>
                <span className='text-primary text-3xl font-bold'>
                  {formatPlanPrice(
                    Number(purchasePlan.price_amount || 0),
                    purchasePlan.currency
                  )}
                </span>
                <span className='text-muted-foreground text-sm'>
                  /{t('month')}
                </span>
              </div>

              <div className='space-y-2 pb-2'>
                {buildBenefits(purchasePlan).map((label) => (
                  <div
                    key={label}
                    className='text-muted-foreground flex items-center gap-2 text-sm'
                  >
                    <Check className='text-primary h-3.5 w-3.5 shrink-0' />
                    <span>{label}</span>
                  </div>
                ))}
              </div>

              <Separator />

              <div className='space-y-2 pt-1'>
                <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                  {t('Payment Method')}
                </div>
                <div className='flex flex-col gap-2 sm:flex-row'>
                  {(
                    [
                      ['alipay', t('Pay with Alipay'), 'default'],
                      ['wechat', t('Pay with WeChat'), 'outline'],
                    ] as const
                  ).map(([pay, label, variant]) => {
                    const key = `${purchasePlan.id}:${pay}`
                    const busy = checkoutKey === key
                    return (
                      <Button
                        key={pay}
                        variant={variant}
                        className='flex-1'
                        disabled={checkoutKey !== null}
                        onClick={() => handleCatfkCheckout(purchasePlan, pay)}
                      >
                        {busy ? t('Waiting for payment...') : label}
                      </Button>
                    )
                  })}
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'After payment succeeds, your plan activates automatically — no redemption code needed. Payments are processed by CatFK.'
                  )}
                </p>
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>
    </TitledCard>
  )
}
