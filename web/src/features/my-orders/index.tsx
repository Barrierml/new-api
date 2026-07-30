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
import {
  AlertTriangle,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Copy,
  ExternalLink,
  Loader2,
  RefreshCw,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'
import { useQuery } from '@tanstack/react-query'

import { getGoodsFullLabel } from '../auto-replenish/goods-meta'
import { getMyOrders } from './api'
import type { CatfkOrderItem } from './types'

const PAGE_SIZE = 20

export function MyOrders() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)

  const ordersQuery = useQuery({
    queryKey: ['my-catfk-orders', page],
    queryFn: async () => {
      const r = await getMyOrders(page, PAGE_SIZE)
      if (!r.success) throw new Error(r.message || 'failed')
      return r.data
    },
    refetchInterval: 15_000, // 15s 轮询,用户付完款回来看时自动刷新状态
  })

  const items = ordersQuery.data?.items ?? []
  const total = ordersQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('My Orders')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void ordersQuery.refetch()}
          disabled={ordersQuery.isFetching}
        >
          <RefreshCw
            className={cn(
              'size-3.5',
              ordersQuery.isFetching && 'animate-spin'
            )}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-5xl flex-col gap-4'>
          {ordersQuery.isLoading && (
            <div className='flex items-center gap-2 text-muted-foreground'>
              <Loader2 className='size-4 animate-spin' /> {t('Loading...')}
            </div>
          )}
          {ordersQuery.isError && (
            <div className='rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive'>
              {t('Failed to load orders')}
            </div>
          )}
          {!ordersQuery.isLoading && items.length === 0 && (
            <div className='rounded-lg border bg-card p-8 text-center text-sm text-muted-foreground'>
              {t('No orders yet — purchases made via the Wallet page will appear here.')}
            </div>
          )}
          {items.length > 0 && (
            <>
              <div className='overflow-x-auto rounded-lg border bg-card'>
                <table className='w-full text-sm'>
                  <thead>
                    <tr className='border-b text-left text-muted-foreground'>
                      <th className='px-4 py-3'>{t('Time')}</th>
                      <th className='px-4 py-3'>{t('Product')}</th>
                      <th className='px-4 py-3'>{t('Payment Method')}</th>
                      <th className='px-4 py-3'>{t('Status')}</th>
                      <th className='px-4 py-3'>{t('Order ID')}</th>
                      <th className='px-4 py-3'>{t('Action')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((order) => (
                      <OrderRow key={order.id} order={order} />
                    ))}
                  </tbody>
                </table>
              </div>
              {totalPages > 1 && (
                <div className='flex items-center justify-between text-sm'>
                  <span className='text-muted-foreground'>
                    {t('Page')} {page} / {totalPages} · {total} {t('orders')}
                  </span>
                  <div className='flex gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => setPage((p) => Math.max(1, p - 1))}
                      disabled={page === 1}
                    >
                      <ChevronLeft className='size-3.5' />
                      {t('Previous')}
                    </Button>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        setPage((p) => Math.min(totalPages, p + 1))
                      }
                      disabled={page === totalPages}
                    >
                      {t('Next')}
                      <ChevronRight className='size-3.5' />
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function OrderRow({ order }: { order: CatfkOrderItem }) {
  const { t } = useTranslation()
  const { copyToClipboard, copiedText } = useCopyToClipboard({
    notify: false,
  })
  const isPending = !order.granted

  const handleCopyTradeNo = () => {
    copyToClipboard(order.trade_no)
    toast.success(t('Order ID copied'))
  }

  const handleReopenPayUrl = () => {
    if (order.payurl) {
      window.open(order.payurl, '_blank', 'noopener,noreferrer')
    }
  }

  return (
    <tr className='border-b last:border-0'>
      <td className='px-4 py-3 text-muted-foreground'>
        {new Date(order.created_at * 1000).toLocaleString()}
      </td>
      <td className='px-4 py-3'>
        <div className='font-medium'>{getGoodsFullLabel(order.goods_key)}</div>
        <div className='font-mono text-xs text-muted-foreground'>
          {order.goods_key}
        </div>
      </td>
      <td className='px-4 py-3'>
        {order.pay === 'alipay'
          ? t('Alipay')
          : order.pay === 'wechat'
            ? t('WeChat Pay')
            : order.pay}
      </td>
      <td className='px-4 py-3'>
        {order.granted ? (
          <span className='inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400'>
            <CheckCircle2 className='size-3.5' />
            {t('Fulfilled')}
          </span>
        ) : (
          <span className='inline-flex items-center gap-1 text-amber-600 dark:text-amber-400'>
            <AlertTriangle className='size-3.5' />
            {t('Pending payment')}
          </span>
        )}
      </td>
      <td className='px-4 py-3'>
        <button
          onClick={handleCopyTradeNo}
          className='inline-flex items-center gap-1 font-mono text-xs text-muted-foreground hover:text-foreground'
          title={t('Click to copy')}
        >
          {order.trade_no}
          <Copy
            className={cn(
              'size-3',
              copiedText === order.trade_no && 'text-emerald-500'
            )}
          />
        </button>
      </td>
      <td className='px-4 py-3'>
        {isPending && order.payurl && (
          <Button
            variant='outline'
            size='sm'
            onClick={handleReopenPayUrl}
          >
            <ExternalLink className='size-3.5' />
            {t('Continue payment')}
          </Button>
        )}
      </td>
    </tr>
  )
}
