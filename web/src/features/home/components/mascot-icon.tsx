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
import { cn } from '@/lib/utils'

interface MascotIconProps {
  size?: number
  withPulse?: boolean
  className?: string
}

/** Tako 章鱼吉祥物 — Hero 区域用。复用 /logo.svg + 双脉冲环 + 珊瑚色阴影。 */
export function MascotIcon({
  size = 80,
  withPulse = true,
  className = '',
}: MascotIconProps) {
  return (
    <div
      className={cn('relative inline-flex items-center justify-center', className)}
      style={{ width: size, height: size }}
    >
      {withPulse && (
        <>
          <span
            className='absolute inset-0 rounded-[28%] bg-orange-500/25 animate-ping'
            style={{ animationDuration: '2.6s' }}
            aria-hidden='true'
          />
          <span
            className='absolute inset-0 rounded-[28%] bg-orange-500/25 animate-ping'
            style={{ animationDuration: '2.6s', animationDelay: '1.3s' }}
            aria-hidden='true'
          />
        </>
      )}
      <img
        src='/logo.svg'
        alt='Tako'
        className='relative z-10 h-full w-full rounded-[22%] drop-shadow-lg'
        style={{ filter: 'drop-shadow(0 8px 24px rgba(240, 104, 88, 0.35))' }}
      />
    </div>
  )
}
