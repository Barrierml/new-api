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
  ArrowRight,
  BookOpen,
  KeyRound,
  ListTree,
  Zap,
} from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  type DocEntry,
  type DocGroup,
  GROUP_LABEL_KEY,
  GROUP_META,
  listDocs,
} from '../lib/docs'
import { GROUP_ICONS } from './docs-icons'

interface DocsHomeProps {
  brand: string
  onNavigate: (href: string) => void
}

/** 文档着陆页:hero + 分组卡片栅格 + 热门页面快捷链接(Mintlify 风)。 */
export function DocsHome(props: DocsHomeProps) {
  const { t } = useTranslation()
  const entries = useMemo(() => listDocs(), [])

  // 每个分组的第一篇文档作为卡片跳转目标
  const firstByGroup = useMemo(() => {
    const map = new Map<DocGroup, DocEntry>()
    for (const e of entries) {
      if (e.group === 'index') continue
      if (!map.has(e.group)) map.set(e.group, e)
    }
    return map
  }, [entries])

  const popular = useMemo(
    () =>
      [
        { slug: '00-quickstart', icon: Zap, labelKey: 'docs.popular.quickstart' },
        { slug: '01-authentication', icon: KeyRound, labelKey: 'docs.popular.auth' },
        { slug: '02-models', icon: ListTree, labelKey: 'docs.popular.models' },
      ]
        .map((p) => ({ ...p, doc: entries.find((e) => e.slug === p.slug) }))
        .filter((p) => p.doc),
    [entries],
  )

  const go = (e: React.MouseEvent, href: string) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
    e.preventDefault()
    props.onNavigate(href)
  }

  const groups = [...firstByGroup.entries()]

  return (
    <div className='relative'>
      {/* Hero:轻微渐变底,克制不花哨 */}
      <div className='from-primary/5 via-background to-background pointer-events-none absolute inset-x-0 top-0 h-72 bg-gradient-to-b' />
      <div className='relative px-6 pb-16 pt-16 md:pt-24'>
        <div className='mx-auto max-w-4xl text-center'>
          <div className='bg-primary/10 text-primary mx-auto mb-6 flex size-14 items-center justify-center rounded-2xl'>
            <BookOpen className='size-7' />
          </div>
          <h1 className='text-4xl font-bold tracking-tight md:text-5xl'>
            {props.brand} {t('Documentation')}
          </h1>
          <p className='text-muted-foreground mx-auto mt-4 max-w-xl text-base leading-7'>
            {t('docs.hero.subtitle')}
          </p>
        </div>

        {/* 分组卡片栅格 */}
        <div className='mx-auto mt-14 grid max-w-4xl gap-4 sm:grid-cols-2'>
          {groups.map(([group, doc]) => {
            const meta = GROUP_META[group]
            const Icon = GROUP_ICONS[meta.icon]
            const href = doc.slug ? `/docs/${doc.slug}` : '/docs'
            return (
              <a
                key={group}
                href={href}
                onClick={(e) => go(e, href)}
                className='group bg-card hover:border-primary/40 relative flex flex-col rounded-xl border p-6 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg'
              >
                <div className='bg-primary/10 text-primary mb-4 flex size-10 items-center justify-center rounded-lg'>
                  <Icon className='size-5' />
                </div>
                <div className='font-semibold'>{t(GROUP_LABEL_KEY[group])}</div>
                <p className='text-muted-foreground mt-1.5 text-sm leading-6'>
                  {t(meta.descKey)}
                </p>
                <ArrowRight className='text-muted-foreground/40 group-hover:text-primary absolute right-5 top-5 size-4 transition-all duration-200 group-hover:translate-x-0.5' />
              </a>
            )
          })}
        </div>

        {/* 热门页面 */}
        {popular.length > 0 && (
          <div className='mx-auto mt-12 max-w-4xl'>
            <h2 className='text-muted-foreground mb-3 text-xs font-semibold uppercase tracking-wider'>
              {t('docs.home.popular')}
            </h2>
            <div className='grid gap-2 sm:grid-cols-3'>
              {popular.map((p) => {
                const Icon = p.icon
                const href = `/docs/${p.doc!.slug}`
                return (
                  <a
                    key={p.doc!.slug}
                    href={href}
                    onClick={(e) => go(e, href)}
                    className='group hover:bg-muted/50 flex items-center gap-2.5 rounded-lg border px-4 py-3 text-sm transition-colors'
                  >
                    <Icon className='text-muted-foreground group-hover:text-primary size-4 shrink-0 transition-colors' />
                    <span className='truncate font-medium'>{p.doc!.title}</span>
                    <ArrowRight className='text-muted-foreground/40 ml-auto size-3.5 shrink-0 transition-transform group-hover:translate-x-0.5' />
                  </a>
                )
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
