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
import { AppWindow, ArrowUpRight, Check, Copy, Terminal } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconGithub } from '@/assets/brand-icons'
import { AnimateInView } from '@/components/animate-in-view'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

const INSTALL_CMD = 'npm install -g tako-cli'

function InstallCommand() {
  const { t } = useTranslation()
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const copied = copiedText === INSTALL_CMD

  return (
    <div className='flex items-center gap-2 rounded-lg border border-zinc-800 bg-zinc-950 py-2.5 pl-4 pr-2.5'>
      <span className='select-none font-mono text-[13px] text-zinc-500'>$</span>
      <code className='flex-1 overflow-x-auto whitespace-nowrap font-mono text-[13px] text-zinc-200'>
        {INSTALL_CMD}
      </code>
      <button
        type='button'
        onClick={() => copyToClipboard(INSTALL_CMD)}
        className='shrink-0 rounded p-1.5 text-zinc-500 transition-colors hover:text-zinc-200'
        aria-label={copied ? t('Copied') : t('Copy to clipboard')}
      >
        {copied ? (
          <Check className='size-3.5 text-emerald-400' />
        ) : (
          <Copy className='size-3.5' />
        )}
      </button>
    </div>
  )
}

function RepoLink({ href, repo }: { href: string; repo: string }) {
  const { t } = useTranslation()
  return (
    <a
      href={href}
      target='_blank'
      rel='noopener noreferrer'
      className='group/link text-muted-foreground hover:text-foreground mt-auto inline-flex items-center gap-1.5 pt-5 text-sm font-medium transition-colors'
    >
      <IconGithub className='size-4' />
      <span className='font-mono text-[13px]'>{repo}</span>
      <span className='text-xs'>{t('Star on GitHub')}</span>
      <ArrowUpRight className='size-3.5 transition-transform group-hover/link:translate-x-0.5 group-hover/link:-translate-y-0.5' />
    </a>
  )
}

export function OpenSource() {
  const { t } = useTranslation()

  return (
    <section className='border-border/40 relative z-10 border-t px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-16 max-w-xl'>
          <p className='text-muted-foreground mb-3 text-xs font-medium uppercase tracking-widest'>
            {t('Open Source')}
          </p>
          <h2 className='text-2xl font-bold leading-tight tracking-tight md:text-3xl'>
            {t('Official tools, built in the open')}
          </h2>
          <p className='text-muted-foreground mt-4 text-sm leading-relaxed md:text-[15px]'>
            {t(
              'Our launcher and desktop app are open source — star them, fork them, or just install and start coding.'
            )}
          </p>
        </AnimateInView>

        <div className='grid gap-6 md:grid-cols-2'>
          {/* tako-cli */}
          <AnimateInView delay={0} animation='fade-up'>
            <div className='group border-border/40 bg-muted/15 hover:border-border flex h-full flex-col rounded-xl border p-7 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-lg md:p-8'>
              <div className='mb-5 flex items-center gap-3'>
                <div className='flex size-11 items-center justify-center rounded-xl border border-orange-500/20 bg-orange-500/10 text-orange-600 dark:text-orange-400'>
                  <Terminal className='size-5' strokeWidth={1.75} />
                </div>
                <div>
                  <h3 className='font-semibold'>tako-cli</h3>
                  <p className='text-muted-foreground text-xs'>
                    {t('Terminal launcher · npm')}
                  </p>
                </div>
              </div>
              <p className='text-muted-foreground mb-6 text-sm leading-relaxed'>
                {t(
                  'One command to launch Claude Code, Codex or Gemini CLI with gateway auth pre-configured — no env vars to manage.'
                )}
              </p>
              <InstallCommand />
              <RepoLink href='https://github.com/tako-dev/cli' repo='tako-dev/cli' />
            </div>
          </AnimateInView>

          {/* Tako Switch */}
          <AnimateInView delay={120} animation='fade-up'>
            <div className='group border-border/40 bg-muted/15 hover:border-border flex h-full flex-col rounded-xl border p-7 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-lg md:p-8'>
              <div className='mb-5 flex items-center gap-3'>
                <div className='flex size-11 items-center justify-center rounded-xl border border-orange-500/20 bg-orange-500/10 text-orange-600 dark:text-orange-400'>
                  <AppWindow className='size-5' strokeWidth={1.75} />
                </div>
                <div>
                  <h3 className='font-semibold'>Tako Switch</h3>
                  <p className='text-muted-foreground text-xs'>
                    {t('Desktop app · Tauri 2 · MIT')}
                  </p>
                </div>
              </div>
              <p className='text-muted-foreground mb-6 text-sm leading-relaxed'>
                {t(
                  'One desktop app to manage and launch Claude Code, Codex, Gemini CLI and more — with the Tako provider built in, ready out of the box.'
                )}
              </p>
              <div className='flex flex-wrap gap-2'>
                {['Windows', 'macOS', 'Linux'].map((os) => (
                  <span
                    key={os}
                    className='border-border/50 bg-muted/40 text-muted-foreground rounded-full border px-3 py-1 text-xs font-medium'
                  >
                    {os}
                  </span>
                ))}
              </div>
              <RepoLink
                href='https://github.com/Barrierml/tako-switch'
                repo='Barrierml/tako-switch'
              />
            </div>
          </AnimateInView>
        </div>
      </div>
    </section>
  )
}
