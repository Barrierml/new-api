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
import { CircleAlert, Gauge, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { getUpstreamQuotaDashboard } from '@/features/system-info/api'
import type {
  UpstreamQuotaChannel,
  UpstreamQuotaEntity,
  UpstreamQuotaStatus,
  UpstreamQuotaWindow,
} from '@/features/system-info/types'
import { cn } from '@/lib/utils'

const POLL_INTERVAL_MS = 30_000
const SKELETON_KEYS = ['quota-skeleton-1', 'quota-skeleton-2']

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

function getProviderLabel(
  provider: string,
  t: ReturnType<typeof useTranslation>['t']
) {
  if (!provider) return t('Unknown provider')
  return t(`upstreamQuota.provider.${provider}`, { defaultValue: provider })
}

function getDisplayName(
  entity: UpstreamQuotaEntity,
  t: ReturnType<typeof useTranslation>['t']
) {
  const displayName = entity.display_name || entity.entity_id
  return t(`upstreamQuota.entity.${entity.entity_id}`, {
    defaultValue: displayName,
  })
}

function isFiveHourWindow(window: UpstreamQuotaWindow) {
  const value = `${window.key ?? ''} ${window.label ?? ''}`.toLowerCase()
  return (
    value.includes('5h') ||
    value.includes('5 h') ||
    value.includes('5 hour') ||
    window.duration_seconds === 5 * 60 * 60
  )
}

function isWeeklyWindow(window: UpstreamQuotaWindow) {
  const value = `${window.key ?? ''} ${window.label ?? ''}`.toLowerCase()
  return value.includes('week') || window.duration_seconds === 7 * 24 * 60 * 60
}

function windowRank(window: UpstreamQuotaWindow) {
  if (isWeeklyWindow(window)) return 0
  if (isFiveHourWindow(window)) return 1
  if (window.kind === 'rate_limit') return 2
  if (window.kind === 'balance') return 4
  return 3
}

function sortedWindows(windows: UpstreamQuotaWindow[] = []) {
  return windows
    .filter(
      (window) =>
        window.remaining !== undefined || window.remaining_pct !== undefined
    )
    .sort((left, right) => {
      const rank = windowRank(left) - windowRank(right)
      if (rank !== 0) return rank
      return (left.remaining_pct ?? 101) - (right.remaining_pct ?? 101)
    })
}

function hasUsefulWindows(entity: UpstreamQuotaEntity) {
  return sortedWindows(entity.windows).length > 0
}

function entityRank(entity: UpstreamQuotaEntity) {
  const windows = sortedWindows(entity.windows)
  if (windows.some(isWeeklyWindow)) return 0
  if (windows.some(isFiveHourWindow)) return 1
  if (windows.length) return 2
  return 3
}

function sortedEntities(entities: UpstreamQuotaEntity[]) {
  return [...entities].sort((left, right) => {
    const rank = entityRank(left) - entityRank(right)
    if (rank !== 0) return rank
    const remaining =
      Math.min(
        ...sortedWindows(left.windows).map(
          (window) => window.remaining_pct ?? 101
        )
      ) -
      Math.min(
        ...sortedWindows(right.windows).map(
          (window) => window.remaining_pct ?? 101
        )
      )
    if (Number.isFinite(remaining) && remaining !== 0) return remaining
    return left.entity_id.localeCompare(right.entity_id)
  })
}

function groupEntitiesByProvider(entities: UpstreamQuotaEntity[]) {
  const groups = new Map<string, UpstreamQuotaEntity[]>()
  for (const entity of sortedEntities(entities)) {
    const provider = entity.provider || ''
    groups.set(provider, [...(groups.get(provider) ?? []), entity])
  }
  return Array.from(groups, ([provider, groupedEntities]) => ({
    provider,
    entities: groupedEntities,
  })).sort((left, right) => {
    const rank = entityRank(left.entities[0]) - entityRank(right.entities[0])
    if (rank !== 0) return rank
    return left.provider.localeCompare(right.provider)
  })
}

function IdList(props: { label: string; values?: number[] }) {
  const { t } = useTranslation()
  return (
    <span className='inline-flex min-w-0 items-center gap-1 text-[11px]'>
      <span className='text-muted-foreground shrink-0'>{props.label}</span>
      <span className='truncate font-mono'>
        {props.values?.length ? props.values.join(', ') : t('Not mapped')}
      </span>
    </span>
  )
}

function ChannelPriority(props: { channel: UpstreamQuotaChannel }) {
  const { t } = useTranslation()
  return (
    <span className='bg-muted/60 inline-flex max-w-full items-center gap-1 rounded border px-1.5 py-0.5 text-[11px]'>
      <span className='min-w-0 truncate'>
        {props.channel.name || `#${props.channel.id}`}
      </span>
      <span className='text-muted-foreground shrink-0 font-mono'>
        #{props.channel.id}
      </span>
      <span className='shrink-0 font-medium tabular-nums'>
        {t('Priority {{priority}}', { priority: props.channel.priority })}
      </span>
    </span>
  )
}

function QuotaWindowRow(props: {
  window: UpstreamQuotaWindow
  prominent?: boolean
}) {
  const { t } = useTranslation()
  const remaining = formatNumber(props.window.remaining)
  const percentage = formatNumber(props.window.remaining_pct)
  const resetAt = formatTime(props.window.reset_at)
  const unit = props.window.unit
    ? t(`upstreamQuota.unit.${props.window.unit}`, {
        defaultValue: props.window.unit,
      })
    : ''
  const progress =
    props.window.remaining_pct === undefined
      ? null
      : Math.min(100, Math.max(0, props.window.remaining_pct))

  return (
    <div
      className={cn(
        'space-y-1 rounded border px-2 py-1.5',
        props.prominent && 'border-primary/20 bg-primary/[0.035]'
      )}
    >
      <div className='flex min-w-0 items-center justify-between gap-2 text-xs'>
        <span
          className={cn('truncate font-medium', props.prominent && 'text-sm')}
        >
          {props.window.label || props.window.key || t('Unnamed quota window')}
        </span>
        <span
          className={cn(
            'shrink-0 tabular-nums',
            props.prominent && 'text-sm font-semibold'
          )}
        >
          {remaining === null && percentage === null ? (
            <span className='text-muted-foreground'>{t('Unavailable')}</span>
          ) : (
            <>
              {remaining !== null && `${remaining}${unit ? ` ${unit}` : ''}`}
              {remaining !== null && percentage !== null && ' · '}
              {percentage !== null && `${percentage}%`}
            </>
          )}
        </span>
      </div>
      <div className='flex items-center gap-2'>
        {progress !== null && (
          <Progress value={progress} className='h-1 min-w-16 flex-1' />
        )}
        <span className='text-muted-foreground shrink-0 text-[11px]'>
          {resetAt
            ? t('Resets at {{time}}', { time: resetAt })
            : t('Reset time unavailable')}
        </span>
      </div>
    </div>
  )
}

function QuotaEntityCard(props: { entity: UpstreamQuotaEntity }) {
  const { t } = useTranslation()
  const effectiveStatus = props.entity.stale ? 'stale' : props.entity.status
  const fetchedAt = formatTime(props.entity.fetched_at)
  const windows = sortedWindows(props.entity.windows)

  return (
    <article className='bg-card rounded-md border p-2 shadow-xs'>
      <div className='flex min-w-0 items-start justify-between gap-2'>
        <div className='min-w-0'>
          <div className='flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5'>
            <h4 className='truncate text-sm font-semibold'>
              {getDisplayName(props.entity, t)}
            </h4>
            <span className='text-muted-foreground font-mono text-[11px]'>
              {props.entity.entity_id}
            </span>
          </div>
          <p className='text-muted-foreground mt-0.5 text-[11px] leading-4'>
            {props.entity.stale
              ? t('Quota snapshot is stale')
              : t(props.entity.status_message)}
            {' · '}
            {fetchedAt
              ? t('Collected at {{time}}', { time: fetchedAt })
              : t('Collection time unavailable')}
          </p>
        </div>
        <Badge
          variant='secondary'
          className={cn(
            'h-5 shrink-0 px-1.5 text-[11px]',
            STATUS_CLASS_NAME[effectiveStatus]
          )}
        >
          {t(`upstreamQuota.status.${effectiveStatus}`, {
            defaultValue: effectiveStatus,
          })}
        </Badge>
      </div>

      <div className='mt-1.5 flex flex-wrap gap-x-3 gap-y-1'>
        <IdList label={t('Account IDs')} values={props.entity.account_ids} />
        <IdList label={t('Group IDs')} values={props.entity.group_ids} />
        <IdList label={t('Channel IDs')} values={props.entity.channel_ids} />
      </div>

      {!!props.entity.channels?.length && (
        <div className='mt-1.5 flex flex-wrap gap-1'>
          {props.entity.channels.map((channel) => (
            <ChannelPriority key={channel.id} channel={channel} />
          ))}
        </div>
      )}

      <div className='mt-1.5 grid gap-1.5 sm:grid-cols-2'>
        {windows.map((window) => (
          <QuotaWindowRow
            key={`${window.key || window.label || 'window'}-${window.kind || ''}-${window.reset_at ?? ''}`}
            window={window}
            prominent={isWeeklyWindow(window) || isFiveHourWindow(window)}
          />
        ))}
      </div>
    </article>
  )
}

function UnavailableEntityPill(props: { entity: UpstreamQuotaEntity }) {
  const { t } = useTranslation()
  const effectiveStatus = props.entity.stale ? 'stale' : props.entity.status
  return (
    <span className='bg-card inline-flex max-w-full items-center gap-1.5 rounded-full border px-2 py-1 text-[11px]'>
      <CircleAlert className='text-muted-foreground size-3 shrink-0' />
      <span className='truncate font-medium'>
        {getDisplayName(props.entity, t)}
      </span>
      <Badge
        variant='secondary'
        className={cn(
          'h-4 shrink-0 rounded-full px-1.5 text-[9px]',
          STATUS_CLASS_NAME[effectiveStatus]
        )}
      >
        {t(`upstreamQuota.status.${effectiveStatus}`, {
          defaultValue: effectiveStatus,
        })}
      </Badge>
    </span>
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
  const usefulEntities = dashboard?.entities.filter(hasUsefulWindows) ?? []
  const unavailableEntities =
    dashboard?.entities.filter((entity) => !hasUsefulWindows(entity)) ?? []
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
      <div className='grid gap-2 lg:grid-cols-2'>
        {SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-44 rounded-md' />
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
        className='min-h-[180px]'
      />
    )
  } else if (!dashboard?.entities.length) {
    dashboardContent = (
      <div className='text-muted-foreground rounded-md border border-dashed py-7 text-center text-sm'>
        {t('No upstream quota entities reported.')}
      </div>
    )
  } else {
    dashboardContent = (
      <div className='space-y-2'>
        {groupEntitiesByProvider(usefulEntities).map((group) => (
          <section
            key={group.provider || 'unknown'}
            className='bg-muted/20 rounded-md border p-1.5'
          >
            <div className='mb-1 flex items-center justify-between gap-2 px-0.5'>
              <h4 className='text-xs font-semibold'>
                {getProviderLabel(group.provider, t)}
              </h4>
              <span className='text-muted-foreground text-[10px]'>
                {t('{{count}} accounts', { count: group.entities.length })}
              </span>
            </div>
            <div className='grid gap-1.5 xl:grid-cols-2'>
              {group.entities.map((entity) => (
                <QuotaEntityCard key={entity.entity_id} entity={entity} />
              ))}
            </div>
          </section>
        ))}
        {!!unavailableEntities.length && (
          <section className='bg-muted/20 rounded-md border border-dashed px-2 py-1.5'>
            <div className='flex flex-wrap items-center gap-1.5'>
              <span className='text-muted-foreground mr-0.5 text-[11px] font-medium'>
                {t('Unavailable')}
              </span>
              {sortedEntities(unavailableEntities).map((entity) => (
                <UnavailableEntityPill key={entity.entity_id} entity={entity} />
              ))}
            </div>
          </section>
        )}
      </div>
    )
  }

  return (
    <section className='space-y-2'>
      <div className='bg-card rounded-md border px-3 py-2.5 shadow-xs'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='bg-muted text-muted-foreground inline-flex size-6 shrink-0 items-center justify-center rounded'>
              <Gauge className='size-3.5' aria-hidden='true' />
            </span>
            <div className='min-w-0'>
              <h3 className='text-sm font-semibold'>{t('Upstream Quota')}</h3>
              <p className='text-muted-foreground truncate text-[11px]'>
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
            className='h-7 px-2 text-xs'
            onClick={() => void quotaQuery.refetch()}
            disabled={quotaQuery.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn('size-3', refreshing && 'animate-spin')}
              aria-hidden='true'
            />
            {refreshing ? t('Refreshing...') : t('Refresh')}
          </Button>
        </div>
        {dashboard && (
          <div className='mt-2 flex flex-wrap items-center gap-1'>
            {statuses.map((status) => (
              <Badge
                key={status}
                variant='secondary'
                className={cn(
                  'h-5 px-1.5 text-[10px]',
                  STATUS_CLASS_NAME[status]
                )}
              >
                {t(`upstreamQuota.status.${status}`, {
                  defaultValue: status,
                })}
                : {dashboard.counts[status] ?? 0}
              </Badge>
            ))}
            <span className='text-muted-foreground ml-auto text-[10px]'>
              {t('Higher priority routes first')}
            </span>
          </div>
        )}
      </div>

      {dashboardContent}
    </section>
  )
}
