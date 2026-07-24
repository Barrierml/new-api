import { Download01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

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
  const imageSrc = url || (b64Json ? `data:image/png;base64,${b64Json}` : '')

  const handleDownload = () => {
    if (!imageSrc) return
    const link = document.createElement('a')
    link.href = imageSrc
    link.download = `image-${Date.now()}.png`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  if (!imageSrc) return null

  return (
    <div className='bg-card overflow-hidden rounded-lg border'>
      <div className='relative aspect-square'>
        <img
          src={imageSrc}
          alt={revisedPrompt || prompt}
          className='size-full object-contain'
        />
      </div>
      <div className='flex items-center justify-between gap-2 border-t p-3'>
        <p className='text-muted-foreground line-clamp-2 text-xs'>
          {revisedPrompt || prompt}
        </p>
        <Button
          variant='ghost'
          size='icon-sm'
          onClick={handleDownload}
          aria-label={t('Download')}
        >
          <HugeiconsIcon icon={Download01Icon} className='size-4' />
        </Button>
      </div>
    </div>
  )
}
