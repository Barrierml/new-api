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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { UserActiveSubscription } from '../types'

type UserQuotaCellProps = {
  used: number
  remaining: number
  subscription?: UserActiveSubscription | null
}

function getQuotaProgressColor(percentage: number): string {
  if (percentage <= 10) return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  if (percentage <= 30) return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}

function WalletRow(props: { used: number; remaining: number }) {
  const { t } = useTranslation()
  const total = props.used + props.remaining
  const percentage = total > 0 ? (props.remaining / total) * 100 : 0
  const formattedRemaining = formatQuota(props.remaining)
  const formattedTotal = formatQuota(total)

  if (total === 0) {
    return (
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='text-muted-foreground shrink-0'>{t('Wallet')}</span>
        <StatusBadge
          label={t('No Quota')}
          variant='neutral'
          copyable={false}
          className='-ml-0'
        />
      </div>
    )
  }

  return (
    <div className='space-y-1'>
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='text-muted-foreground shrink-0'>{t('Wallet')}</span>
        <span className='min-w-0 truncate font-medium tabular-nums'>
          {formattedRemaining}
          <span className='text-muted-foreground font-normal'>
            {' '}
            / {formattedTotal}
          </span>
        </span>
      </div>
      <Progress
        value={percentage}
        className={cn('h-1.5', getQuotaProgressColor(percentage))}
      />
    </div>
  )
}

function PlanRow(props: { subscription: UserActiveSubscription }) {
  const { t } = useTranslation()
  const sub = props.subscription
  const total = Number(sub.amount_total || 0)
  const used = Number(sub.amount_used || 0)
  const remaining = Math.max(0, Number(sub.amount_remain ?? total - used))
  const percentage = total > 0 ? (remaining / total) * 100 : 0
  const title = sub.plan_title || t('Subscription')

  return (
    <div className='space-y-1'>
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='text-muted-foreground shrink-0'>{t('Plan')}</span>
        <span className='min-w-0 truncate font-medium'>
          {title}
          <span className='text-muted-foreground font-normal tabular-nums'>
            {' '}
            · {formatQuota(remaining)} / {formatQuota(total)}
          </span>
        </span>
      </div>
      {total > 0 && (
        <Progress
          value={percentage}
          className={cn('h-1.5', getQuotaProgressColor(percentage))}
        />
      )}
    </div>
  )
}

export function UserQuotaCell(props: UserQuotaCellProps) {
  const { t } = useTranslation()
  const walletTotal = props.used + props.remaining
  const hasWallet = walletTotal > 0
  const hasPlan = !!props.subscription

  if (!hasWallet && !hasPlan) {
    return (
      <StatusBadge
        label={t('No Quota')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div className='w-full min-w-0 cursor-help space-y-2 overflow-hidden' />
        }
      >
        <WalletRow used={props.used} remaining={props.remaining} />
        {hasPlan && props.subscription ? (
          <PlanRow subscription={props.subscription} />
        ) : (
          <div className='flex items-center justify-between gap-2 text-xs'>
            <span className='text-muted-foreground shrink-0'>{t('Plan')}</span>
            <span className='text-muted-foreground'>{t('No Plan')}</span>
          </div>
        )}
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-2 text-xs'>
          <div className='font-medium'>{t('Wallet')}</div>
          <div>
            {t('Used:')} {formatQuota(props.used)}
          </div>
          <div>
            {t('Remaining:')} {formatQuota(props.remaining)}
          </div>
          <div>
            {t('Total:')} {formatQuota(walletTotal)}
          </div>
          {hasPlan && props.subscription && (
            <>
              <div className='border-border mt-1 border-t pt-1 font-medium'>
                {t('Plan')}: {props.subscription.plan_title || t('Subscription')}
              </div>
              <div>
                {t('Used:')} {formatQuota(props.subscription.amount_used)}
              </div>
              <div>
                {t('Remaining:')} {formatQuota(props.subscription.amount_remain)}
              </div>
              <div>
                {t('Total:')} {formatQuota(props.subscription.amount_total)}
              </div>
            </>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
