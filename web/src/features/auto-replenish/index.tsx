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
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, CheckCircle2, Loader2, PackagePlus, RefreshCw, Zap } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import {
  getReplenishEvents,
  getReplenishOverview,
  runReplenish,
  updateReplenishConfig,
} from './api'
import type { ReplenishEvent, ReplenishOverview } from './types'

export function AutoReplenish() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const overviewQuery = useQuery({
    queryKey: ['catfk-replenish-overview'],
    queryFn: async () => {
      const r = await getReplenishOverview()
      if (!r.success || !r.data) throw new Error(r.message || 'failed')
      return r.data
    },
    refetchInterval: 30_000,
    placeholderData: keepPreviousData,
  })

  const eventsQuery = useQuery({
    queryKey: ['catfk-replenish-events'],
    queryFn: async () => {
      const r = await getReplenishEvents(1, 20)
      if (!r.success || !r.data) throw new Error(r.message || 'failed')
      return r.data
    },
    refetchInterval: 60_000,
    placeholderData: keepPreviousData,
  })

  const runMutation = useMutation({
    mutationFn: (goodsKey: string) => runReplenish(goodsKey),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['catfk-replenish-overview'] })
      void queryClient.invalidateQueries({ queryKey: ['catfk-replenish-events'] })
    },
  })

  const data = overviewQuery.data

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Auto Replenish')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => {
            void overviewQuery.refetch()
            void eventsQuery.refetch()
          }}
          disabled={overviewQuery.isFetching}
        >
          <RefreshCw className={cn('size-3.5', overviewQuery.isFetching && 'animate-spin')} />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          {overviewQuery.isLoading && (
            <div className='flex items-center gap-2 text-muted-foreground'>
              <Loader2 className='size-4 animate-spin' /> {t('Loading...')}
            </div>
          )}
          {overviewQuery.isError && (
            <div className='text-destructive'>{t('Failed to load overview')}</div>
          )}
          {data && (
            <>
              <CatfkErrorBanner error={data.catfk_error} />
              <OrphanBanner orphanCount={data.orphan_count} />
              <StatCards data={data} />
              <ConfigCard data={data} onRun={(gk) => runMutation.mutate(gk)} runPending={runMutation.isPending} />
              <StockPanel data={data} onRun={(gk) => runMutation.mutate(gk)} runPending={runMutation.isPending} />
              <EventsTable events={eventsQuery.data?.items ?? []} loading={eventsQuery.isLoading} />
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function StatCards({ data }: { data: ReplenishOverview }) {
  const { t } = useTranslation()
  const totalAvail = data.stock.reduce((s, r) => s + (r.available > 0 ? r.available : 0), 0)
  const cards = [
    {
      label: t('Daemon'),
      value: data.status.enabled ? t('Running') : t('Stopped'),
      icon: data.status.enabled ? <CheckCircle2 className='size-4 text-emerald-500' /> : <AlertTriangle className='size-4 text-amber-500' />,
      hint: data.status.is_master ? t('master node') : t('slave node (idle)'),
    },
    { label: t('Available Stock'), value: String(totalAvail), icon: <PackagePlus className='size-4' />, hint: `${data.stock.length} ${t('goods')}` },
    { label: t('Orphan Codes'), value: data.orphan_count < 0 ? '-' : String(data.orphan_count), icon: <AlertTriangle className={cn('size-4', data.orphan_count > 0 && 'text-destructive')} />, hint: t('on shelf but not in prod') },
    { label: t('Last Run'), value: data.status.last_run_at ? new Date(data.status.last_run_at * 1000).toLocaleTimeString() : '-', icon: <RefreshCw className='size-4' />, hint: data.status.interval_sec ? `${t('every')} ${Math.round(data.status.interval_sec / 60)} ${t('min')}` : '' },
  ]
  return (
    <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
      {cards.map((c) => (
        <div key={c.label} className='rounded-lg border bg-card p-4'>
          <div className='flex items-center justify-between text-muted-foreground'>
            <span className='text-xs'>{c.label}</span>
            {c.icon}
          </div>
          <div className='mt-1 text-2xl font-semibold'>{c.value}</div>
          {c.hint && <div className='mt-1 text-xs text-muted-foreground'>{c.hint}</div>}
        </div>
      ))}
    </div>
  )
}

function CatfkErrorBanner({ error }: { error?: string }) {
  const { t } = useTranslation()
  if (!error) return null
  return (
    <div className='flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive'>
      <AlertTriangle className='mt-0.5 size-4 shrink-0' />
      <div>
        <div className='font-medium'>{t('Catfk stock query failed')}</div>
        <div className='mt-1 break-all text-xs'>{error}</div>
      </div>
    </div>
  )
}

function OrphanBanner({ orphanCount }: { orphanCount: number }) {
  const { t } = useTranslation()
  if (orphanCount <= 0) return null
  return (
    <div className='flex items-center gap-2 rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive'>
      <AlertTriangle className='size-4 shrink-0' />
      <span>
        {orphanCount} {t('orphan card(s) on shelf not in production — buyers cannot redeem them. Clean them up.')}
      </span>
    </div>
  )
}

function ConfigCard({
  data,
  onRun,
  runPending,
}: {
  data: ReplenishOverview
  onRun: (goodsKey: string) => void
  runPending: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [lowWater, setLowWater] = useState(data.status.low_water)
  const [batch, setBatch] = useState(data.status.batch)
  const [enabled, setEnabled] = useState(data.status.enabled)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setLowWater(data.status.low_water)
    setBatch(data.status.batch)
    setEnabled(data.status.enabled)
  }, [data.status.low_water, data.status.batch, data.status.enabled])

  const save = async () => {
    setSaving(true)
    try {
      await updateReplenishConfig({ replenish_enabled: enabled, low_water: lowWater, batch })
      void queryClient.invalidateQueries({ queryKey: ['catfk-replenish-overview'] })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className='rounded-lg border bg-card p-4'>
      <div className='mb-3 text-sm font-medium'>{t('Settings & Manual Replenish')}</div>
      <div className='flex flex-wrap items-end gap-4'>
        <label className='flex items-center gap-2 text-sm'>
          <input type='checkbox' checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          {t('Auto replenish enabled')}
        </label>
        <label className='flex flex-col gap-1 text-sm'>
          <span className='text-muted-foreground'>{t('Low water')}</span>
          <input type='number' min={0} max={1000} value={lowWater} onChange={(e) => setLowWater(Number(e.target.value))} className='w-20 rounded border bg-background px-2 py-1' />
        </label>
        <label className='flex flex-col gap-1 text-sm'>
          <span className='text-muted-foreground'>{t('Batch size')}</span>
          <input type='number' min={1} max={100} value={batch} onChange={(e) => setBatch(Number(e.target.value))} className='w-20 rounded border bg-background px-2 py-1' />
        </label>
        <Button size='sm' variant='outline' onClick={save} disabled={saving}>
          {saving && <Loader2 className='size-3.5 animate-spin' />}
          {t('Save')}
        </Button>
        <div className='ml-auto'>
          <Button size='sm' onClick={() => onRun('')} disabled={runPending}>
            {runPending ? <Loader2 className='size-3.5 animate-spin' /> : <Zap className='size-3.5' />}
            {t('Replenish all now')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function StockPanel({
  data,
  onRun,
  runPending,
}: {
  data: ReplenishOverview
  onRun: (goodsKey: string) => void
  runPending: boolean
}) {
  const { t } = useTranslation()
  const maxBar = Math.max(15, data.status.low_water * 2, ...data.stock.map((r) => r.available))
  return (
    <div className='rounded-lg border bg-card p-4'>
      <div className='mb-3 text-sm font-medium'>{t('Stock by Goods')}</div>
      <div className='overflow-x-auto'>
        <table className='w-full text-sm'>
          <thead>
            <tr className='border-b text-left text-muted-foreground'>
              <th className='py-2 pr-4'>{t('Goods')}</th>
              <th className='py-2 pr-4'>{t('Type')}</th>
              <th className='py-2 pr-4'>{t('Available')}</th>
              <th className='py-2 pr-4'>{t('Level')}</th>
              <th className='py-2 pr-4'>{t('Action')}</th>
            </tr>
          </thead>
          <tbody>
            {data.stock.map((r) => {
              const low = r.available >= 0 && r.available < r.low_water
              return (
                <tr key={r.goods_key} className='border-b last:border-0'>
                  <td className='py-2 pr-4 font-mono'>{r.goods_key}</td>
                  <td className='py-2 pr-4'>{r.kind === 'plan' ? t('Plan') : t('Quota')}</td>
                  <td className={cn('py-2 pr-4 font-medium', low && 'text-destructive')}>
                    {r.available < 0 ? '-' : r.available}
                    <span className='text-muted-foreground'> / {r.low_water}</span>
                  </td>
                  <td className='py-2 pr-4 w-40'>
                    <div className='h-2 w-full overflow-hidden rounded bg-muted'>
                      <div
                        className={cn('h-full', low ? 'bg-destructive' : 'bg-emerald-500')}
                        style={{ width: `${Math.min(100, ((r.available < 0 ? 0 : r.available) / maxBar) * 100)}%` }}
                      />
                    </div>
                  </td>
                  <td className='py-2 pr-4'>
                    <Button size='sm' variant='outline' onClick={() => onRun(r.goods_key)} disabled={runPending}>
                      {t('Replenish')}
                    </Button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function EventsTable({ events, loading }: { events: ReplenishEvent[]; loading: boolean }) {
  const { t } = useTranslation()
  return (
    <div className='rounded-lg border bg-card p-4'>
      <div className='mb-3 text-sm font-medium'>{t('Replenish History')}</div>
      {loading && (
        <div className='flex items-center gap-2 text-muted-foreground'><Loader2 className='size-4 animate-spin' /> {t('Loading...')}</div>
      )}
      {!loading && events.length === 0 && (
        <div className='text-sm text-muted-foreground'>{t('No replenish events yet')}</div>
      )}
      {!loading && events.length > 0 && (
        <div className='overflow-x-auto'>
          <table className='w-full text-sm'>
            <thead>
              <tr className='border-b text-left text-muted-foreground'>
                <th className='py-2 pr-4'>{t('Time')}</th>
                <th className='py-2 pr-4'>{t('Goods')}</th>
                <th className='py-2 pr-4'>{t('Trigger')}</th>
                <th className='py-2 pr-4'>{t('Generated')}</th>
                <th className='py-2 pr-4'>{t('Upload')}</th>
                <th className='py-2 pr-4'>{t('Error')}</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id} className='border-b last:border-0'>
                  <td className='py-2 pr-4 text-muted-foreground'>{new Date(e.created_at * 1000).toLocaleString()}</td>
                  <td className='py-2 pr-4 font-mono'>{e.goods_key}</td>
                  <td className='py-2 pr-4'>
                    <span className='rounded bg-muted px-1.5 py-0.5 text-xs'>{e.trigger}</span>
                  </td>
                  <td className='py-2 pr-4'>{e.codes_generated}</td>
                  <td className='py-2 pr-4'>
                    {e.catfk_upload_ok ? (
                      <CheckCircle2 className='size-4 text-emerald-500' />
                    ) : (
                      <AlertTriangle className='size-4 text-destructive' />
                    )}
                  </td>
                  <td className='py-2 pr-4 max-w-xs truncate text-muted-foreground' title={e.error_message}>{e.error_message || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
