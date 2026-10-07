import {
  CLIP_CAPTION_INK,
  CLIP_INK,
  clipInkCoverage,
  clipInkFont,
  type ClipCaptionInkStyle,
} from '@/entities/clip-design/@x/clip-preview'

export interface InkBox {
  x: number
  y: number
  width: number
  height: number
}
export interface InkRole {
  face: string
  weight: number
  tracking: number
  size: number
}
export interface InkRun {
  text: string
  face: string
  substituted: boolean
  fill?: string
}
export class ClipInkError extends Error {
  constructor(
    public readonly code: string,
    public readonly subject = '',
  ) {
    super(subject ? `${code}:${subject}` : code)
  }
}
/** Physical pixels are reserved before raster work; only versioned scales are admitted. */
export function inkRasterDimensions(bounds: InkBox, scale = 1) {
  if (
    ![bounds.x, bounds.y, bounds.width, bounds.height].every(Number.isFinite) ||
    bounds.width <= 0 ||
    bounds.height <= 0
  )
    throw new ClipInkError('CLIP_INK_INVALID_GEOMETRY')
  if (scale !== 1 && scale !== CLIP_INK.transformInkScale)
    throw new ClipInkError('CLIP_INK_RESOURCE_LIMIT', 'raster scale')
  const width = Math.ceil(bounds.width),
    height = Math.ceil(bounds.height)
  if (width * height > CLIP_INK.maxSurfacePixels) throw new ClipInkError('CLIP_INK_RESOURCE_LIMIT')
  return { width: width * scale, height: height * scale }
}
export const inkNumber = (value: number) => value.toFixed(3)
export const inkEscape = (value: string) =>
  value.replace(
    /[&<>"']/gu,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;' })[c]!,
  )

/** Mirrors native coverageExempt; controls are still rejected by the copy validator. */
const exempt = (c: string) => c === '\n' || c === '\u200d' || c === '\ufe0e' || c === '\ufe0f'
export function inkValidateText(text: string, allowLines = false) {
  if (
    [...text].length > CLIP_INK.maxTextRunes ||
    /[\p{Cc}\ufffe\uffff]/u.test(allowLines ? text.replaceAll('\n', '') : text)
  )
    throw new ClipInkError('CLIP_INK_INVALID_TEXT')
}
export function inkCaptionStyle(id: string): ClipCaptionInkStyle {
  const styles = CLIP_CAPTION_INK as Record<string, ClipCaptionInkStyle>
  const style = styles[id]
  if (!style) throw new ClipInkError('CLIP_INK_UNSUPPORTED_STYLE', id)
  return style
}
export function inkRuns(text: string, role: InkRole, caption: boolean): InkRun[] {
  clipInkFont(role.face, role.weight)
  const runs: InkRun[] = []
  for (const c of text) {
    let face = role.face
    if (!exempt(c) && !clipInkCoverage(face, role.weight, c)) {
      if (caption && clipInkCoverage('wantedsans', role.weight, c)) face = 'wantedsans'
      else throw new ClipInkError('CLIP_INK_UNSUPPORTED_GLYPH', `${face}:${c}`)
    }
    const last = runs.at(-1)
    if (last?.face === face) last.text += c
    else runs.push({ text: c, face, substituted: face !== role.face })
  }
  return runs
}
/** The only whole-style substitution the current native glyph contract admits. */
export function inkResolveCaption(id: string, text: string) {
  inkValidateText(text, true)
  let style = inkCaptionStyle(id)
  let notice = ''
  const role = () => ({ ...style.role, face: style.face, weight: style.weight })
  try {
    inkRuns(text, role(), true)
  } catch (error) {
    if (!(error instanceof ClipInkError) || error.code !== 'CLIP_INK_UNSUPPORTED_GLYPH') throw error
    style = inkCaptionStyle('bold')
    inkRuns(text, role(), true)
    notice = 'composition_caption_glyph'
  }
  return { style, notice }
}
export function inkTextMarkup(
  text: string,
  role: InkRole,
  caption: boolean,
  keyword = '',
  accent = '',
) {
  const parts =
    keyword && accent && text.includes(keyword)
      ? [
          { text: text.slice(0, text.indexOf(keyword)), fill: '' },
          { text: keyword, fill: accent },
          { text: text.slice(text.indexOf(keyword) + keyword.length), fill: '' },
        ]
      : [{ text, fill: '' }]
  return parts
    .map((part) =>
      inkRuns(part.text, role, caption)
        .map((run) => {
          const family = run.substituted
            ? ` font-family="${clipInkFont(run.face, role.weight).family}"`
            : ''
          const fill = part.fill ? ` fill="${part.fill}"` : ''
          return family || fill
            ? `<tspan${family}${fill}>${inkEscape(run.text)}</tspan>`
            : inkEscape(run.text)
        })
        .join(''),
    )
    .join('')
}
