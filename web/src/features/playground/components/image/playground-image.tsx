import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { NativeSelect } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'

import { generateImage } from '../../api'
import type {
  ImageGenerationResponse,
  ModelOption,
  GroupOption,
} from '../../types'
import { ImageResult } from './image-result'

interface ImageHistoryItem {
  prompt: string
  images: ImageGenerationResponse['data']
}

interface PlaygroundImageProps {
  models: ModelOption[]
  groups: GroupOption[]
  currentGroup: string
  onGroupChange: (group: string) => void
  isLoadingModels: boolean
}

const IMAGE_SIZES = ['1024x1024', '1024x1792', '1792x1024'] as const

function isImageModel(model: string): boolean {
  return model.startsWith('gpt-image') || model.startsWith('grok-imagine')
}

export function PlaygroundImage({
  models,
  groups,
  currentGroup,
  onGroupChange,
  isLoadingModels,
}: PlaygroundImageProps) {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [model, setModel] = useState('')
  const [size, setSize] = useState<string>('1024x1024')
  const [isGenerating, setIsGenerating] = useState(false)
  const [history, setHistory] = useState<ImageHistoryItem[]>([])

  const imageModels = models.filter((m) => isImageModel(m.value))

  // Auto-select first image model if none selected
  if (!model && imageModels.length > 0) {
    setModel(imageModels[0].value)
  }

  const handleGenerate = async () => {
    if (!prompt.trim() || !model) return
    setIsGenerating(true)
    try {
      const res = await generateImage({
        model,
        prompt: prompt.trim(),
        size,
        group: currentGroup || undefined,
      })
      setHistory((prev) => [
        { prompt: prompt.trim(), images: res.data },
        ...prev,
      ])
      setPrompt('')
    } catch (err) {
      const message =
        err instanceof Error ? err.message : t('Image generation failed')
      toast.error(message)
    } finally {
      setIsGenerating(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !isGenerating) {
      e.preventDefault()
      handleGenerate()
    }
  }

  return (
    <div className='flex size-full min-h-0 flex-col'>
      {/* Results area */}
      <div className='flex-1 overflow-y-auto p-4'>
        {history.length === 0 ? (
          <div className='flex size-full items-center justify-center'>
            <p className='text-muted-foreground text-sm'>
              {t('Generated images will appear here')}
            </p>
          </div>
        ) : (
          <div className='mx-auto grid max-w-4xl grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {history.map((item, idx) =>
              item.images.map((img, imgIdx) => (
                <ImageResult
                  key={`${idx}-${imgIdx}`}
                  url={img.url}
                  b64Json={img.b64_json}
                  revisedPrompt={img.revised_prompt}
                  prompt={item.prompt}
                />
              ))
            )}
          </div>
        )}
      </div>

      {/* Input area */}
      <div className='mx-auto w-full max-w-4xl border-t p-4'>
        <div className='flex flex-wrap items-center gap-2 pb-3'>
          <NativeSelect
            value={model}
            onChange={(e) => setModel(e.target.value)}
            disabled={isLoadingModels || imageModels.length === 0}
            size='sm'
          >
            {imageModels.length === 0 ? (
              <option value=''>{t('No image models available')}</option>
            ) : (
              imageModels.map((m) => (
                <option key={m.value} value={m.value}>
                  {m.label}
                </option>
              ))
            )}
          </NativeSelect>

          <NativeSelect
            value={size}
            onChange={(e) => setSize(e.target.value)}
            size='sm'
          >
            {IMAGE_SIZES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </NativeSelect>

          {groups.length > 1 && (
            <NativeSelect
              value={currentGroup}
              onChange={(e) => onGroupChange(e.target.value)}
              size='sm'
            >
              {groups.map((g) => (
                <option key={g.value} value={g.value}>
                  {g.label}
                </option>
              ))}
            </NativeSelect>
          )}
        </div>

        <div className='flex gap-2'>
          <Textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={t('Describe the image you want to generate...')}
            disabled={isGenerating}
            className='min-h-20 resize-none'
          />
          <Button
            onClick={handleGenerate}
            disabled={!prompt.trim() || !model || isGenerating}
            className='self-end'
          >
            {isGenerating ? <Spinner className='size-4' /> : t('Generate')}
          </Button>
        </div>
      </div>
    </div>
  )
}
