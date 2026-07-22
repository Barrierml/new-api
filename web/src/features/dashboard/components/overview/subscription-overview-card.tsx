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
import { ArrowRight, Crown } from 'lucide-react'
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

function SubQuotaBar({ usage }: { usage: SubQuotaUsage }) {
  const { t } = useTranslation()
  const pct = Math.min(100, Math.round(usage.percent || 0))
  return (
    <div className='min-w-0'>
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='text-muted-foreground truncate'>
          {usage.name || t('Sub Limit')}
        </span>
        <span className='shrink-0 font-medium'>
          ${(usage.used_usd || 0).toFixed(2)} / $
          {(usage.limit_usd || 0).toFixed(2)}
        </span>
      </div>
      <Progress value={pct} className='mt-1.5 h-1.5' />
    </div>
  )
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
        <Skeleton className='mt-3 h-16 w-full' />
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
      <div className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-3'>
            <span className='bg-background/70 flex size-9 shrink-0 items-center justify-center rounded-xl border shadow-xs'>
              <Crown className='text-warning size-4' aria-hidden='true' />
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
    totalAmount > 0 ? Math.min(100, Math.round((usedAmount / totalAmount) * 100)) : 0
  const remainDays = Math.max(
    0,
    Math.ceil(((sub?.end_time || 0) - Date.now() / 1000) / 86400)
  )
  const nextResetTime = sub?.next_reset_time ?? 0
  const subUsages = (active.sub_quota_usage || []).slice(0, 2)

  return (
    <div className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='flex min-w-0 items-center gap-3'>
          <span className='bg-background/70 flex size-9 shrink-0 items-center justify-center rounded-xl border shadow-xs'>
            <Crown className='text-warning size-4' aria-hidden='true' />
          </span>
          <div className='min-w-0'>
            <div className='flex items-center gap-2'>
              <h3 className='truncate text-sm font-semibold'>
                {planTitle || t('Subscription Plans')}
              </h3>
              <span className='text-success bg-success/10 rounded-md px-2 py-0.5 text-xs font-medium'>
                {t('Active')}
              </span>
            </div>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t('{{count}} days remaining', { count: remainDays })}
              {nextResetTime > 0 &&
                ` · ${t('Next reset')}: ${new Date(nextResetTime * 1000).toLocaleString()}`}
            </p>
          </div>
        </div>
        <Button
          variant='outline'
          size='sm'
          render={<Link to='/wallet' />}
        >
          {t('Manage')}
          <ArrowRight data-icon='inline-end' />
        </Button>
      </div>

      <div className='mt-4 grid gap-4 sm:grid-cols-2'>
        {totalAmount > 0 && (
          <div className='min-w-0'>
            <div className='flex items-center justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>
                {t('Weekly Quota')}
              </span>
              <span className='shrink-0 font-medium'>
                {formatQuota(usedAmount)} / {formatQuota(totalAmount)} (
                {usagePct}%)
              </span>
            </div>
            <Progress value={usagePct} className='mt-1.5 h-1.5' />
          </div>
        )}
        {subUsages.map((u) => (
          <SubQuotaBar
            key={`${u.name || 'sub'}-${u.window_start || 0}`}
            usage={u}
          />
        ))}
      </div>
    </div>
  )
}
