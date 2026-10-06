import {
  CLIP_DESIGN,
  clipInkFont,
  type ClipRatioId,
  type ClipCaptionInkStyle,
} from '@/entities/clip-design/@x/clip-preview'
import { copyChars } from '@/entities/clip-plan/@x/clip-preview'
import { ClipInkError, inkResolveCaption, type InkBox, type InkRole } from './ink-typography'
import type { BrowserInkRasterizer } from './ink-raster'

export interface InkCaptionInput {
  text: string
  style: string
  ratio: ClipRatioId
  position: string
  align: string
  keyword: string
  pace: string
  ownerPosition?: { x: number; y: number }
  ownerSizePx?: number
}
export interface InkCaptionWord {
  text: string
  x: number
  width: number
}
export interface InkCaptionLine {
  text: string
  x: number
  y: number
  top: number
  width: number
  height: number
  words: InkCaptionWord[]
  keyword: string
  keywordX: number
  keywordWidth: number
}
export interface InkCaptionLayout {
  style: ClipCaptionInkStyle
  role: InkRole
  region: InkBox
  lines: InkCaptionLine[]
  notice: string
}
export function inkStrokeWidth(stroke: string) {
  return stroke === 'text'
    ? CLIP_DESIGN.spacing.stroke_text
    : stroke === 'small'
      ? CLIP_DESIGN.spacing.stroke_small
      : 0
}
export function inkPlace(
  ratio: ClipRatioId,
  anchor: string,
  align: string,
  width: number,
  height: number,
  owner?: { x: number; y: number },
): InkBox {
  const { anchor: a, safe: s } = CLIP_DESIGN.ratios[ratio]
  let x: number, y: number
  if (owner) {
    if (width > s.width || height > s.height) throw new ClipInkError('CLIP_INK_SAFE_AREA')
    x = Math.min(Math.max(owner.x, s.x), s.x + s.width - width)
    y = Math.min(Math.max(owner.y, s.y), s.y + s.height - height)
  } else {
    if (
      !['top', 'upper_mid', 'lower_mid', 'bottom'].includes(anchor) ||
      !['left', 'center', 'right'].includes(align)
    )
      throw new ClipInkError('CLIP_INK_UNRESOLVED_GEOMETRY', anchor)
    x = align === 'center' ? a.center - width / 2 : align === 'right' ? a.right - width : a.left
    y =
      anchor === 'upper_mid'
        ? a.upper_mid - height / 2
        : anchor === 'lower_mid'
          ? a.lower_mid - height / 2
          : anchor === 'bottom'
            ? a.bottom - height
            : a.top
  }
  if (x < s.x || y < s.y || x + width > s.x + s.width || y + height > s.y + s.height)
    throw new ClipInkError('CLIP_INK_SAFE_AREA')
  return { x, y, width, height }
}
function candidates(text: string, maxLines: number): string[][] {
  if (text.includes('\n')) {
    const lines = text.split('\n')
    if (maxLines < 2 || lines.length !== 2 || lines.some((l) => !l.trim()))
      throw new ClipInkError('CLIP_INK_COPY_LIMIT')
    return [lines]
  }
  const out = [[text]]
  if (maxLines < 2) return out
  const segments = new Intl.Segmenter('ko', { granularity: 'grapheme' })
  for (const item of segments.segment(text)) {
    const end = item.index + item.segment.length
    if (end === text.length) continue
    const lines = [text.slice(0, end), text.slice(end)]
    if (lines.every((line) => line.trim())) out.push(lines)
  }
  return out
}
const scaled = (box: InkBox, factor: number): InkBox => ({
  x: box.x * factor,
  y: box.y * factor,
  width: box.width * factor,
  height: box.height * factor,
})

/** Native copy.go fitCopy, with shaped logical bounds and unchanged authored size. */
export async function inkLayoutCaption(
  input: InkCaptionInput,
  rasterizer: BrowserInkRasterizer,
  signal?: AbortSignal,
): Promise<InkCaptionLayout> {
  const { style, notice } = inkResolveCaption(input.style, input.text)
  const role = { face: style.face, weight: style.weight, tracking: style.role.tracking, size: 100 }
  const choices = candidates(input.text, input.pace === 'rapid' ? 1 : style.rule.lines)
  const values = new Set(choices.flat())
  for (const line of [...values]) {
    if (input.keyword && line.includes(input.keyword)) {
      values.add(input.keyword)
      values.add(line.slice(0, line.indexOf(input.keyword) + input.keyword.length))
    }
    if (style.perWord) {
      const words = [...line.matchAll(/\S+/gu)]
      for (const word of words) {
        values.add(word[0])
        values.add(line.slice(0, word.index + word[0].length))
      }
    }
  }
  const measured = new Map<string, InkBox>()
  for (const value of values)
    measured.set(value, await rasterizer.measure(value, role, true, signal))
  const stroke = inkStrokeWidth(style.rule.stroke) / 2
  const left = Math.max(style.rule.padding.h, style.rule.pad_left) + stroke
  const right = style.rule.padding.h + stroke
  const vertical = style.rule.padding.v + stroke
  const largest = input.ownerSizePx || style.role.size
  const smallest = input.ownerSizePx || style.role.min
  if (largest < style.role.min || largest > style.role.size || !Number.isInteger(largest))
    throw new ClipInkError('CLIP_INK_CAPTION_SIZE')
  for (let size = largest; size >= smallest; size--) {
    let best:
      | { lines: string[]; bounds: InkBox[]; region: InkBox; clean: boolean; width: number }
      | undefined
    for (const lines of choices) {
      const chars = input.pace === 'rapid' ? CLIP_DESIGN.rapid.max_chars : style.rule.chars
      if (lines.some((line) => chars > 0 && copyChars(line) > chars)) continue
      const bounds = lines.map((line) => scaled(measured.get(line)!, size / 100))
      const width = Math.max(...bounds.map((b) => b.width)) + left + right
      const height =
        bounds.reduce((n, b) => n + b.height, 0) +
        (lines.length - 1) * size * (style.role.lineHeight - 1) +
        2 * vertical
      let region: InkBox
      try {
        region = inkPlace(
          input.ratio,
          input.position,
          input.align,
          Math.ceil(width),
          Math.ceil(height),
          input.ownerPosition,
        )
      } catch (error) {
        if (error instanceof ClipInkError && error.code === 'CLIP_INK_SAFE_AREA') continue
        throw error
      }
      const clean = lines.length < 2 || lines[0]!.endsWith(' ') || lines[1]!.startsWith(' ')
      if (!best || (clean && !best.clean) || (clean === best.clean && width < best.width))
        best = { lines, bounds, region, clean, width }
      if (lines.length === 1) break
    }
    if (!best) continue
    const layout: InkCaptionLayout = {
      style,
      role: { ...role, size },
      region: best.region,
      lines: [],
      notice,
    }
    let top = best.region.y + vertical
    const inner = best.region.width - left - right
    for (const [index, text] of best.lines.entries()) {
      const box = best.bounds[index]!
      const x = best.region.x + left + (inner - box.width) / 2
      const line: InkCaptionLine = {
        text,
        x: x - box.x,
        y: top - box.y,
        top,
        width: box.width,
        height: box.height,
        words: [],
        keyword: '',
        keywordX: 0,
        keywordWidth: 0,
      }
      if (input.keyword && text.includes(input.keyword)) {
        const through = measured.get(
          text.slice(0, text.indexOf(input.keyword) + input.keyword.length),
        )!
        const word = measured.get(input.keyword)!
        const whole = measured.get(text)!
        line.keyword = input.keyword
        line.keywordX = x + ((through.x + through.width - word.width - whole.x) * size) / 100
        line.keywordWidth = (word.width * size) / 100
      }
      if (style.perWord)
        for (const word of text.matchAll(/\S+/gu)) {
          const through = measured.get(text.slice(0, word.index + word[0].length))!
          const single = measured.get(word[0])!
          const whole = measured.get(text)!
          line.words.push({
            text: word[0],
            x: x + ((through.x + through.width - single.width - whole.x) * size) / 100,
            width: (single.width * size) / 100,
          })
        }
      layout.lines.push(line)
      top += box.height + size * (style.role.lineHeight - 1)
    }
    return layout
  }
  throw new ClipInkError('CLIP_INK_COPY_LIMIT', input.style)
}

export function inkCaptionFonts(layout: InkCaptionLayout) {
  const fonts = new Map([
    [
      clipInkFont(layout.role.face, layout.role.weight).sha256,
      clipInkFont(layout.role.face, layout.role.weight),
    ],
  ])
  // Caption substitution is one face, at the original style's exact weight.
  const wanted = clipInkFont('wantedsans', layout.role.weight)
  fonts.set(wanted.sha256, wanted)
  return [...fonts.values()]
}
