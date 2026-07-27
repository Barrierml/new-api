import { useEffect, useRef } from 'react'
import { useTheme } from 'next-themes'
import QRCodeStyling from 'qr-code-styling'

interface StyledQRCodeProps {
  data: string
  size?: number
}

// Read a theme CSS variable and resolve it to an rgb() string.
// qr-code-styling renders via canvas/SVG pipelines that don't reliably accept
// oklch(), so we force a computed rgb() by reading `color` off a temp element.
function readThemeColor(varName: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback
  const el = document.createElement('div')
  el.style.color = `var(${varName})`
  el.style.display = 'none'
  document.body.appendChild(el)
  const color = getComputedStyle(el).color
  document.body.removeChild(el)
  return color || fallback
}

export function StyledQRCode({ data, size = 200 }: StyledQRCodeProps) {
  const ref = useRef<HTMLDivElement>(null)
  const { resolvedTheme } = useTheme()

  useEffect(() => {
    if (!ref.current) return

    const primary = readThemeColor('--primary', '#3b82f6')
    const ring = readThemeColor('--ring', primary)

    const qr = new QRCodeStyling({
      width: size,
      height: size,
      type: 'svg',
      data,
      dotsOptions: {
        type: 'classy-rounded',
        gradient: {
          type: 'linear',
          rotation: 45,
          colorStops: [
            { offset: 0, color: primary },
            { offset: 1, color: ring },
          ],
        },
      },
      cornersSquareOptions: {
        type: 'extra-rounded',
        color: ring,
      },
      cornersDotOptions: {
        type: 'dot',
        color: primary,
      },
      backgroundOptions: {
        color: 'transparent',
      },
    })

    ref.current.innerHTML = ''
    qr.append(ref.current)
  }, [data, size, resolvedTheme])

  return <div ref={ref} className='inline-flex items-center justify-center' />
}