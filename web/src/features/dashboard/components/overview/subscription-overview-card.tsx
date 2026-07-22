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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowRight, CalendarDays, Crown, Gauge, TimerReset } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import type { SubQuotaUsage } from '@/features/subscriptions/types'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

function formatWindowUsd(usd: number): string {
  // 小额用量保留 4 位,避免 $0.002 显示成 $0.00 看起来像没计费
  return usd > 0 && usd < 0.01 ? usd.toFixed(4) : usd.toFixed(2)
}

function formatDateTime(unix: number): string {
  return new Date(unix * 1000).toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function SubscriptionOverviewCard() {
  const { t } = useTranslation()

  const { data: subData, isLoading: subLoading } = useQuery({
    queryKey: ['overview-self-subscription'],
    queryFn: async () => {
      const res = await getSelfSubscriptionFull()
      if (!res.success) throw new Error(res.message || 'failed')
      return res.data
    },
    staleTime: 30_000,
  })

  const { data: plans } = useQuery({
    queryKey: ['overview-public-plans'],
    queryFn: async () => {
      const res = await getPublicPlans()
      return res.success ? res.data || [] : []
    },
    staleTime: 60_000,
  })

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans || []) {
      if (p?.plan?.id) map.set(p.plan.id, p.plan.title || '')
    }
    return map
  }, [plans])

  if (subLoading) {
    return (
      <div className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <Skeleton className='h-5 w-32' />
        <Skeleton className='mt-3 h-20 w-full' />
      </div>
    )
  }

  const subscriptions = subData?.subscriptions || []
  const active = subscriptions.find(
    (s) =>
      s?.subscription?.status === 'active' &&
      (s.subscription.end_time || 0) > Date.now() / 1000
  )

  // 未开通:引导卡片
  if (!active) {
    return (
      <div className='bg-card relative overflow-hidden rounded-2xl border p-4 shadow-xs sm:p-5'>
        <div
          className='from-warning/10 pointer-events-none absolute inset-0 bg-gradient-to-br via-transparent to-transparent'
          aria-hidden='true'
        />
        <div className='relative flex flex-wrap items-center justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-3'>
            <span className='bg-warning/15 text-warning flex size-10 shrink-0 items-center justify-center rounded-xl shadow-xs'>
              <Crown className='size-5' aria-hidden='true' />
            </span>
            <div className='min-w-0'>
              <h3 className='truncate text-sm font-semibold'>
                {t('Subscription Plans')}
              </h3>
              <p className='text-muted-foreground line-clamp-1 text-xs'>
                {t(
                  'No active plan. Subscribe for weekly quota and better rates.'
                )}
              </p>
            </div>
          </div>
          <Button size='sm' render={<Link to='/wallet' />}>
            {t('View Plans')}
            <ArrowRight data-icon='inline-end' />
          </Button>
        </div>
      </div>
    )
  }

  const sub = active.subscription
  const planTitle = planTitleMap.get(sub?.plan_id || 0) || ''
  const totalAmount = Number(sub?.amount_total || 0)
  const usedAmount = Number(sub?.amount_used || 0)
  const usagePct =
    totalAmount > 0
      ? Math.min(100, Math.round((usedAmount / totalAmount) * 100))
      : 0
  const remainDays = Math.max(
    0,
    Math.ceil(((sub?.end_time || 0) - Date.now() / 1000) / 86400)
  )
  const nextResetTime = sub?.next_reset_time ?? 0
  const fiveHour = (active.sub_quota_usage || [])[0]

  return (
    <div className='bg-card relative overflow-hidden rounded-2xl border p-4 shadow-xs sm:p-5'>
      <div
        className='from-warning/10 pointer-events-none absolute inset-0 bg-gradient-to-br via-transparent to-transparent'
        aria-hidden='true'
      />
      <div className='relative flex flex-col gap-4'>
        {/* 头部:套餐名 + 状态 + 管理 */}
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-3'>
            <span className='bg-warning/15 text-warning flex size-10 shrink-0 items-center justify-center rounded-xl shadow-xs'>
              <Crown className='size-5' aria-hidden='true' />
            </span>
            <div className='min-w-0'>
              <div className='flex items-center gap-2'>
                <h3 className='truncate text-base font-semibold tracking-tight'>
                  {planTitle || t('Subscription Plans')}
                </h3>
                <span className='text-success bg-success/10 rounded-md px-2 py-0.5 text-xs font-medium'>
                  {t('Active')}
                </span>
              </div>
              <p className='text-muted-foreground mt-0.5 text-xs'>
                {t('Until')}{' '}
                {new Date((sub?.end_time || 0) * 1000).toLocaleDateString()}
              </p>
            </div>
          </div>
          <Button variant='outline' size='sm' render={<Link to='/wallet' />}>
            {t('Manage')}
            <ArrowRight data-icon='inline-end' />
          </Button>
        </div>

        {/* 统计区:剩余天数 / 周限额 / 5h 窗口 */}
        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='bg-background/60 rounded-xl border p-3'>
            <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
              <CalendarDays className='size-3.5' aria-hidden='true' />
              {t('Days Remaining')}
            </div>
            <div className='mt-1.5 text-2xl font-bold tracking-tight tabular-nums'>
              {remainDays}
              <span className='text-muted-foreground ml-1 text-xs font-normal'>
                {t('days')}
              </span>
            </div>
          </div>

          <div className='bg-background/60 rounded-xl border p-3'>
            <div className='text-muted-foreground flex items-center justify-between gap-2 text-xs'>
              <span className='flex items-center gap-1.5'>
                <Gauge className='size-3.5' aria-hidden='true' />
                {t('Weekly Quota')}
              </span>
              <span className='tabular-nums'>{usagePct}%</span>
            </div>
            <div className='mt-1.5 text-sm font-semibold tabular-nums'>
              {formatQuota(usedAmount)}
              <span className='text-muted-foreground font-normal'>
                {' '}
                / {totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')}
              </span>
            </div>
            <Progress value={usagePct} className='mt-2 h-1.5' />
            {nextResetTime > 0 && (
              <div className='text-muted-foreground mt-1.5 text-[11px]'>
                {t('Next reset')}: {formatDateTime(nextResetTime)}
              </div>
            )}
          </div>

          {fiveHour && (
            <SubQuotaStat usage={fiveHour} />
          )}
        </div>
      </div>
    </div>
  )
}

function SubQuotaStat({ usage }: { usage: SubQuotaUsage }) {
  const { t } = useTranslation()
  const pct = Math.min(100, Math.round(usage.percent || 0))
  return (
    <div className='bg-background/60 rounded-xl border p-3'>
      <div className='text-muted-foreground flex items-center justify-between gap-2 text-xs'>
        <span className='flex items-center gap-1.5 truncate'>
          <TimerReset className='size-3.5 shrink-0' aria-hidden='true' />
          <span className='truncate'>{usage.name || t('Sub Limit')}</span>
        </span>
        <span
          className={cn(
            'tabular-nums',
            usage.exceeded && 'text-destructive font-medium'
          )}
        >
          {pct}%
        </span>
      </div>
      <div className='mt-1.5 text-sm font-semibold tabular-nums'>
        ${formatWindowUsd(usage.used_usd || 0)}
        <span className='text-muted-foreground font-normal'>
          {' '}
          / ${formatWindowUsd(usage.limit_usd || 0)}
        </span>
      </div>
      <Progress value={pct} className='mt-2 h-1.5' />
      {usage.reset_time > 0 && (
        <div className='text-muted-foreground mt-1.5 text-[11px]'>
          {t('Next reset')}: {formatDateTime(usage.reset_time)}
        </div>
      )}
    </div>
  )
}
