import { useState } from 'react'
import { Download, Maximize2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface ImageResultProps {
  url?: string
  b64Json?: string
  revisedPrompt?: string
  prompt: string
}

export function ImageResult({
  url,
  b64Json,
  revisedPrompt,
  prompt,
}: ImageResultProps) {
  const { t } = useTranslation()
  const [isLoaded, setIsLoaded] = useState(false)
  const [showFullscreen, setShowFullscreen] = useState(false)
  const imageSrc = url || (b64Json ? `data:image/png;base64,${b64Json}` : '')

  const handleDownload = () => {
    if (!imageSrc) return
    const link = document.createElement('a')
    link.href = imageSrc
    link.download = `generated-${Date.now()}.png`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  if (!imageSrc) return null

  return (
    <>
      <div className='bg-card group relative overflow-hidden rounded-xl border transition-shadow hover:shadow-md'>
        <div className='relative aspect-square bg-neutral-100 dark:bg-neutral-900'>
          {!isLoaded && (
            <div className='absolute inset-0 flex items-center justify-center'>
              <div className='bg-muted size-8 animate-pulse rounded-full' />
            </div>
          )}
          <img
            src={imageSrc}
            alt={revisedPrompt || prompt}
            className={cn(
              'size-full object-cover transition-opacity duration-300',
              isLoaded ? 'opacity-100' : 'opacity-0'
            )}
            onLoad={() => setIsLoaded(true)}
          />
          <div className='absolute inset-0 flex items-center justify-center gap-2 bg-black/0 opacity-0 transition-all group-hover:bg-black/40 group-hover:opacity-100'>
            <Button
              variant='secondary'
              size='icon-sm'
              onClick={() => setShowFullscreen(true)}
              className='size-9 rounded-full shadow-lg'
              aria-label={t('View full size')}
            >
              <Maximize2 className='size-4' />
            </Button>
            <Button
              variant='secondary'
              size='icon-sm'
              onClick={handleDownload}
              className='size-9 rounded-full shadow-lg'
              aria-label={t('Download')}
            >
              <Download className='size-4' />
            </Button>
          </div>
        </div>
        <div className='px-3 py-2.5'>
          <p className='text-muted-foreground line-clamp-2 text-xs leading-relaxed'>
            {revisedPrompt || prompt}
          </p>
        </div>
      </div>

      {showFullscreen && (
        <div
          className='fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm'
          onClick={() => setShowFullscreen(false)}
        >
          <img
            src={imageSrc}
            alt={revisedPrompt || prompt}
            className='max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl'
          />
        </div>
      )}
    </>
  )
}
