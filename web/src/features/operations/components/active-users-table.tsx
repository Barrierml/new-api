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

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { PanelWrapper } from '@/features/dashboard/components/ui/panel-wrapper'
import {
  formatCurrencyUSD,
  formatQuota,
  formatTimestampRelative,
  formatTokens,
  quotaUnitsToDollars,
} from '@/lib/format'

import type { OperationsActiveUser } from '../types'
import { UsageProgress } from './usage-progress'

interface ActiveUsersTableProps {
  users: OperationsActiveUser[]
  loading: boolean
}

export function ActiveUsersTable(props: ActiveUsersTableProps) {
  const { t } = useTranslation()

  const columns: StaticDataTableColumn<OperationsActiveUser>[] = [
    {
      id: 'user',
      header: t('User'),
      cell: (row) => (
        <div className='flex flex-col'>
          <span className='truncate text-sm font-medium'>
            {row.display_name || row.username}
          </span>
          {row.display_name && row.display_name !== row.username && (
            <span className='text-muted-foreground truncate text-xs'>
              {row.username}
            </span>
          )}
        </div>
      ),
    },
    {
      id: 'plan',
      header: t('Plan'),
      cell: (row) =>
        row.plan_title ? (
          <StatusBadge label={row.plan_title} variant='info' copyable={false} />
        ) : (
          <span className='text-muted-foreground text-xs'>{t('No Plan')}</span>
        ),
    },
    {
      id: 'today_quota',
      header: t("Today's Consumption"),
      cellClassName: 'tabular-nums',
      cell: (row) => formatCurrencyUSD(quotaUnitsToDollars(row.today_quota)),
    },
    {
      id: 'today_tokens',
      header: t("Today's Tokens"),
      cell: (row) => (
        <div className='flex flex-col text-xs tabular-nums'>
          <span className='font-medium'>
            {formatTokens(row.today_prompt_tokens + row.today_completion_tokens)}
          </span>
          <span className='text-muted-foreground'>
            {formatTokens(row.today_prompt_tokens)} ·{' '}
            {formatTokens(row.today_completion_tokens)}
          </span>
        </div>
      ),
    },
    {
      id: 'window_5h',
      header: t('5h Window'),
      cell: (row) => <UsageProgress window={row.window_5h} />,
    },
    {
      id: 'weekly',
      header: t('Weekly Usage'),
      cell: (row) => <UsageProgress window={row.weekly} />,
    },
    {
      id: 'wallet',
      header: t('Wallet Balance'),
      cellClassName: 'tabular-nums',
      cell: (row) => formatQuota(row.wallet_quota),
    },
    {
      id: 'last_used',
      header: t('Last Used'),
      cell: (row) => {
        // 10 分钟内活跃的用户时间标绿,30s 轮询自动消退
        const recentlyActive =
          row.last_used_at > 0 && Date.now() / 1000 - row.last_used_at < 600
        return (
          <span
            className={
              recentlyActive
                ? 'text-xs font-medium text-emerald-600 dark:text-emerald-400'
                : 'text-muted-foreground text-xs'
            }
          >
            {formatTimestampRelative(row.last_used_at)}
          </span>
        )
      },
    },
  ]

  return (
    <PanelWrapper
      title={t('Active Users Today')}
      description={t('Top 50 users by consumption today')}
      loading={props.loading}
      empty={!props.loading && props.users.length === 0}
      emptyMessage={t('No active users today')}
      height='h-40'
      contentClassName='p-0 sm:p-0'
    >
      <StaticDataTable
        columns={columns}
        data={props.users}
        getRowKey={(row) => row.user_id}
      />
    </PanelWrapper>
  )
}
