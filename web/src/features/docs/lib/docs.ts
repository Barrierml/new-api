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
import { DOC_ENTRIES, type DocEntry, type DocGroup } from '../docs.generated'

export { DOC_ENTRIES } from '../docs.generated'
export type { DocEntry, DocGroup } from '../docs.generated'

const ENTRIES: DocEntry[] = DOC_ENTRIES

/** 文档总数(供 UI 判空)。 */
export function listDocs(): DocEntry[] {
  return ENTRIES
}

/** 按 URL slug 找文档;"" 命中首页(README)。 */
export function findDoc(slug: string): DocEntry | undefined {
  const cleaned = slug.replaceAll(/^\/+|\/+$/g, '')
  return ENTRIES.find((d) => d.slug === cleaned)
}

/**
 * 把 markdown 里的 {{BASE_URL}} / {{BRAND}} 占位替换成当前站点的真实值,
 * 这样同一份文档在 dev / prod / 自部署下都自动正确。
 */
export function injectTokens(
  content: string,
  opts: { baseUrl: string; brand: string },
): string {
  return content
    .replaceAll('{{BASE_URL}}', opts.baseUrl)
    .replaceAll('{{BRAND}}', opts.brand)
}

// ── 分组元数据 ───────────────────────────────────────────────────────────────

const GROUP_ORDER: Record<DocGroup, number> = {
  index: 0,
  overview: 1,
  cli: 2,
  api: 3,
  integrations: 4,
}

export function groupOrder(group: DocGroup): number {
  return GROUP_ORDER[group]
}

export const GROUP_LABEL_KEY: Record<DocGroup, string> = {
  index: 'docs.group.index',
  overview: 'docs.group.overview',
  cli: 'docs.group.cli',
  api: 'docs.group.api',
  integrations: 'docs.group.integrations',
}

// ── Markdown 链接解析(把相对 .md 链接映射到 /docs 路由)─────────────────────────

export function resolveMdLink(
  href: string,
  currentSlug: string,
): { internal: boolean; href: string } {
  if (!href) return { internal: false, href }
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return { internal: false, href } // http/mailto
  if (href.startsWith('#')) return { internal: false, href } // 纯锚点
  if (!/\.md(#|$)/i.test(href)) return { internal: false, href } // 非 .md 链接

  const hashIdx = href.indexOf('#')
  const pathPart = hashIdx >= 0 ? href.slice(0, hashIdx) : href
  const anchor = hashIdx >= 0 ? href.slice(hashIdx) : ''

  const currentDir = currentSlug.includes('/')
    ? currentSlug.slice(0, currentSlug.lastIndexOf('/'))
    : ''

  let p = pathPart.replace(/\.md$/i, '')
  const parts = currentDir ? currentDir.split('/').filter(Boolean) : []
  if (p.startsWith('./')) p = p.slice(2)
  while (p.startsWith('../')) {
    parts.pop()
    p = p.slice(3)
  }
  const resolved = parts.length ? `${parts.join('/')}/${p}` : p

  if (resolved === '' || resolved.toUpperCase() === 'README') {
    return { internal: true, href: `/docs${anchor}` }
  }
  return { internal: true, href: `/docs/${resolved}${anchor}` }
}

// ── Anchor / 标题 id(右侧 TOC 用)─────────────────────────────────────────────

/** GitHub 风格 slug,兼容 CJK。 */
export function slugify(text: string): string {
  return text
    .toLowerCase()
    .replaceAll(/[\s ]+/g, '-')
    .replaceAll(/[^\w一-龥-]/g, '')
    .replaceAll(/^-+|-+$/g, '')
    .replaceAll(/-{2,}/g, '-')
}

export function normalizeHeadingText(text: string): string {
  return text
    .replace(/^\d+\s*[—-]\s*/, '')
    .replaceAll('`', '')
    .trim()
}

export function makeUniqueHeadingId(
  text: string,
  seen: Map<string, number>,
): string {
  const base = slugify(normalizeHeadingText(text)) || 'section'
  const count = seen.get(base) ?? 0
  seen.set(base, count + 1)
  return count === 0 ? base : `${base}-${count}`
}

export interface AnchorEntry {
  level: 2 | 3
  text: string
  id: string
}

/** 从 markdown 解析 H2/H3 标题(跳过围栏代码块),供右侧 TOC。 */
export function parseAnchors(content: string): AnchorEntry[] {
  const out: AnchorEntry[] = []
  const seen = new Map<string, number>()
  let inCode = false
  for (const line of content.split('\n')) {
    if (line.startsWith('```')) {
      inCode = !inCode
      continue
    }
    if (inCode) continue
    const m = line.match(/^(#{2,3})\s+(.+?)\s*$/)
    if (!m) continue
    const level = m[1].length as 2 | 3
    const text = normalizeHeadingText(m[2])
    out.push({ level, text, id: makeUniqueHeadingId(text, seen) })
  }
  return out
}

/** 遍历 react-markdown 子节点提取纯文本(算 heading id / 代码块文本用)。 */
export function extractTextFromChildren(children: unknown): string {
  if (typeof children === 'string') return children
  if (typeof children === 'number') return String(children)
  if (!children) return ''
  if (Array.isArray(children)) {
    return children.map(extractTextFromChildren).join('')
  }
  if (typeof children === 'object' && 'props' in (children as object)) {
    return extractTextFromChildren(
      (children as { props: { children?: unknown } }).props.children,
    )
  }
  return ''
}

/** 从 react-markdown code 子节点的 className 提取语言("language-bash" → "bash")。 */
export function extractCodeLanguage(children: unknown): string {
  if (children && typeof children === 'object' && 'props' in (children as object)) {
    const cls =
      (children as { props?: { className?: string } }).props?.className || ''
    const m = cls.match(/language-(\w+)/)
    if (m) return m[1]
  }
  return ''
}
