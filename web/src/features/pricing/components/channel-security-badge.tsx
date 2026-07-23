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
import { ShieldAlert, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

// Channel security category. The backend stores the raw Chinese tag on the
// channel (`安全` / `无法验证安全性`); we classify it into a stable enum here so
// the visual treatment is decoupled from the exact string.
type ChannelSecurity = 'secure' | 'unverified' | 'unknown'

const SECURE_TAG = '安全'
const UNVERIFIED_TAG = '无法验证安全性'

function classifyChannelSecurity(tag?: string): ChannelSecurity {
  const t = (tag ?? '').trim()
  if (t === SECURE_TAG) return 'secure'
  if (t === UNVERIFIED_TAG) return 'unverified'
  return 'unknown'
}

interface ChannelSecurityBadgeProps {
  tag?: string
  /** Compact = icon only (table cells); default shows icon + label. */
  compact?: boolean
  className?: string
}

export function ChannelSecurityBadge(props: ChannelSecurityBadgeProps) {
  const { t } = useTranslation()
  const kind = classifyChannelSecurity(props.tag)
  if (kind === 'unknown') return null

  const isSecure = kind === 'secure'
  const Icon = isSecure ? ShieldCheck : ShieldAlert
  const label = isSecure
    ? t('Security verified')
    : t('Security unverified')
  const tip = isSecure
    ? t('This channel uses official / self-hosted upstreams we can vouch for.')
    : t('This channel routes through a third-party relay whose security we cannot verify.')

  const badge = (
    <span
      className={cn(
        'inline-flex h-5 w-fit shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-xs font-medium',
        isSecure
          ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
          : 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400',
        props.className
      )}
    >
      <Icon className='size-3' />
      {!props.compact && <span>{label}</span>}
    </span>
  )

  return (
    <Tooltip>
      <TooltipTrigger render={badge} />
      <TooltipContent>
        <p className='max-w-52'>{tip}</p>
      </TooltipContent>
    </Tooltip>
  )
}
