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
import { Check, Copy } from 'lucide-react'
import { type ReactNode, useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

interface DocsCodeBlockProps {
  code: string
  language?: string
  children?: ReactNode
}

/** 文档专用的轻量代码块:语言标签 + 复制按钮 + 纯文本 pre(不走 CodeMirror,
 * 避免一页多个 CodeMirror 实例的开销;语法着色留作后续增强)。 */
export function DocsCodeBlock(props: DocsCodeBlockProps) {
  const { t } = useTranslation()
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const [activeKey] = useState(() => props.code)
  const isCopied = copiedText === activeKey

  const onCopy = useCallback(() => {
    copyToClipboard(props.code)
  }, [copyToClipboard, props.code])

  return (
    <div className='group/codeblock bg-muted/40 my-4 overflow-hidden rounded-lg border'>
      <div className='text-muted-foreground flex items-center justify-between border-b px-3 py-1.5 text-xs'>
        <span className='font-mono'>{props.language || 'text'}</span>
        <button
          type='button'
          onClick={onCopy}
          className='hover:text-foreground flex items-center gap-1 rounded px-1.5 py-0.5 transition-colors'
          aria-label={isCopied ? t('Copied') : t('Copy to clipboard')}
        >
          {isCopied ? (
            <>
              <Check className='text-success size-3.5' />
              <span className='text-success'>{t('Copied')}</span>
            </>
          ) : (
            <>
              <Copy className='size-3.5' />
              <span>{t('Copy')}</span>
            </>
          )}
        </button>
      </div>
      <pre className='overflow-x-auto p-3 text-sm leading-relaxed'>
        <code className={cn('font-mono')}>{props.children}</code>
      </pre>
    </div>
  )
}
