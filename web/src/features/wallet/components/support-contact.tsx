import { Headphones } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StyledQRCode } from '@/components/qr-code-styling'
import { useStatus } from '@/hooks/use-status'

interface SupportContactProps {
  variant?: 'inline' | 'floating'
}

/**
 * 联系客服(企业微信)入口。读 /api/status 的 support_qrcode_url /
 * support_qrcode_description;URL 为空时整块不渲染。
 *
 * - floating:钱包页右下角 FAB,醒目大按钮。
 * - inline:购买弹窗底部用,低调文字链(右对齐),不抢主操作视觉;
 *   hover 弹出二维码气泡,写明用微信扫码。
 */
export function SupportContact({ variant = 'inline' }: SupportContactProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const url = status?.support_qrcode_url as string | undefined
  const description =
    (status?.support_qrcode_description as string | undefined) ||
    t('Working hours 9:00 - 22:00 · Fast response')

  if (!url) return null

  const popup = (
    <div className='pointer-events-none absolute bottom-full right-0 z-50 mb-3 scale-95 opacity-0 transition-all duration-200 group-hover/cs:pointer-events-auto group-hover/cs:scale-100 group-hover/cs:opacity-100'>
      <div className='relative flex w-56 flex-col items-center gap-2 rounded-2xl border bg-card p-4 shadow-2xl'>
        <div className='absolute -bottom-2 right-4 h-4 w-4 rotate-45 border-b border-r bg-card' />
        <p className='text-sm font-semibold'>
          {t('Scan with WeChat to contact support')}
        </p>
        <div className='rounded-lg bg-white p-1.5'>
          <StyledQRCode data={url} size={160} />
        </div>
        <p className='text-muted-foreground text-xs'>{description}</p>
        <p className='text-muted-foreground text-[11px]'>
          {t('Please scan with WeChat')}
        </p>
      </div>
    </div>
  )

  if (variant === 'floating') {
    return (
      <div className='fixed bottom-6 right-6 z-50'>
        <div className='group/cs relative'>
          <button
            type='button'
            className='flex h-12 items-center justify-center gap-2.5 rounded-xl bg-primary px-4 font-bold text-primary-foreground shadow-lg shadow-primary/20 transition-all hover:bg-primary/90 hover:shadow-primary/30'
          >
            <Headphones className='h-5 w-5' />
            <span>{t('Contact support')}</span>
            <span className='ml-1 text-xs opacity-70'>
              {t('Enterprise WeChat')}
            </span>
          </button>
          {popup}
        </div>
      </div>
    )
  }

  // inline:弹窗底部,低调文字链,右对齐
  return (
    <div className='group/cs relative flex justify-end pt-1'>
      <button
        type='button'
        className='inline-flex items-center gap-1.5 text-muted-foreground text-xs transition-colors hover:text-primary'
      >
        <Headphones className='h-3.5 w-3.5' />
        <span>{t('Contact support')}</span>
      </button>
      {popup}
    </div>
  )
}