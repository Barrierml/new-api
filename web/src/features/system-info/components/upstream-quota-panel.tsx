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
import { Gauge, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { getUpstreamQuotaDashboard } from '@/features/system-info/api'
import type {
  UpstreamQuotaEntity,
  UpstreamQuotaStatus,
  UpstreamQuotaWindow,
} from '@/features/system-info/types'
import { cn } from '@/lib/utils'

const POLL_INTERVAL_MS = 30_000
const SKELETON_KEYS = [
  'quota-skeleton-1',
  'quota-skeleton-2',
  'quota-skeleton-3',
  'quota-skeleton-4',
]

const STATUS_CLASS_NAME: Record<UpstreamQuotaStatus | 'stale', string> = {
  available:
    'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
  limited:
    'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
  exhausted: 'bg-destructive/10 text-destructive',
  unknown: 'bg-muted text-muted-foreground',
  error: 'bg-destructive/10 text-destructive',
  deferred: 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300',
  stale: 'bg-muted text-muted-foreground',
}

function formatNumber(value: number | undefined) {
  if (value === undefined) return null
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(
    value
  )
}

function formatTime(value: string | number | undefined) {
  if (value === undefined || value === '') return null
  const date = new Date(typeof value === 'number' ? value * 1000 : value)
  if (Number.isNaN(date.getTime())) return null
  return date.toLocaleString()
}

function IdList(props: { label: string; values?: number[] }) {
  const { t } = useTranslation()
  return (
    <div className='flex min-w-0 items-center gap-1.5 text-xs'>
      <span className='text-muted-foreground shrink-0'>{props.label}</span>
      <span className='truncate font-mono'>
        {props.values?.length ? props.values.join(', ') : t('Not mapped')}
      </span>
    </div>
  )
}

function QuotaWindowRow(props: { window: UpstreamQuotaWindow }) {
  const { t } = useTranslation()
  const remaining = formatNumber(props.window.remaining)
  const percentage = formatNumber(props.window.remaining_pct)
  const resetAt = formatTime(props.window.reset_at)
  const hasPercentage = props.window.remaining_pct !== undefined
  const progress = hasPercentage
    ? Math.min(100, Math.max(0, props.window.remaining_pct ?? 0))
    : null

  return (
    <div className='space-y-2 rounded-md border p-3'>
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <div className='min-w-0'>
          <div className='font-medium'>
            {props.window.label ||
              props.window.key ||
              t('Unnamed quota window')}
          </div>
          {props.window.key && (
            <div className='text-muted-foreground font-mono text-[11px]'>
              {props.window.key}
            </div>
          )}
        </div>
        <div className='text-right text-sm tabular-nums'>
          {remaining === null && percentage === null ? (
            <span className='text-muted-foreground'>{t('Unavailable')}</span>
          ) : (
            <>
              {remaining !== null && (
                <span>
                  {remaining}
                  {props.window.unit ? ` ${props.window.unit}` : ''}
                </span>
              )}
              {remaining !== null && percentage !== null && ' · '}
              {percentage !== null && <span>{percentage}%</span>}
            </>
          )}
        </div>
      </div>
      {progress !== null && <Progress value={progress} className='h-1.5' />}
      <div className='text-muted-foreground text-xs'>
        {resetAt
          ? t('Resets at {{time}}', { time: resetAt })
          : t('Reset time unavailable')}
      </div>
    </div>
  )
}

function QuotaEntityCard(props: { entity: UpstreamQuotaEntity }) {
  const { t } = useTranslation()
  const effectiveStatus = props.entity.stale ? 'stale' : props.entity.status
  const fetchedAt = formatTime(props.entity.fetched_at)

  return (
    <article className='bg-card overflow-hidden rounded-lg border shadow-xs'>
      <div className='space-y-3 border-b p-4'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='min-w-0'>
            <h4 className='truncate font-semibold'>
              {props.entity.display_name || props.entity.entity_id}
            </h4>
            <div className='text-muted-foreground mt-0.5 flex flex-wrap gap-x-2 text-xs'>
              <span>{props.entity.provider || t('Unknown provider')}</span>
              <span className='font-mono'>{props.entity.entity_id}</span>
            </div>
          </div>
          <Badge
            variant='secondary'
            className={STATUS_CLASS_NAME[effectiveStatus]}
          >
            {t(effectiveStatus)}
          </Badge>
        </div>
        <p className='text-muted-foreground text-xs'>
          {props.entity.stale
            ? t('Quota snapshot is stale')
            : t(props.entity.status_message)}
        </p>
        <div className='grid gap-1.5 sm:grid-cols-3'>
          <IdList label={t('Account IDs')} values={props.entity.account_ids} />
          <IdList label={t('Group IDs')} values={props.entity.group_ids} />
          <IdList label={t('Channel IDs')} values={props.entity.channel_ids} />
        </div>
        <div className='text-muted-foreground text-xs'>
          {fetchedAt
            ? t('Collected at {{time}}', { time: fetchedAt })
            : t('Collection time unavailable')}
        </div>
      </div>
      <div className='space-y-2 p-4'>
        {props.entity.windows?.length ? (
          props.entity.windows.map((window) => (
            <QuotaWindowRow
              key={`${window.key || window.label || 'window'}-${window.kind || ''}-${window.reset_at ?? ''}`}
              window={window}
            />
          ))
        ) : (
          <div className='text-muted-foreground rounded-md border border-dashed px-3 py-5 text-center text-sm'>
            {t('No quota windows reported.')}
          </div>
        )}
      </div>
    </article>
  )
}

export function UpstreamQuotaPanel() {
  const { t } = useTranslation()
  const quotaQuery = useQuery({
    queryKey: ['system-info', 'upstream-quota'],
    queryFn: async () => {
      const res = await getUpstreamQuotaDashboard()
      if (!res.success || !res.data) {
        throw new Error(res.message || t('We could not load upstream quota.'))
      }
      return res.data
    },
    refetchInterval: POLL_INTERVAL_MS,
    retry: false,
  })

  const dashboard = quotaQuery.data
  const refreshing = quotaQuery.isFetching && !quotaQuery.isLoading
  const generatedAt = formatTime(dashboard?.generated_at)
  const statuses: Array<UpstreamQuotaStatus | 'stale'> = [
    'available',
    'limited',
    'exhausted',
    'unknown',
    'error',
    'deferred',
    'stale',
  ]

  let dashboardContent
  if (quotaQuery.isLoading) {
    dashboardContent = (
      <div className='grid gap-4 lg:grid-cols-2'>
        {SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-72 rounded-lg' />
        ))}
      </div>
    )
  } else if (quotaQuery.isError) {
    dashboardContent = (
      <ErrorState
        title={t('We could not load upstream quota.')}
        description={
          quotaQuery.error instanceof Error
            ? quotaQuery.error.message
            : undefined
        }
        onRetry={() => void quotaQuery.refetch()}
        className='min-h-[260px]'
      />
    )
  } else if (!dashboard?.entities.length) {
    dashboardContent = (
      <div className='text-muted-foreground rounded-lg border border-dashed py-12 text-center text-sm'>
        {t('No upstream quota entities reported.')}
      </div>
    )
  } else {
    dashboardContent = (
      <div className='grid gap-4 lg:grid-cols-2'>
        {dashboard.entities.map((entity) => (
          <QuotaEntityCard key={entity.entity_id} entity={entity} />
        ))}
      </div>
    )
  }

  return (
    <section className='space-y-4'>
      <div className='bg-card rounded-lg border px-4 py-3 shadow-xs sm:px-5'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex items-center gap-2'>
            <span className='bg-muted text-muted-foreground inline-flex size-7 items-center justify-center rounded-md'>
              <Gauge className='size-4' aria-hidden='true' />
            </span>
            <div>
              <h3 className='text-sm font-semibold'>{t('Upstream Quota')}</h3>
              <p className='text-muted-foreground mt-0.5 text-xs'>
                {generatedAt
                  ? t('Snapshot generated at {{time}}', { time: generatedAt })
                  : t('Read-only remaining quota and freshness by account.')}
              </p>
            </div>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void quotaQuery.refetch()}
            disabled={quotaQuery.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn('size-3.5', refreshing && 'animate-spin')}
              aria-hidden='true'
            />
            {refreshing ? t('Refreshing...') : t('Refresh')}
          </Button>
        </div>
        {dashboard && (
          <div className='mt-3 flex flex-wrap gap-2'>
            {statuses.map((status) => (
              <Badge
                key={status}
                variant='secondary'
                className={STATUS_CLASS_NAME[status]}
              >
                {t(status)}: {dashboard.counts[status] ?? 0}
              </Badge>
            ))}
          </div>
        )}
      </div>

      {dashboardContent}
    </section>
  )
}
