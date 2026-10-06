import fonts from './clip-ink-fonts.json'
import catalog from './clip-caption-ink.json'
import identity from './clip-ink-identity.json'

/** Qualified same-face ink resources; this is independent of final-render activation. */
export const CLIP_INK = {
  version: 'clip-ink-v2-resvg-2.6.2',
  transformInkScale: 2,
  transformInkStyles: ['word-pop', 'pop', 'stack', 'sticker', 'bubble'] as readonly string[],
  wasmVersion: '2.6.2',
  wasmSHA256: '22bf6e9f9a100d972da0411a69c5ba504367fc1fa87b3b64e3f35e53926d2d70',
  cacheBytes: 64 * 1024 * 1024,
  cacheEntries: 64,
  measurements: 1024,
  maxSurfacePixels: 1080 * 1920,
  maxFontBytes: 20 * 1024 * 1024,
  maxTextRunes: 1000,
  fontBaseURL: '/fonts/clip/',
} as const

export const CLIP_INK_FONT_DATA = fonts
export const CLIP_INK_IDENTITY = identity
export const CLIP_CAPTION_INK = catalog.styles
export const CLIP_CAPTION_TRANSFORM_PAINT = catalog.transformPaint
export const CLIP_CAPTION_TRANSFORM_STYLES = [
  'word-pop',
  'pop',
  'stack',
  'sticker',
  'bubble',
  'serif',
  'outline',
] as const
export type ClipInkFont = (typeof fonts.resources)[number]
export type ClipInkFace = 'wantedsans' | 'paperlogy' | 'jua' | 'nanummyeongjo'
export type ClipCaptionInkStyle = (typeof catalog.styles)[keyof typeof catalog.styles]

export function clipInkFont(face: string, weight: number): ClipInkFont {
  const font = fonts.resources.find((f) => f.face === face && f.weight === weight)
  if (!font) throw new Error(`CLIP_INK_UNSUPPORTED_FONT:${face}:${weight}`)
  return font
}

export function clipInkCoverage(face: string, weight: number, character: string): boolean {
  const font = clipInkFont(face, weight)
  const source = fonts.sources[font.source as keyof typeof fonts.sources]
  const code = character.codePointAt(0)
  if (code === undefined) return false
  let low = 0
  let high = source.coverage.length - 1
  while (low <= high) {
    const middle = (low + high) >>> 1
    const range = source.coverage[middle]!
    if (code < range[0]!) high = middle - 1
    else if (code > range[1]!) low = middle + 1
    else return true
  }
  return false
}

/** Exact native hhea metrics, not the tight raster ink bounds. */
export function clipInkMetrics(face: string, weight: number) {
  const font = clipInkFont(face, weight)
  return fonts.sources[font.source as keyof typeof fonts.sources]
}
