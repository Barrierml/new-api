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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { AnchorEntry } from '../lib/docs'

interface DocsTocProps {
  anchors: AnchorEntry[]
  container: React.RefObject<HTMLDivElement | null>
}

export function DocsToc(props: DocsTocProps) {
  const { t } = useTranslation()
  const [activeId, setActiveId] = useState<string>(
    props.anchors[0]?.id ?? '',
  )

  useEffect(() => {
    setActiveId(props.anchors[0]?.id ?? '')
    const root = props.container.current
    if (!root || props.anchors.length === 0) return
    const observed: HTMLElement[] = []
    for (const a of props.anchors) {
      const el = document.getElementById(a.id)
      if (el) observed.push(el)
    }
    if (observed.length === 0) return
    const obs = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort(
            (a, b) => a.boundingClientRect.top - b.boundingClientRect.top,
          )[0]
        if (visible) setActiveId(visible.target.id)
      },
      { root, rootMargin: '-80px 0px -60% 0px', threshold: 0 },
    )
    observed.forEach((el) => obs.observe(el))
    return () => obs.disconnect()
  }, [props.anchors, props.container])

  if (props.anchors.length <= 1) return null

  return (
    <div className='sticky top-14 max-h-[calc(100vh-4rem)] overflow-y-auto py-8 pl-6 pr-4'>
      <div className='text-muted-foreground mb-3 text-[11px] font-semibold uppercase tracking-wider'>
        {t('On this page')}
      </div>
      <ul className='space-y-1'>
        {props.anchors.map((a) => (
          <li
            key={a.id}
            style={{ paddingLeft: a.level === 3 ? '0.75rem' : 0 }}
          >
            <a
              href={`#${a.id}`}
              className={cn(
                'text-muted-foreground/80 block py-1 pl-3 text-[12.5px] leading-snug',
                activeId === a.id && 'text-foreground font-medium',
              )}
            >
              {a.text}
            </a>
          </li>
        ))}
      </ul>
    </div>
  )
}
