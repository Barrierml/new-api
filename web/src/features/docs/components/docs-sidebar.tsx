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
import { Link } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import {
  type DocEntry,
  type DocGroup,
  GROUP_LABEL_KEY,
} from '../lib/docs'

interface DocsSidebarProps {
  entries: DocEntry[]
  currentSlug: string
}

const SECTION_ORDER: DocGroup[] = [
  'index',
  'overview',
  'cli',
  'api',
  'integrations',
]

export function DocsSidebar(props: DocsSidebarProps) {
  const { t } = useTranslation()

  const grouped = useMemo(() => {
    const map = new Map<DocGroup, DocEntry[]>()
    for (const e of props.entries) {
      const arr = map.get(e.group) ?? []
      arr.push(e)
      map.set(e.group, arr)
    }
    return map
  }, [props.entries])

  return (
    <nav className='flex h-full flex-col gap-5 overflow-y-auto p-4'>
      {SECTION_ORDER.filter((g) => grouped.has(g)).map((g) => {
        const items = grouped.get(g) ?? []
        const isIndex = g === 'index'
        return (
          <section key={g}>
            {!isIndex && (
              <h2 className='text-muted-foreground mb-1.5 px-3 text-[11px] font-semibold uppercase tracking-wider'>
                {t(GROUP_LABEL_KEY[g])}
              </h2>
            )}
            <div className='space-y-0.5'>
              {items.map((d) => (
                <NavLink
                  key={d.slug || 'index'}
                  doc={d}
                  active={props.currentSlug === d.slug}
                  label={isIndex ? t('Documentation Home') : undefined}
                />
              ))}
            </div>
          </section>
        )
      })}
    </nav>
  )
}

function NavLink({
  doc,
  active,
  label,
}: {
  doc: DocEntry
  active: boolean
  label?: string
}) {
  const to = doc.slug ? `/docs/${doc.slug}` : '/docs'
  return (
    <Link
      to={to}
      className={cn(
        'block rounded-md px-3 py-1.5 text-[13.5px] transition-colors',
        active
          ? 'bg-primary/10 text-primary font-medium'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground',
      )}
    >
      {label ?? doc.title}
    </Link>
  )
}
