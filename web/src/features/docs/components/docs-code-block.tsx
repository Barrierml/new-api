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
import bash from '@shikijs/langs/bash'
import go from '@shikijs/langs/go'
import json from '@shikijs/langs/json'
import markdown from '@shikijs/langs/markdown'
import python from '@shikijs/langs/python'
import toml from '@shikijs/langs/toml'
import tsx from '@shikijs/langs/tsx'
import typescript from '@shikijs/langs/typescript'
import yaml from '@shikijs/langs/yaml'
import githubDark from '@shikijs/themes/github-dark'
import { Check, Copy } from 'lucide-react'
import { type ReactNode, useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { createHighlighterCore, type HighlighterCore } from 'shiki/core'
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

// 模块级 shiki 单例:js regex 引擎(无 WASM 异步加载),只注册文档会用的语言,
// 避免 bundle/web 全量语言的开销。首次使用时才创建。
let highlighterPromise: Promise<HighlighterCore> | null = null

function getHighlighter(): Promise<HighlighterCore> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighterCore({
      themes: [githubDark],
      langs: [bash, json, python, typescript, tsx, go, yaml, toml, markdown],
      engine: createJavaScriptRegexEngine(),
    })
  }
  return highlighterPromise
}

/** markdown fence 语言别名 → shiki 语言 id。 */
const LANG_ALIASES: Record<string, string> = {
  sh: 'bash',
  shell: 'bash',
  zsh: 'bash',
  console: 'bash',
  curl: 'bash',
  js: 'typescript',
  jsx: 'tsx',
  ts: 'typescript',
  yml: 'yaml',
  md: 'markdown',
  golang: 'go',
  py: 'python',
  jsonc: 'json',
  json5: 'json',
}

const SUPPORTED = new Set([
  'bash',
  'json',
  'python',
  'typescript',
  'tsx',
  'go',
  'yaml',
  'toml',
  'markdown',
])

function resolveLanguage(language?: string): string | null {
  const raw = (language ?? '').trim().toLowerCase()
  if (!raw || raw === 'text' || raw === 'plaintext' || raw === 'txt') return null
  const mapped = LANG_ALIASES[raw] ?? raw
  return SUPPORTED.has(mapped) ? mapped : null
}

interface DocsCodeBlockProps {
  code: string
  language?: string
  children?: ReactNode
}

/** 文档代码块:macOS 窗口栏 + shiki 高亮(常驻深色,Mintlify 风格)。
 * 高亮异步完成前回退纯文本 pre;语言不支持时同样回退。 */
export function DocsCodeBlock(props: DocsCodeBlockProps) {
  const { t } = useTranslation()
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const [activeKey] = useState(() => props.code)
  const isCopied = copiedText === activeKey
  const [html, setHtml] = useState<string | null>(null)

  const lang = resolveLanguage(props.language)

  useEffect(() => {
    if (!lang) return
    let cancelled = false
    getHighlighter()
      .then((h) => h.codeToHtml(props.code, { lang, theme: 'github-dark' }))
      .then((out) => {
        if (!cancelled) setHtml(out)
      })
      .catch(() => {
        // 高亮失败保持纯文本回退,不影响阅读
      })
    return () => {
      cancelled = true
    }
  }, [props.code, lang])

  const onCopy = useCallback(() => {
    copyToClipboard(props.code)
  }, [copyToClipboard, props.code])

  return (
    <div className='group/codeblock my-5 overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 shadow-sm'>
      <div className='flex items-center gap-2 border-b border-zinc-800 bg-zinc-900/60 px-4 py-2'>
        {/* macOS 窗口圆点 */}
        <span className='flex gap-1.5'>
          <span className='size-2.5 rounded-full bg-zinc-700' />
          <span className='size-2.5 rounded-full bg-zinc-700' />
          <span className='size-2.5 rounded-full bg-zinc-700' />
        </span>
        <span className='font-mono text-xs text-zinc-500'>
          {props.language || 'text'}
        </span>
        <button
          type='button'
          onClick={onCopy}
          className='ml-auto flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-zinc-500 transition-colors hover:text-zinc-200'
          aria-label={isCopied ? t('Copied') : t('Copy to clipboard')}
        >
          {isCopied ? (
            <>
              <Check className='size-3.5 text-emerald-400' />
              <span className='text-emerald-400'>{t('Copied')}</span>
            </>
          ) : (
            <>
              <Copy className='size-3.5' />
              <span className='opacity-0 transition-opacity group-hover/codeblock:opacity-100'>
                {t('Copy')}
              </span>
            </>
          )}
        </button>
      </div>
      {html ? (
        <div
          className={cn(
            'overflow-x-auto text-[13px] leading-6',
            // 清掉 shiki pre 自带的背景/边距,让容器接管圆角和底色
            '[&_pre]:!bg-transparent [&_pre]:p-4 [&_pre]:font-mono',
          )}
          // shiki 输出是可信的静态 HTML(本地 codegen 内容 + 转义)
          dangerouslySetInnerHTML={{ __html: html }}
        />
      ) : (
        <pre className='overflow-x-auto p-4 font-mono text-[13px] leading-6 text-zinc-300'>
          <code>{props.code}</code>
        </pre>
      )}
    </div>
  )
}
