import {
  CLIP_DESIGN,
  CLIP_INK,
  CLIP_CAPTION_TRANSFORM_PAINT as PAINT,
} from '@/entities/clip-design/@x/clip-preview'
import type { ClipRatioId } from '@/entities/clip-design/@x/clip-preview'
import { document as inkDocument, type InkPaint } from './ink-static'
import {
  inkCaptionFonts,
  inkStrokeWidth,
  type InkCaptionLayout,
  type InkCaptionLine,
} from './ink-layout'
import { inkNumber as num, inkTextMarkup, type InkBox } from './ink-typography'
import type { InkDocument } from './ink-raster'

export type InkMatrix = readonly [number, number, number, number, number, number]
export interface InkCaptionPose {
  opacity: number
  matrix: InkMatrix
  clip?: InkBox
  tint?: string
  rect?: InkBox
  effect?:
    | { kind: 'blur'; sigma: number }
    | {
        kind: 'neon'
        wideSigma: number
        tightSigma: number
        wideAlpha: number
        tightAlpha: number
        wide: string
        tight: string
      }
    | { kind: 'gradient'; box: InkBox; stops: readonly { at: number; hex: string }[] }
  light?: { ellipses: readonly { cx: number; cy: number; rx: number; ry: number }[] }
}
export interface InkCaptionSceneNode {
  id: string
  document?: InkDocument
  rect?: {
    box: InkBox
    fill: string
    alpha: number
    shadow?: InkCaptionLayout['style']['paint']['shadow']
  }
  light?: {
    palettes: readonly (readonly { at: number; hex: string; alpha: number }[])[]
    sigma: number
  }
  pose: (progress: number, durationMs: number) => InkCaptionPose
}
export interface InkCaptionScene {
  layout: InkCaptionLayout
  nodes: InkCaptionSceneNode[]
  opacity: (progress: number, durationMs: number) => number
  bounds: InkBox
}
export const inkClamp = (n: number) => Math.max(0, Math.min(1, n))
export const inkCubic = (n: number) => 1 - (1 - n) ** 3
export const inkBack = (n: number) => 1 + 2.70158 * (n - 1) ** 3 + 1.70158 * (n - 1) ** 2
export const inkRound = (n: number) => Number(num(n))
const identity: InkMatrix = [1, 0, 0, 1, 0, 0]
const shadow = (id: string, s: InkCaptionLayout['style']['paint']['shadow']) =>
  `<filter id="${id}" x="-30%" y="-30%" width="160%" height="180%" color-interpolation-filters="sRGB"><feOffset in="SourceAlpha" dx="${num(s.dx)}" dy="${num(s.dy)}" result="o"/><feGaussianBlur in="o" stdDeviation="${num(s.blur / 2)}" result="b"/><feFlood flood-color="${s.hex}" flood-opacity="${num(s.alpha)}"/><feComposite in2="b" operator="in" result="s"/><feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`
export const inkTranslation = (x: number, y: number): InkMatrix => [
  1,
  0,
  0,
  1,
  inkRound(x),
  inkRound(y),
]
export const inkPivot = (cx: number, cy: number, scale: number, degrees = 0): InkMatrix => {
  cx = inkRound(cx)
  cy = inkRound(cy)
  scale = inkRound(scale)
  const rotation = (inkRound(degrees) * Math.PI) / 180,
    c = Math.cos(rotation) * scale,
    s = Math.sin(rotation) * scale
  return [c, s, -s, c, cx - c * cx + s * cy, cy - s * cx - c * cy]
}
export function inkEntrance(layout: InkCaptionLayout, progress: number, durationMs: number) {
  if (durationMs <= 0) return { opacity: 1, dy: 0 }
  const motion = layout.style.motion,
    elapsed = progress * durationMs
  const entrance = inkCubic(inkClamp(elapsed / Math.max(1, motion.inMs)))
  return {
    opacity: inkRound(
      Math.min(entrance, inkClamp((durationMs - elapsed) / Math.max(1, motion.outMs))),
    ),
    dy: inkRound(motion.dy * (1 - entrance)),
  }
}
export function inkPopProgress(progress: number, index: number, count: number) {
  const p = PAINT.pop
  const step =
    count > p.envelopeWords ? (p.stagger * (p.envelopeWords - 1)) / (count - 1) : p.stagger
  return inkClamp((progress - (p.onset + index * step)) / p.rise)
}

/** Intrinsic caption math is independent of wall clock, seek direction and renderer speed. */
export function inkCaptionScene(
  ratio: ClipRatioId,
  layout: InkCaptionLayout,
  paint: InkPaint = {},
): InkCaptionScene {
  const { style, role, region, lines } = layout
  const fonts = inkCaptionFonts(layout)
  const accent = style.paint.accent ? (paint.accentWhite ? PAINT.white : paint.accent || '') : ''
  const chosen = (fallback: string) => accent || fallback
  const stroke = style.paint.stroke
    ? ` stroke="${style.paint.stroke}" stroke-width="${num(inkStrokeWidth(style.rule.stroke))}" stroke-linejoin="round" paint-order="stroke"`
    : ''
  const text = (line: Pick<InkCaptionLine, 'text' | 'x' | 'y'>, extra: string) =>
    `<text x="${num(line.x)}" y="${num(line.y)}" xml:space="preserve" font-family="${fonts[0]!.family}" font-size="${num(role.size)}" font-weight="${role.weight}" letter-spacing="${num(role.tracking * role.size)}"${extra}>${inkTextMarkup(line.text, role, true)}</text>`
  const doc = (
    id: string,
    box: InkBox,
    bleed: number,
    draw: (origin: { x: number; y: number }) => string,
  ) => {
    const value = inkDocument(ratio, `caption/${style.id}/${id}`, box, bleed, fonts, draw)
    const scale = CLIP_INK.transformInkStyles.includes(style.id) ? CLIP_INK.transformInkScale : 1
    return { ...value, key: JSON.stringify([value.key, scale]), rasterScale: scale }
  }
  const movedLine = (
    line: Pick<InkCaptionLine, 'text' | 'x' | 'y'>,
    o: { x: number; y: number },
  ) => ({ ...line, x: line.x - o.x, y: line.y - o.y })
  const pose = (opacity = 1, matrix: InkMatrix = identity): InkCaptionPose => ({
    opacity: inkRound(opacity),
    matrix,
  })
  const nodes: InkCaptionSceneNode[] = []
  const opacity = (p: number, duration: number) => inkEntrance(layout, p, duration).opacity
  const settling = (p: number, duration: number) =>
    duration <= 0 || style.motion.inMs <= 0 ? 1 : inkClamp((p * duration) / style.motion.inMs)
  const wholeText = (o: { x: number; y: number }, extra: string) =>
    lines.map((line) => text(movedLine(line, o), extra)).join('')
  const words = lines.flatMap((line) => line.words.map((word) => ({ line, word })))
  if (style.id === 'word-pop') {
    for (const [index, { line, word }] of words.entries()) {
      const one = { text: word.text, x: word.x, y: line.y },
        box = { x: word.x, y: line.top, width: word.width, height: line.height }
      const state = (p: number) => {
        const active = Math.trunc(inkClamp((p - 0.1) / 0.74) * words.length)
        const alpha = index < active ? 0.62 : index === active ? 1 : 0.22
        return {
          alpha,
          matrix:
            index === active
              ? inkPivot(word.x + word.width / 2, line.y - role.size * 0.3, 1.07)
              : identity,
          tint: index === active ? chosen(style.paint.fill) : style.paint.fill,
        }
      }
      if (stroke)
        nodes.push({
          id: `${index}/stroke`,
          document: doc(`${index}/stroke`, box, style.bleed, (o) =>
            text(movedLine(one, o), ` fill="none"${stroke}`),
          ),
          pose: (p) => {
            const s = state(p)
            return pose(s.alpha * 0.9, s.matrix)
          },
        })
      nodes.push({
        id: `${index}/fill`,
        document: doc(`${index}/fill`, box, style.bleed, (o) =>
          text(movedLine(one, o), ` fill="${PAINT.white}"`),
        ),
        pose: (p) => {
          const s = state(p)
          return { ...pose(s.alpha, s.matrix), tint: s.tint }
        },
      })
    }
  } else if (style.id === 'pop') {
    for (const [index, { line, word }] of words.entries()) {
      const one = { text: word.text, x: word.x, y: line.y },
        box = { x: word.x, y: line.top, width: word.width, height: line.height }
      nodes.push({
        id: `${index}`,
        document: doc(
          `${index}`,
          box,
          style.bleed,
          (o) =>
            text(movedLine(one, o), ` fill="${style.paint.fill}"${stroke}`) +
            text(movedLine(one, o), ` fill="${style.paint.fill}"`),
        ),
        pose: (progress) => {
          const p = inkPopProgress(progress, index, words.length),
            scale = p > 0 ? Math.max(inkBack(p), 0.001) : 0.001,
            rotation = (1 - p) * 7 * (index % 2 ? -1 : 1)
          return pose(
            Math.min(1, p * 2.2),
            inkPivot(word.x + word.width / 2, line.y - role.size * 0.32, scale, rotation),
          )
        },
      })
    }
  } else if (style.id === 'stack') {
    for (const [index, line] of lines.entries()) {
      const box = {
          x: line.x - 26,
          y: line.top - 15,
          width: line.width + 52,
          height: line.height + 30,
        },
        plate = index % 2 ? chosen(PAINT.white) : style.paint.plate,
        fill = index % 2 ? PAINT.stackInk : style.paint.fill
      nodes.push({
        id: `${index}`,
        document: doc(
          `${index}`,
          box,
          0,
          (o) =>
            `<rect x="${num(box.x - o.x)}" y="${num(box.y - o.y)}" width="${num(box.width)}" height="${num(box.height)}" rx="10" fill="${plate}" fill-opacity="0.94"/>` +
            text(movedLine(line, o), ` fill="${fill}"`),
        ),
        pose: (progress) => {
          const p = inkCubic(inkClamp((progress - 0.06 - index * 0.12) / 0.34))
          return pose(p, inkTranslation((1 - p) * -70, 0))
        },
      })
    }
  } else if (style.id === 'outline') {
    for (const [index, line] of lines.entries()) {
      const box = { x: line.x, y: line.top, width: line.width, height: line.height }
      const matrix = (progress: number, duration: number) =>
        inkTranslation(0, inkEntrance(layout, progress, duration).dy)
      nodes.push({
        id: `${index}/stroke`,
        document: doc(`${index}/stroke`, box, style.bleed, (o) =>
          text(
            movedLine(line, o),
            ` fill="none" stroke="${style.paint.stroke}" stroke-width="${num(inkStrokeWidth(style.rule.stroke))}" stroke-linejoin="round"`,
          ),
        ),
        pose: (p, d) => pose(1, matrix(p, d)),
      })
      if (line.keyword)
        nodes.push({
          id: `${index}/fill`,
          document: doc(`${index}/fill`, box, style.bleed, (o) =>
            text(movedLine(line, o), ` fill="${chosen(PAINT.white)}"`),
          ),
          pose: (p, d) => ({
            ...pose(1, matrix(p, d)),
            clip: {
              x: inkRound(line.keywordX - 6),
              y: inkRound(line.y - role.size),
              width: inkRound((line.keywordWidth + 12) * inkCubic(inkClamp((p - 0.28) / 0.28))),
              height: inkRound(role.size * 1.6),
            },
          }),
        })
    }
  } else if (style.id === 'sticker') {
    const pw = region.width + 72,
      ph = region.height + 46,
      px = region.x - 36,
      py = region.y - 23
    const document = doc(
      'sticker',
      region,
      style.bleed,
      (o) =>
        `<defs>${shadow('sticker', { hex: PAINT.black, alpha: 0.38, blur: 18, dx: 0, dy: 8 })}</defs><g filter="url(#sticker)"><rect x="${num(px - o.x)}" y="${num(py - o.y)}" width="${num(pw)}" height="${num(ph)}" rx="${num(ph * 0.34)}" fill="${style.paint.plate}"/><rect x="${num(px + 6 - o.x)}" y="${num(py + 6 - o.y)}" width="${num(pw - 12)}" height="${num(ph - 12)}" rx="${num(ph * 0.29)}" fill="none" stroke="${chosen(PAINT.stickerEdge)}" stroke-width="3" stroke-opacity="0.85"/>${wholeText(o, ` fill="${style.paint.fill}"`)}</g>`,
    )
    nodes.push({
      id: 'sticker',
      document,
      pose: (progress, duration) => {
        const p = settling(progress, duration)
        return pose(
          1,
          inkPivot(
            region.x + region.width / 2,
            region.y + region.height / 2,
            Math.max(inkBack(p), 0.001),
            -2.4 + (1 - p) * 8,
          ),
        )
      },
    })
  } else if (style.id === 'bubble') {
    const pw = region.width + 64,
      ph = region.height + 40,
      px = region.x - 32,
      py = region.y - 20
    const document = doc(
      'bubble',
      region,
      style.bleed,
      (o) =>
        `<defs>${shadow('bubble', { hex: PAINT.black, alpha: 0.42, blur: 20, dx: 0, dy: 6 })}</defs><g filter="url(#bubble)"><path d="M${num(px + 34 - o.x)} ${num(py + ph - o.y)} L${num(px + 30 - o.x)} ${num(py + ph + 26 - o.y)} L${num(px + 72 - o.x)} ${num(py + ph - o.y)} Z" fill="${style.paint.plate}"/><rect x="${num(px - o.x)}" y="${num(py - o.y)}" width="${num(pw)}" height="${num(ph)}" rx="${num(ph * 0.42)}" fill="${style.paint.plate}"/>${wholeText(o, ` fill="${style.paint.fill}"`)}</g>`,
    )
    nodes.push({
      id: 'bubble',
      document,
      pose: (p, d) => pose(1, inkTranslation(0, inkEntrance(layout, p, d).dy)),
    })
  } else if (style.id === 'serif') {
    const document = doc(
      'text',
      region,
      style.bleed,
      (o) =>
        `<defs>${shadow('serif', style.paint.shadow)}</defs><g filter="url(#serif)">${wholeText(o, ` fill="${style.paint.fill}"`)}</g>`,
    )
    nodes.push({
      id: 'text',
      document,
      pose: (p, d) => pose(1, inkTranslation(0, inkEntrance(layout, p, d).dy)),
    })
    for (const [index, y] of [region.y - 26, region.y + region.height + 24].entries()) {
      const box = { x: region.x - 20, y, width: region.width + 40, height: 1.4 }
      nodes.push({
        id: `rule/${index}`,
        rect: { box, fill: PAINT.serifRule, alpha: 0.72, shadow: style.paint.shadow },
        pose: (p, d) => {
          const width = inkRound((region.width + 40) * inkCubic(inkClamp((p - 0.2) / 0.42)))
          return {
            ...pose(0.72, inkTranslation(0, inkEntrance(layout, p, d).dy)),
            rect: {
              x: inkRound(region.x + region.width / 2 - width / 2),
              y: inkRound(y),
              width,
              height: 1.4,
            },
          }
        },
      })
    }
  } else throw new Error(`CLIP_INK_STYLE_NOT_IMPLEMENTED:${style.id}`)
  const canvas = CLIP_DESIGN.ratios[ratio].canvas,
    b = style.bleed,
    x = Math.max(0, Math.floor(region.x - b)),
    y = Math.max(0, Math.floor(region.y - b))
  return {
    layout,
    nodes,
    opacity,
    bounds: {
      x,
      y,
      width: Math.min(canvas.width, Math.ceil(region.x + region.width + b)) - x,
      height: Math.min(canvas.height, Math.ceil(region.y + region.height + b)) - y,
    },
  }
}
