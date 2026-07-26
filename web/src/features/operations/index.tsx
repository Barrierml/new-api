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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { UpstreamQuotaPanel } from '@/features/system-info/components/upstream-quota-panel'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { getOperationsOverview } from './api'
import { ActiveUsersTable } from './components/active-users-table'
import { OperationsStatCards } from './components/operations-stat-cards'
import { RecentLogsPanel } from './components/recent-logs-panel'

export function Operations() {
  const { t } = useTranslation()
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )

  const overviewQuery = useQuery({
    queryKey: ['operations-overview'],
    queryFn: async () => {
      const result = await getOperationsOverview()
      if (!result.success || !result.data) {
        throw new Error(result.message || 'failed to load operations overview')
      }
      return result.data
    },
    refetchInterval: 30_000,
    placeholderData: keepPreviousData,
  })

  const data = overviewQuery.data

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Operations')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void overviewQuery.refetch()}
          disabled={overviewQuery.isFetching}
        >
          <RefreshCw
            className={cn(
              'size-3.5',
              overviewQuery.isFetching && 'animate-spin'
            )}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          {isRoot && <UpstreamQuotaPanel />}
          <OperationsStatCards
            cards={data?.cards}
            loading={overviewQuery.isLoading}
            error={overviewQuery.isError}
          />
          <ActiveUsersTable
            users={data?.users ?? []}
            loading={overviewQuery.isLoading}
          />
          <div className='grid gap-4 lg:grid-cols-2'>
            <RecentLogsPanel
              kind='error'
              logs={data?.error_logs ?? []}
              loading={overviewQuery.isLoading}
            />
            <RecentLogsPanel
              kind='consume'
              logs={data?.consume_logs ?? []}
              loading={overviewQuery.isLoading}
            />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
