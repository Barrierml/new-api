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
import type { ReactNode } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { cn } from '@/lib/utils'

import {
  extractCodeLanguage,
  extractTextFromChildren,
  makeUniqueHeadingId,
  resolveMdLink,
} from '../lib/docs'
import { DocsCodeBlock } from './docs-code-block'

interface DocsMarkdownProps {
  content: string
  /** 当前文档的 slug,用于解析相对 .md 链接。 */
  currentSlug: string
  /** 点击内部文档链接时的回调(由父组件做 TanStack 客户端跳转)。 */
  onNavigate: (href: string) => void
}

const PROSE_CLASS = cn(
  'prose prose-sm dark:prose-invert max-w-none',
  '[&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:text-2xl [&_h1]:font-semibold',
  '[&_h2]:mt-6 [&_h2]:mb-3 [&_h2]:scroll-mt-24 [&_h2]:text-xl [&_h2]:font-semibold',
  '[&_h3]:mt-5 [&_h3]:mb-2 [&_h3]:scroll-mt-24 [&_h3]:text-lg [&_h3]:font-semibold',
  '[&_p]:my-3 [&_p]:leading-relaxed',
  '[&_a]:text-primary [&_a]:underline hover:[&_a]:text-primary/80',
  '[&_ol]:my-3 [&_ul]:my-3 [&_ol]:list-decimal [&_ul]:list-disc [&_ol]:pl-5 [&_ul]:pl-5 [&_li]:my-1',
  '[&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-primary [&_blockquote]:bg-muted/50 [&_blockquote]:py-1 [&_blockquote]:pl-4',
  '[&_:not(pre)_>code]:rounded [&_:not(pre)_>code]:bg-muted [&_:not(pre)_>code]:px-1 [&_:not(pre)_>code]:py-0.5 [&_:not(pre)_>code]:font-mono [&_:not(pre)_>code]:text-[0.85em]',
  '[&_table]:my-4 [&_table]:block [&_table]:w-full [&_table]:overflow-x-auto',
  '[&_thead]:bg-muted [&_th]:border [&_td]:border [&_th]:px-3 [&_td]:px-3 [&_th]:py-2 [&_td]:py-2 [&_th]:text-left',
  '[&_hr]:my-6',
  '[&>*:first-child]:mt-0 [&>*:last-child]:mb-0',
  '[overflow-wrap:anywhere]',
)

export function DocsMarkdown(props: DocsMarkdownProps) {
  // 每次渲染新建一个 seen Map,保证 h2/h3 的 heading id 与 parseAnchors 算出的一致。
  // (内容稳定时,标题顺序不变 → id 不变;不依赖 useMemo 避免与 hooks 规则冲突。)
  const seen = new Map<string, number>()
  const { content, currentSlug, onNavigate } = props

  const components = {
    h2({ children }: { children?: ReactNode }) {
      const id = makeUniqueHeadingId(extractTextFromChildren(children), seen)
      return <h2 id={id}>{children}</h2>
    },
    h3({ children }: { children?: ReactNode }) {
      const id = makeUniqueHeadingId(extractTextFromChildren(children), seen)
      return <h3 id={id}>{children}</h3>
    },
    pre({ children }: { children?: ReactNode }) {
      const code = extractTextFromChildren(children)
      const language = extractCodeLanguage(children)
      return (
        <DocsCodeBlock code={code} language={language}>
          {children}
        </DocsCodeBlock>
      )
    },
    a({
      href,
      children,
    }: {
      href?: string
      children?: ReactNode
    }) {
      const h = href ?? ''
      const resolved = resolveMdLink(h, currentSlug)
      if (resolved.internal) {
        return (
          <a
            href={resolved.href}
            onClick={(e) => {
              if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) {
                return
              }
              e.preventDefault()
              onNavigate(resolved.href)
            }}
          >
            {children}
          </a>
        )
      }
      const isExternal = /^https?:\/\//i.test(h)
      return (
        <a
          href={h}
          target={isExternal ? '_blank' : undefined}
          rel={isExternal ? 'noopener noreferrer' : undefined}
        >
          {children}
        </a>
      )
    },
  }

  return (
    <div className={PROSE_CLASS}>
      <Markdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </Markdown>
    </div>
  )
}
