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

import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatCurrencyUSD, formatTimeStr, formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { OperationsWindow } from '../types'

// 与 features/users/components/user-quota-cell.tsx 的 getQuotaProgressColor 同一规则,
// 输入是"剩余百分比":<=10 红,<=30 黄,否则绿。
function getQuotaProgressColor(remainingPercent: number): string {
  if (remainingPercent <= 10) return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  if (remainingPercent <= 30) return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}

export function UsageProgress(props: { window: OperationsWindow | null }) {
  const { t } = useTranslation()
  const w = props.window

  if (!w || w.limit_usd <= 0) {
    return <span className='text-muted-foreground text-xs'>—</span>
  }

  const usedPercent = Math.min(Math.max(w.percent || 0, 0), 100)
  const remainingPercent = 100 - usedPercent

  return (
    <Tooltip>
      <TooltipTrigger
        render={<div className='w-full min-w-[120px] cursor-help space-y-1' />}
      >
        <div className='flex items-center justify-between gap-2 text-xs tabular-nums'>
          <span className='font-medium'>{formatCurrencyUSD(w.used_usd)}</span>
          <span className='text-muted-foreground'>
            / {formatCurrencyUSD(w.limit_usd)}
          </span>
        </div>
        <Progress
          value={usedPercent}
          className={cn('h-1.5', getQuotaProgressColor(remainingPercent))}
        />
        {w.window_end > 0 && (
          <div className='text-muted-foreground text-[10px] tabular-nums'>
            {formatTimeStr(new Date(w.window_end * 1000))} {t('Reset')}
          </div>
        )}
      </TooltipTrigger>
      <TooltipContent side='top'>
        <div className='space-y-0.5 text-xs'>
          <div>
            {t('Used:')} {formatCurrencyUSD(w.used_usd)} (
            {usedPercent.toFixed(1)}%)
          </div>
          <div>
            {t('Remaining:')}{' '}
            {formatCurrencyUSD(Math.max(0, w.limit_usd - w.used_usd))}
          </div>
          {w.window_end > 0 && (
            <div>
              {t('Reset:')} {formatTimestamp(w.window_end)}
            </div>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
