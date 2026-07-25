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
import { Activity, Coins, Users, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import {
  formatCurrencyUSD,
  formatNumber,
  formatTokens,
  quotaUnitsToDollars,
} from '@/lib/format'

import type { OperationsCards } from '../types'

interface OperationsStatCardsProps {
  cards?: OperationsCards
  loading: boolean
  error: boolean
}

export function OperationsStatCards(props: OperationsStatCardsProps) {
  const { t } = useTranslation()
  const { cards, loading, error } = props

  const totalTokens =
    (cards?.today_prompt_tokens ?? 0) + (cards?.today_completion_tokens ?? 0)

  const items = [
    <StatCard
      key='cost'
      title={t("Today's Consumption")}
      value={formatCurrencyUSD(quotaUnitsToDollars(cards?.today_quota ?? 0))}
      description={t('All users, since midnight (UTC+8)')}
      icon={Coins}
      tone='accent-1'
      loading={loading}
      error={error}
    />,
    <StatCard
      key='tokens'
      title={t("Today's Tokens")}
      value={formatTokens(totalTokens)}
      description={t('Cached tokens not included')}
      icon={Zap}
      tone='accent-2'
      loading={loading}
      error={error}
      details={[
        {
          label: t('Input'),
          value: formatTokens(cards?.today_prompt_tokens ?? 0),
        },
        {
          label: t('Output'),
          value: formatTokens(cards?.today_completion_tokens ?? 0),
        },
      ]}
    />,
    <StatCard
      key='users'
      title={t('Active Users')}
      value={formatNumber(cards?.active_users_today ?? 0)}
      description={t('Users with consumption today')}
      icon={Users}
      tone='accent-3'
      loading={loading}
      error={error}
      details={[
        {
          label: t('Today'),
          value: formatNumber(cards?.active_users_today ?? 0),
        },
        {
          label: t('Total'),
          value: formatNumber(cards?.total_users ?? 0),
          tone: 'muted',
        },
      ]}
    />,
    <StatCard
      key='concurrency'
      title={t('Concurrent Requests')}
      value={formatNumber(cards?.active_connections ?? 0)}
      description={t('In-flight relay requests on this node')}
      icon={Activity}
      tone='accent-1'
      loading={loading}
      error={error}
    />,
  ]

  return (
    <div className='bg-card overflow-hidden rounded-2xl border shadow-xs'>
      <div className='grid grid-cols-2 gap-1.5 p-2 sm:gap-3 sm:p-3 lg:grid-cols-4'>
        {items.map((card) => (
          <div
            key={card.key}
            className='bg-background/60 rounded-lg border px-2 py-1.5 sm:rounded-xl sm:p-3'
          >
            {card}
          </div>
        ))}
      </div>
    </div>
  )
}
