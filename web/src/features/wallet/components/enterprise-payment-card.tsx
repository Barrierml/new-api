import { Building2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StyledQRCode } from '@/components/qr-code-styling'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { useStatus } from '@/hooks/use-status'

/**
 * 企业 / 对公支付入口。
 * 说明性区块:支持为企业组织开具发票、支持对公转账付款;
 * 目前不接入在线支付,直接常显客服二维码引导联系。
 * 二维码常显(非 hover),避免被上方充值区遮挡。
 */
export function EnterprisePaymentCard() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const url = status?.support_qrcode_url as string | undefined

  return (
    <Card data-card-hover='false' className='bg-muted/20 py-0'>
      <CardContent className='flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:p-4'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <IconBadge tone='primary'>
            <Building2 />
          </IconBadge>
          <div className='min-w-0'>
            <h3 className='text-sm font-semibold'>
              {t('Enterprise / Bank Transfer')}
            </h3>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t(
                'Supports invoicing to organizations and bank transfer payments. Please contact support to proceed.'
              )}
            </p>
          </div>
        </div>
        {url ? (
          <div className='flex shrink-0 flex-col items-center gap-1 self-start sm:self-center'>
            <div className='rounded-lg bg-white p-1.5'>
              <StyledQRCode data={url} size={104} />
            </div>
            <span className='text-muted-foreground text-xs'>
              {t('Scan with WeChat')}
            </span>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}