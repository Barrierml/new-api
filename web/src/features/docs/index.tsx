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
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { ArrowLeft, ChevronLeft, ChevronRight, Menu } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'

import { DocsMarkdown } from './components/docs-markdown'
import { DocsSidebar } from './components/docs-sidebar'
import { DocsToc } from './components/docs-toc'
import {
  type DocEntry,
  findDoc,
  injectTokens,
  listDocs,
  parseAnchors,
} from './lib/docs'

export function DocsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = useParams({ strict: false })
  const { status } = useStatus()

  const slug = String(params._splat ?? '').replaceAll(/^\/+|\/+$/g, '')
  const entries = useMemo(() => listDocs(), [])
  const current = useMemo(() => findDoc(slug) ?? findDoc(''), [slug])
  const [mobileNavOpen, setMobileNavOpen] = useState(false)

  const baseUrl = useMemo(() => {
    const s = status as Record<string, unknown> | null
    const candidate =
      s?.server_address ??
      s?.serverAddress ??
      (s?.data as Record<string, unknown> | undefined)?.server_address
    if (candidate && typeof candidate === 'string') {
      return candidate.replace(/\/$/, '')
    }
    return typeof window !== 'undefined' ? window.location.origin : ''
  }, [status])

  const brand = useMemo(() => {
    const s = status as Record<string, unknown> | null
    const candidate =
      s?.system_name ??
      s?.systemName ??
      (s?.data as Record<string, unknown> | undefined)?.system_name
    if (candidate && typeof candidate === 'string') return candidate
    return 'Tako'
  }, [status])

  const content = useMemo(
    () => injectTokens(current?.content ?? '', { baseUrl, brand }),
    [current, baseUrl, brand],
  )
  const anchors = useMemo(
    () => (current ? parseAnchors(content) : []),
    [current, content],
  )

  const onNavigate = (href: string) => {
    navigate({ to: href })
  }

  // 滚动容器:slug / hash 变化时滚到对应位置
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const hash = typeof window !== 'undefined' ? window.location.hash : ''
  useEffect(() => {
    const container = scrollRef.current
    if (!container) return
    if (hash) {
      const el = document.getElementById(decodeURIComponent(hash.slice(1)))
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'start' })
        return
      }
    }
    container.scrollTo({ top: 0, behavior: 'auto' })
  }, [slug, hash])

  useEffect(() => setMobileNavOpen(false), [slug])

  const idx = entries.findIndex((d) => d.slug === (current?.slug ?? ''))
  const prev = idx > 0 ? entries[idx - 1] : null
  const next = idx >= 0 && idx < entries.length - 1 ? entries[idx + 1] : null

  return (
    <div className='bg-background flex h-[100dvh] flex-col overflow-hidden'>
      <DocsTopBar onMobileNavOpen={() => setMobileNavOpen(true)} />

      <div className='flex flex-1 overflow-hidden'>
        {/* 左侧目录 — 桌面 */}
        <aside className='bg-card/40 hidden h-full w-60 shrink-0 border-r md:flex lg:w-64'>
          <DocsSidebar entries={entries} currentSlug={current?.slug ?? ''} />
        </aside>

        {/* 左侧目录 — 移动抽屉 */}
        {mobileNavOpen && (
          <div
            className='bg-black/50 fixed inset-0 z-40 backdrop-blur-sm md:hidden'
            onClick={() => setMobileNavOpen(false)}
          >
            <aside
              className='bg-card absolute left-0 top-0 flex h-full w-72 flex-col border-r shadow-xl'
              onClick={(e) => e.stopPropagation()}
            >
              <div className='flex h-14 items-center border-b px-4'>
                <span className='text-sm font-semibold'>
                  {t('Documentation')}
                </span>
              </div>
              <div className='min-h-0 flex-1'>
                <DocsSidebar
                  entries={entries}
                  currentSlug={current?.slug ?? ''}
                />
              </div>
            </aside>
          </div>
        )}

        {/* 中间正文 */}
        <main ref={scrollRef} className='flex-1 overflow-y-auto'>
          <article
            key={current?.slug ?? 'missing'}
            className='bg-card mx-auto mb-12 max-w-3xl rounded-xl border px-4 py-8 shadow-sm sm:px-6 sm:py-10 md:px-10 md:py-14'
          >
            {current ? (
              <DocsMarkdown
                content={content}
                currentSlug={current.slug}
                onNavigate={onNavigate}
              />
            ) : (
              <p className='text-muted-foreground'>
                {t('Documentation not found.')}{' '}
                <Link to='/docs' className='text-primary underline'>
                  {t('Back to documentation home')}
                </Link>
              </p>
            )}

            {current && current.slug && (prev || next) && (
              <DocFooterNav
                prev={prev}
                next={next}
                onNavigate={onNavigate}
              />
            )}
          </article>
        </main>

        {/* 右侧 TOC */}
        {anchors.length > 1 && (
          <aside className='bg-background/30 hidden w-56 shrink-0 border-l xl:block'>
            <DocsToc anchors={anchors} container={scrollRef} />
          </aside>
        )}
      </div>
    </div>
  )
}

function DocsTopBar({ onMobileNavOpen }: { onMobileNavOpen: () => void }) {
  const { t } = useTranslation()
  return (
    <header className='sticky top-0 z-30 flex h-14 items-center gap-3 border-b bg-background/80 px-4 backdrop-blur-md md:px-6'>
      <button
        type='button'
        onClick={onMobileNavOpen}
        className='hover:bg-muted rounded-md p-1.5 md:hidden'
        aria-label={t('Open navigation')}
      >
        <Menu className='size-5' />
      </button>
      <Link
        to='/docs'
        className='flex items-center gap-2 text-sm font-semibold'
      >
        {t('Documentation')}
      </Link>
      <Link
        to='/'
        className='text-muted-foreground hover:text-foreground ml-auto inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs'
      >
        <ArrowLeft className='size-3.5' />
        <span className='hidden sm:inline'>{t('Back to Home')}</span>
      </Link>
    </header>
  )
}

function DocFooterNav({
  prev,
  next,
  onNavigate,
}: {
  prev: DocEntry | null
  next: DocEntry | null
  onNavigate: (href: string) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='mt-12 grid grid-cols-2 gap-3 border-t pt-6'>
      {prev ? (
        <a
          href={prev.slug ? `/docs/${prev.slug}` : '/docs'}
          onClick={(e) => {
            e.preventDefault()
            onNavigate(prev.slug ? `/docs/${prev.slug}` : '/docs')
          }}
          className='hover:bg-muted/40 group flex flex-col rounded-lg border p-4 text-sm transition-colors'
        >
          <span className='text-muted-foreground flex items-center gap-1 text-[11px] uppercase tracking-wider'>
            <ChevronLeft className='size-3' />
            {t('Previous')}
          </span>
          <span className='group-hover:text-primary mt-1 font-medium transition-colors'>
            {prev.title}
          </span>
        </a>
      ) : (
        <span />
      )}
      {next ? (
        <a
          href={next.slug ? `/docs/${next.slug}` : '/docs'}
          onClick={(e) => {
            e.preventDefault()
            onNavigate(next.slug ? `/docs/${next.slug}` : '/docs')
          }}
          className='group flex flex-col items-end rounded-lg border p-4 text-right text-sm transition-colors hover:bg-muted/40'
        >
          <span className='text-muted-foreground flex items-center gap-1 text-[11px] uppercase tracking-wider'>
            {t('Next')}
            <ChevronRight className='size-3' />
          </span>
          <span className='group-hover:text-primary mt-1 font-medium transition-colors'>
            {next.title}
          </span>
        </a>
      ) : (
        <span />
      )}
    </div>
  )
}
