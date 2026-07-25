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
  CircleAlert,
  Info,
  Lightbulb,
  OctagonX,
  TriangleAlert,
} from 'lucide-react'
import {
  Children,
  cloneElement,
  isValidElement,
  type ReactElement,
  type ReactNode,
} from 'react'
import Markdown from 'react-markdown'
import { useTranslation } from 'react-i18next'
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

// Mintlify 风排版:h1 大标题、h2 分区线、链接去下划线、品牌色行内 code、
// 表格行分割线(去全格边框)、宽松的 CJK 行高。
const PROSE_CLASS = cn(
  'max-w-none text-[15px]',
  '[&_h1]:mt-2 [&_h1]:mb-4 [&_h1]:scroll-mt-24 [&_h1]:text-3xl [&_h1]:font-bold [&_h1]:tracking-tight',
  '[&_h2]:mt-10 [&_h2]:mb-4 [&_h2]:scroll-mt-24 [&_h2]:border-b [&_h2]:pb-2 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:tracking-tight',
  '[&_h3]:mt-8 [&_h3]:mb-3 [&_h3]:scroll-mt-24 [&_h3]:text-base [&_h3]:font-semibold',
  '[&_h4]:mt-6 [&_h4]:mb-2 [&_h4]:text-sm [&_h4]:font-semibold',
  '[&_p]:my-4 [&_p]:leading-7 [&_p]:text-foreground/90',
  '[&_strong]:font-semibold [&_strong]:text-foreground',
  '[&_a]:font-medium [&_a]:text-primary [&_a]:no-underline [&_a]:underline-offset-4 hover:[&_a]:underline',
  '[&_ol]:my-4 [&_ol]:list-decimal [&_ol]:pl-6 [&_ul]:my-4 [&_ul]:list-disc [&_ul]:pl-6',
  '[&_li]:my-1.5 [&_li]:leading-7 [&_li::marker]:text-muted-foreground/60',
  '[&_:not(pre)_>code]:rounded-md [&_:not(pre)_>code]:border [&_:not(pre)_>code]:border-primary/15 [&_:not(pre)_>code]:bg-primary/5 [&_:not(pre)_>code]:px-1.5 [&_:not(pre)_>code]:py-0.5 [&_:not(pre)_>code]:font-mono [&_:not(pre)_>code]:text-[0.85em] [&_:not(pre)_>code]:text-primary',
  '[&_table]:my-6 [&_table]:block [&_table]:w-full [&_table]:overflow-x-auto [&_table]:rounded-lg [&_table]:border [&_table]:text-sm',
  '[&_thead]:bg-muted/60',
  '[&_th]:px-3 [&_th]:py-2.5 [&_th]:text-left [&_th]:font-medium',
  '[&_td]:border-t [&_td]:px-3 [&_td]:py-2.5 [&_td]:align-top [&_td]:leading-6',
  '[&_hr]:my-10 [&_hr]:border-border/60',
  '[&_img]:my-6 [&_img]:rounded-lg [&_img]:border',
  '[overflow-wrap:anywhere]',
  '[&>*:first-child]:mt-0 [&>*:last-child]:mb-0',
)

// ── GFM alert(> [!NOTE] 等)→ 彩色 callout 卡片 ─────────────────────────────

type AlertKind = 'note' | 'tip' | 'important' | 'warning' | 'caution'

const ALERT_META: Record<
  AlertKind,
  { icon: typeof Info; labelKey: string; className: string }
> = {
  note: {
    icon: Info,
    labelKey: 'Note',
    className:
      'border-blue-500/30 bg-blue-500/10 [&_.alert-title]:text-blue-600 dark:[&_.alert-title]:text-blue-400',
  },
  tip: {
    icon: Lightbulb,
    labelKey: 'Tip',
    className:
      'border-emerald-500/30 bg-emerald-500/10 [&_.alert-title]:text-emerald-600 dark:[&_.alert-title]:text-emerald-400',
  },
  important: {
    icon: CircleAlert,
    labelKey: 'Important',
    className:
      'border-violet-500/30 bg-violet-500/10 [&_.alert-title]:text-violet-600 dark:[&_.alert-title]:text-violet-400',
  },
  warning: {
    icon: TriangleAlert,
    labelKey: 'Warning',
    className:
      'border-amber-500/30 bg-amber-500/10 [&_.alert-title]:text-amber-600 dark:[&_.alert-title]:text-amber-400',
  },
  caution: {
    icon: OctagonX,
    labelKey: 'Caution',
    className:
      'border-rose-500/30 bg-rose-500/10 [&_.alert-title]:text-rose-600 dark:[&_.alert-title]:text-rose-400',
  },
}

/** 检测 blockquote 首段的 [!X] 前缀;命中则剥除前缀,返回 alert 类型 + 剩余内容。 */
function splitAlert(
  children: ReactNode,
): { kind: AlertKind; body: ReactNode } | null {
  const arr = Children.toArray(children)
  for (let i = 0; i < arr.length; i++) {
    const el = arr[i]
    if (!isValidElement(el)) continue
    const inner = Children.toArray(
      (el as ReactElement<{ children?: ReactNode }>).props.children,
    )
    const head = inner[0]
    if (typeof head !== 'string') continue
    const m = head.match(/^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*/i)
    if (!m) continue
    const kind = m[1].toLowerCase() as AlertKind
    const rest = head.slice(m[0].length)
    const newInner = [...inner]
    if (rest) {
      newInner[0] = rest
    } else {
      newInner.shift()
      const next = newInner[0]
      if (isValidElement(next) && next.type === 'br') newInner.shift()
    }
    const body = [...arr]
    body[i] = cloneElement(el as ReactElement, undefined, ...newInner)
    return { kind, body }
  }
  return null
}

function DocsBlockquote({ children }: { children?: ReactNode }) {
  const { t } = useTranslation()
  const alert = splitAlert(children)

  if (alert) {
    const meta = ALERT_META[alert.kind]
    const Icon = meta.icon
    return (
      <div
        className={cn(
          'my-5 rounded-lg border px-4 py-3 [&_p]:my-1.5 [&_p]:leading-6',
          meta.className,
        )}
      >
        <div className='alert-title mb-1 flex items-center gap-1.5 text-sm font-semibold'>
          <Icon className='size-4' />
          {t(meta.labelKey)}
        </div>
        <div className='text-sm text-foreground/90'>{alert.body}</div>
      </div>
    )
  }

  return (
    <blockquote className='my-5 rounded-r-lg border-l-2 border-primary/40 bg-muted/30 px-4 py-3 [&_p]:my-1.5 [&_p]:leading-6'>
      {children}
    </blockquote>
  )
}

export function DocsMarkdown(props: DocsMarkdownProps) {
  // 每次渲染新建一个 seen Map,保证 h2/h3 的 heading id 与 parseAnchors 算出的一致。
  // (内容稳定时,标题顺序不变 → id 不变;不依赖 useMemo 避免与 hooks 规则冲突。)
  const seen = new Map<string, number>()
  const { content, currentSlug, onNavigate } = props

  const components = {
    h1({ children }: { children?: ReactNode }) {
      // 「00 — 快速开始」→「快速开始」,序号在侧边栏/排序里已经表达过了
      const arr = Children.toArray(children)
      if (typeof arr[0] === 'string') {
        arr[0] = arr[0].replace(/^\d+\s*[—-]\s*/, '')
      }
      return <h1>{arr}</h1>
    },
    h2({ children }: { children?: ReactNode }) {
      const id = makeUniqueHeadingId(extractTextFromChildren(children), seen)
      return <h2 id={id}>{children}</h2>
    },
    h3({ children }: { children?: ReactNode }) {
      const id = makeUniqueHeadingId(extractTextFromChildren(children), seen)
      return <h3 id={id}>{children}</h3>
    },
    blockquote({ children }: { children?: ReactNode }) {
      return <DocsBlockquote>{children}</DocsBlockquote>
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
