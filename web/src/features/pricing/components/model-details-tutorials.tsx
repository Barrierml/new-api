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
import { ArrowUpRight, Sparkles } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useStatus } from '@/hooks/use-status'

import type { PricingModel } from '../types'

type ToolKey = 'claude-code' | 'codex' | 'cursor' | 'gemini-cli'

const TOOL_ORDER: ToolKey[] = ['claude-code', 'codex', 'cursor', 'gemini-cli']

// 每个工具适用的 endpoint type(来自模型 supported_endpoint_types)。
const TOOL_ENDPOINT_TYPE: Record<ToolKey, string> = {
  'claude-code': 'anthropic',
  codex: 'openai-response',
  cursor: 'openai',
  'gemini-cli': 'gemini',
}

interface ModelDetailsTutorialsProps {
  model: PricingModel
}

export function ModelDetailsTutorials(props: ModelDetailsTutorialsProps) {
  const { t } = useTranslation()
  const { status } = useStatus()

  const baseUrl = useMemo(() => {
    const s = status as Record<string, unknown> | null
    const candidate =
      s?.server_address ??
      s?.serverAddress ??
      (s?.data as Record<string, unknown> | undefined)?.server_address
    if (candidate && typeof candidate === 'string') {
      return candidate.replace(/\/$/, '')
    }
    if (typeof window !== 'undefined') return window.location.origin
    return 'https://api.example.com'
  }, [status])

  const tools = useMemo(
    () => {
      const types = props.model.supported_endpoint_types ?? []
      return TOOL_ORDER.filter((tk) => types.includes(TOOL_ENDPOINT_TYPE[tk]))
    },
    [props.model],
  )

  const [tool, setTool] = useState<ToolKey>(tools[0] ?? 'claude-code')

  if (tools.length === 0) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t(
          'This model has no direct tool integration. See the API tab for raw request examples.',
        )}
      </p>
    )
  }

  const modelName = props.model.model_name || ''
  const activeTool = tools.includes(tool) ? tool : tools[0]

  return (
    <section className='space-y-4'>
      <TakoCliCallout tools={tools} />

      <Tabs value={activeTool} onValueChange={(v) => setTool(v as ToolKey)}>
        <TabsList className='bg-muted/40 h-8 p-0.5'>
          {tools.map((tk) => (
            <TabsTrigger
              key={tk}
              value={tk}
              className='h-7 px-2.5 text-xs'
            >
              {TOOL_LABEL[tk]}
            </TabsTrigger>
          ))}
        </TabsList>

        {tools.map((tk) => (
          <TabsContent key={tk} value={tk} className='mt-3 space-y-2'>
            <p className='text-muted-foreground text-xs'>
              {TOOL_DESC(tk, t)}
            </p>
            <CodeBlock
              code={buildToolSnippet(tk, baseUrl, modelName)}
              language={TOOL_LANG[tk]}
            >
              <CodeBlockCopyButton />
            </CodeBlock>
          </TabsContent>
        ))}
      </Tabs>

      <p className='text-muted-foreground mt-2 text-xs'>
        {t('Replace')}{' '}
        <code className='bg-muted rounded px-1 py-0.5 font-mono text-[11px]'>
          {'<YOUR_API_KEY>'}
        </code>{' '}
        {t('with the API key from your token settings.')}
      </p>
    </section>
  )
}

const TOOL_LABEL: Record<ToolKey, string> = {
  'claude-code': 'Claude Code',
  codex: 'Codex',
  cursor: 'Cursor · Cline',
  'gemini-cli': 'Gemini CLI',
}

const TOOL_LANG: Record<ToolKey, string> = {
  'claude-code': 'bash',
  codex: 'toml',
  cursor: 'bash',
  'gemini-cli': 'bash',
}

const TAKO_LAUNCH_CMD: Record<ToolKey, string> = {
  'claude-code': 'tako --claude',
  codex: 'tako --codex',
  cursor: 'tako --claude',
  'gemini-cli': 'tako --gemini',
}

function TOOL_DESC(tk: ToolKey, t: (k: string) => string): string {
  switch (tk) {
    case 'claude-code':
      return t('Point Claude Code at this gateway via ANTHROPIC_BASE_URL.')
    case 'codex':
      return t('Configure OpenAI Codex via ~/.codex/config.toml (Responses endpoint).')
    case 'cursor':
      return t('Any OpenAI-compatible client (Cursor / Cline / Continue) using /v1.')
    case 'gemini-cli':
      return t('Point Gemini CLI at this gateway via GOOGLE_GEMINI_BASE_URL.')
  }
}

function TakoCliCallout({ tools }: { tools: ToolKey[] }) {
  const { t } = useTranslation()
  const example = TAKO_LAUNCH_CMD[tools[0]] ?? 'tako --claude'
  return (
    <div className='bg-primary/5 flex items-start gap-2.5 rounded-lg border border-primary/20 p-3 text-sm'>
      <Sparkles className='text-primary mt-0.5 size-4 shrink-0' />
      <div className='min-w-0'>
        <span className='font-medium'>{t('Recommended: launch via tako-cli')}</span>
        <p className='text-muted-foreground mt-0.5 text-xs'>
          <code className='bg-muted rounded px-1 py-0.5 font-mono text-[11px]'>
            {example}
          </code>{' '}
          {t('auto-configures the env vars for you.')}
        </p>
      </div>
      <Link
        to='/docs/$'
        params={{ _splat: 'cli/00-quickstart' }}
        className='text-primary hover:text-primary/80 ml-auto inline-flex shrink-0 items-center gap-0.5 text-xs font-medium'
      >
        {t('Guide')}
        <ArrowUpRight className='size-3.5' />
      </Link>
    </div>
  )
}

function buildToolSnippet(
  tool: ToolKey,
  baseUrl: string,
  model: string,
): string {
  switch (tool) {
    case 'claude-code':
      return [
        `# Claude Code 会自动在 base url 后拼 /v1/messages`,
        `export ANTHROPIC_BASE_URL="${baseUrl}"`,
        `export ANTHROPIC_AUTH_TOKEN="<YOUR_API_KEY>"`,
        `export ANTHROPIC_MODEL="${model}"`,
        ``,
        `# 写到 ~/.zshrc 持久化,然后: claude "hi"`,
      ].join('\n')
    case 'codex':
      return [
        `# ~/.codex/config.toml`,
        `model_provider = "tako"`,
        `model = "${model}"`,
        ``,
        `[model_providers.tako]`,
        `name = "tako"`,
        `base_url = "${baseUrl}/v1"`,
        `requires_openai_auth = true`,
        `experimental_bearer_token = "<YOUR_API_KEY>"`,
      ].join('\n')
    case 'cursor':
      return [
        `# 任意 OpenAI 兼容客户端(Cursor / Cline / Continue / Open WebUI ...)`,
        `# Base URL: ${baseUrl}/v1`,
        `# API Key:  <YOUR_API_KEY>`,
        `# Model:    ${model}`,
        ``,
        `curl "${baseUrl}/v1/chat/completions" \\`,
        `  -H "Authorization: Bearer <YOUR_API_KEY>" \\`,
        `  -H "Content-Type: application/json" \\`,
        `  -d '{"model":"${model}","messages":[{"role":"user","content":"hi"}]}'`,
      ].join('\n')
    case 'gemini-cli':
      return [
        `# GOOGLE_GEMINI_BASE_URL 给根域名,Gemini CLI 会自动拼 /v1beta/...`,
        `export GEMINI_API_KEY="<YOUR_API_KEY>"`,
        `export GOOGLE_GEMINI_BASE_URL="${baseUrl}"`,
        ``,
        `# 验证: gemini --model ${model} "hi"`,
      ].join('\n')
  }
}
