import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ImageIcon, Sparkles, Upload, X } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { NativeSelect } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import { generateImage } from '../../api'
import type {
  ImageGenerationResponse,
  ModelOption,
  GroupOption,
} from '../../types'
import { ImageResult } from './image-result'

interface ImageHistoryItem {
  prompt: string
  model: string
  size: string
  referenceImage?: string
  images: ImageGenerationResponse['data']
  timestamp: number
}

interface PlaygroundImageProps {
  models: ModelOption[]
  groups: GroupOption[]
  currentGroup: string
  onGroupChange: (group: string) => void
  isLoadingModels: boolean
}

const IMAGE_SIZES = [
  { value: '1024x1024', label: '1:1 (1024×1024)' },
  { value: '1024x1792', label: '9:16 (1024×1792)' },
  { value: '1792x1024', label: '16:9 (1792×1024)' },
] as const

function isImageModel(model: string): boolean {
  return model.startsWith('gpt-image') || model.startsWith('grok-imagine')
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result as string
      resolve(result)
    }
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
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
  const [referenceImage, setReferenceImage] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const imageModels = models.filter((m) => isImageModel(m.value))

  if (!model && imageModels.length > 0) {
    setModel(imageModels[0].value)
  }

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error(t('Please select an image file'))
      return
    }
    if (file.size > 20 * 1024 * 1024) {
      toast.error(t('Image must be less than 20MB'))
      return
    }
    const base64 = await fileToBase64(file)
    setReferenceImage(base64)
    if (fileInputRef.current) fileInputRef.current.value = ''
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
        image: referenceImage || undefined,
      })
      setHistory((prev) => [
        {
          prompt: prompt.trim(),
          model,
          size,
          referenceImage: referenceImage || undefined,
          images: res.data,
          timestamp: Date.now(),
        },
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
      <div className='flex-1 overflow-y-auto'>
        {history.length === 0 ? (
          <EmptyState />
        ) : (
          <div className='mx-auto max-w-5xl space-y-6 p-6'>
            {history.map((item, idx) => (
              <div key={idx} className='space-y-3'>
                <div className='flex items-center gap-2'>
                  <span className='bg-muted text-muted-foreground rounded px-2 py-0.5 text-[11px] font-medium'>
                    {item.model}
                  </span>
                  <span className='bg-muted text-muted-foreground rounded px-2 py-0.5 text-[11px]'>
                    {item.size}
                  </span>
                  {item.referenceImage && (
                    <span className='bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded px-2 py-0.5 text-[11px]'>
                      {t('img2img')}
                    </span>
                  )}
                </div>
                <div
                  className={cn(
                    'grid gap-4',
                    item.images.length === 1
                      ? 'grid-cols-1 max-w-lg'
                      : 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3'
                  )}
                >
                  {item.images.map((img, imgIdx) => (
                    <ImageResult
                      key={`${idx}-${imgIdx}`}
                      url={img.url}
                      b64Json={img.b64_json}
                      revisedPrompt={img.revised_prompt}
                      prompt={item.prompt}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Input panel */}
      <div className='border-t bg-gradient-to-t from-background to-background/80 backdrop-blur-sm'>
        <div className='mx-auto max-w-4xl p-4'>
          <div className='bg-card rounded-xl border shadow-sm'>
            <div className='flex flex-wrap items-center gap-2 border-b px-4 py-2.5'>
              <NativeSelect
                value={model}
                onChange={(e) => setModel(e.target.value)}
                disabled={isLoadingModels || imageModels.length === 0}
                size='sm'
                className='text-xs'
              >
                {imageModels.length === 0 ? (
                  <option value=''>{t('No image models')}</option>
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
                className='text-xs'
              >
                {IMAGE_SIZES.map((s) => (
                  <option key={s.value} value={s.value}>
                    {s.label}
                  </option>
                ))}
              </NativeSelect>

              {groups.length > 1 && (
                <NativeSelect
                  value={currentGroup}
                  onChange={(e) => onGroupChange(e.target.value)}
                  size='sm'
                  className='text-xs'
                >
                  {groups.map((g) => (
                    <option key={g.value} value={g.value}>
                      {g.label}
                    </option>
                  ))}
                </NativeSelect>
              )}

              <Button
                variant='ghost'
                size='sm'
                className='ml-auto text-xs'
                onClick={() => fileInputRef.current?.click()}
                disabled={isGenerating}
              >
                <Upload className='mr-1.5 size-3.5' />
                {t('Reference Image')}
              </Button>
              <input
                ref={fileInputRef}
                type='file'
                accept='image/*'
                className='hidden'
                onChange={handleFileSelect}
              />
            </div>

            {referenceImage && (
              <div className='flex items-center gap-3 border-b px-4 py-2.5'>
                <div className='relative size-14 shrink-0 overflow-hidden rounded-lg border'>
                  <img
                    src={referenceImage}
                    alt='Reference'
                    className='size-full object-cover'
                  />
                  <button
                    onClick={() => setReferenceImage(null)}
                    className='absolute -right-0.5 -top-0.5 flex size-5 items-center justify-center rounded-full bg-black/70 text-white transition-colors hover:bg-black/90'
                  >
                    <X className='size-3' />
                  </button>
                </div>
                <span className='text-muted-foreground text-xs'>
                  {t('Image will be used as reference for generation (img2img)')}
                </span>
              </div>
            )}

            <div className='flex items-end gap-3 p-4'>
              <Textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={
                  referenceImage
                    ? t('Describe how to modify the reference image...')
                    : t('Describe the image you want to generate...')
                }
                disabled={isGenerating}
                className='min-h-[4.5rem] resize-none border-0 px-3 py-2 shadow-none focus-visible:ring-0'
                rows={3}
              />
              <Button
                onClick={handleGenerate}
                disabled={!prompt.trim() || !model || isGenerating}
                size='sm'
                className='shrink-0'
              >
                {isGenerating ? (
                  <Spinner className='size-4' />
                ) : (
                  <Sparkles className='size-4' />
                )}
                <span className='ml-1.5'>
                  {isGenerating ? t('Generating...') : t('Generate')}
                </span>
              </Button>
            </div>
            <div className='text-muted-foreground border-t px-4 py-2 text-[11px]'>
              {t('Press ⌘+Enter to generate')}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function EmptyState() {
  const { t } = useTranslation()
  return (
    <div className='flex size-full flex-col items-center justify-center gap-4 p-8'>
      <div className='bg-muted/50 flex size-16 items-center justify-center rounded-2xl'>
        <ImageIcon className='text-muted-foreground size-8' />
      </div>
      <div className='text-center'>
        <p className='text-foreground text-sm font-medium'>
          {t('Image Generation')}
        </p>
        <p className='text-muted-foreground mt-1 max-w-sm text-xs'>
          {t('Describe what you want to create, or upload a reference image for img2img editing.')}
        </p>
      </div>
    </div>
  )
}
