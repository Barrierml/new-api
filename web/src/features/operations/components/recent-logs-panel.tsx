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
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PanelWrapper } from '@/features/dashboard/components/ui/panel-wrapper'
import {
  formatCurrencyUSD,
  formatTimestampRelative,
  quotaUnitsToDollars,
} from '@/lib/format'

import type { OperationsLogItem } from '../types'

interface RecentLogsPanelProps {
  kind: 'error' | 'consume'
  logs: OperationsLogItem[]
  loading: boolean
}

const LOG_TYPE_FILTER = { error: '5', consume: '2' } as const

export function RecentLogsPanel(props: RecentLogsPanelProps) {
  const { t } = useTranslation()
  const isError = props.kind === 'error'

  return (
    <PanelWrapper
      title={isError ? t('Recent Error Logs') : t('Recent Consumption Logs')}
      loading={props.loading}
      empty={!props.loading && props.logs.length === 0}
      emptyMessage={t('No logs yet')}
      height='h-48'
      contentClassName='p-0 sm:p-0'
      headerActions={
        <Link
          to='/usage-logs/$section'
          params={{ section: 'common' }}
          search={{ type: [LOG_TYPE_FILTER[props.kind]] }}
          className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-xs'
        >
          {t('View All')}
          <ArrowRight className='size-3' />
        </Link>
      }
    >
      <ul className='divide-border divide-y'>
        {props.logs.map((log) => {
          const userLabel = log.user_display_name || log.username
          return (
            <li key={log.id} className='flex items-center gap-3 px-4 py-2.5'>
              <span
                className='text-muted-foreground w-16 shrink-0 text-xs tabular-nums'
                title={String(log.created_at)}
              >
                {formatTimestampRelative(log.created_at)}
              </span>
              <span className='w-24 shrink-0 truncate text-sm font-medium'>
                {userLabel}
              </span>
              {isError ? (
                <span
                  className='text-destructive min-w-0 flex-1 truncate text-xs'
                  title={log.content}
                >
                  {log.content || log.model_name}
                </span>
              ) : (
                <>
                  <span className='min-w-0 flex-1 truncate text-xs'>
                    {log.model_name}
                    {log.channel_name && (
                      <span className='text-muted-foreground'>
                        {' '}
                        · {log.channel_name}
                      </span>
                    )}
                  </span>
                  <span className='shrink-0 text-xs tabular-nums'>
                    {formatCurrencyUSD(quotaUnitsToDollars(log.quota))}
                  </span>
                </>
              )}
            </li>
          )
        })}
      </ul>
    </PanelWrapper>
  )
}
