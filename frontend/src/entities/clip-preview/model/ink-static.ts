import {
  CLIP_DESIGN,
  CLIP_INK,
  clipInkFont,
  clipLayoutRegion,
  clipRegionSlots,
  type ClipInkFont,
  type ClipRatioId,
  type ClipRegionLayout,
  type ClipRegionShape,
} from '@/entities/clip-design/@x/clip-preview'
import {
  ClipInkError,
  inkEscape,
  inkNumber,
  inkTextMarkup,
  inkValidateText,
  type InkBox,
  type InkRole,
} from './ink-typography'
import { inkCaptionFonts, inkStrokeWidth, inkPlace, type InkCaptionLayout } from './ink-layout'
import type { BrowserInkRasterizer, InkDocument } from './ink-raster'
import { backgroundBoxUnion } from './background-math'

export interface InkPaint {
  accent?: string
  accentWhite?: boolean
}
export interface InkRegionPart {
  offset: number
  count: number
  rules: boolean
}
const BLACK = '#000000' // style-escape: native component shadow/fill, not interface colour

function shadow(
  id: string,
  paint: { hex: string; alpha: number; blur: number; dx: number; dy: number },
  srgb = true,
) {
  return `<filter id="${id}" x="-20%" y="-20%" width="140%" height="140%"${srgb ? ' color-interpolation-filters="sRGB"' : ''}><feDropShadow dx="${inkNumber(paint.dx)}" dy="${inkNumber(paint.dy)}" stdDeviation="${inkNumber(paint.blur / 2)}" flood-color="${paint.hex}" flood-opacity="${paint.alpha}"/></filter>`
}
function text(
  text: string,
  role: InkRole,
  x: number,
  y: number,
  paint: string,
  caption = false,
  keyword = '',
  accent = '',
) {
  return `<text x="${inkNumber(x)}" y="${inkNumber(y)}" xml:space="preserve" font-family="${clipInkFont(role.face, role.weight).family}" font-size="${role.size.toFixed(0)}" font-weight="${role.weight}" letter-spacing="${(role.tracking * role.size).toFixed(4)}" ${paint}>${inkTextMarkup(text, role, caption, keyword, accent)}</text>`
}
function document(
  ratio: ClipRatioId,
  kind: string,
  bounds: InkBox,
  bleed: number,
  fonts: readonly ClipInkFont[],
  draw: (origin: { x: number; y: number }) => string,
): InkDocument {
  const nominal = { x: bounds.x - bleed, y: bounds.y - bleed }
  const origin = { x: Math.floor(nominal.x), y: Math.floor(nominal.y) }
  const phase = { x: nominal.x - origin.x, y: nominal.y - origin.y }
  const width = Math.ceil(bounds.width + 2 * bleed) + 1,
    height = Math.ceil(bounds.height + 2 * bleed) + 1
  if (!(width > 0 && height > 0)) throw new ClipInkError('CLIP_INK_INVALID_GEOMETRY', kind)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">${draw(origin)}</svg>`
  // Absolute movement is absent: the body is normalized around the same ink.
  const key = JSON.stringify([
    CLIP_INK.version,
    ratio,
    kind,
    fonts.map((f) => f.sha256),
    draw(nominal),
    width,
    height,
  ])
  return { key, svg, fonts, phase, placement: { ...bounds }, bounds: { ...origin, width, height } }
}
const token = (name: string) =>
  (CLIP_DESIGN.color as Record<string, { hex: string; alpha: number }>)[name]
function defaultShadow() {
  const s = CLIP_DESIGN.shadow.text
  return { hex: s.hex, alpha: s.alpha, blur: s.blur, dx: s.dx, dy: s.dy }
}

/** Native bundled caption template, preserving fill/stroke/shadow and substituted runs. */
export function inkStaticCaption(
  ratio: ClipRatioId,
  layout: InkCaptionLayout,
  paint: InkPaint = {},
): InkDocument {
  const { style, role, region } = layout
  if (style.rendering !== 'static')
    throw new ClipInkError('CLIP_INK_STYLE_NOT_IMPLEMENTED', style.id)
  const accent = style.paint.accent
    ? paint.accentWhite
      ? CLIP_DESIGN.color.text_white.hex
      : paint.accent || ''
    : ''
  return document(
    ratio,
    `caption/${style.id}`,
    region,
    style.bleed,
    inkCaptionFonts(layout),
    (o) => {
      const s = style.paint.shadow
      const defs = style.rule.shadow ? `<defs>${shadow('shadow', s)}</defs>` : ''
      return (
        defs +
        layout.lines
          .map((line) =>
            text(
              line.text,
              role,
              line.x - o.x,
              line.y - o.y,
              `fill="${style.paint.fill}" stroke="${style.paint.stroke || 'none'}" stroke-opacity="${CLIP_DESIGN.color.stroke_dark.alpha}" stroke-width="${inkNumber(inkStrokeWidth(style.rule.stroke))}" stroke-linejoin="round" paint-order="stroke fill"${style.rule.shadow ? ' filter="url(#shadow)"' : ''}`,
              true,
              line.keyword,
              accent,
            ),
          )
          .join('')
      )
    },
  )
}
function shapeMarkup(shape: ClipRegionShape, o: { x: number; y: number }) {
  const b = shape.box
  const geometry = shape.circle
    ? `circle cx="${inkNumber(b.x + b.width / 2 - o.x)}" cy="${inkNumber(b.y + b.height / 2 - o.y)}" r="${inkNumber(shape.radius)}"`
    : `rect x="${inkNumber(b.x - o.x)}" y="${inkNumber(b.y - o.y)}" width="${inkNumber(b.width)}" height="${inkNumber(b.height)}" rx="${inkNumber(shape.radius)}"`
  return `<${geometry} fill="${shape.fill || 'none'}" fill-opacity="${shape.fill ? shape.fillAlpha : 1}" stroke="${shape.stroke || 'none'}" stroke-opacity="${shape.stroke ? shape.strokeAlpha : 1}" stroke-width="${inkNumber(shape.strokeWidth)}"${shape.shadow ? ' filter="url(#shadow)"' : ''}/>`
}

/** One native region entry owns its slots; all entries share one measured block. */
export async function inkStaticRegion(
  ratio: ClipRatioId,
  kind: 'intro' | 'outro',
  id: string,
  rows: readonly string[],
  part: InkRegionPart,
  rasterizer: BrowserInkRasterizer,
  signal?: AbortSignal,
): Promise<{ document?: InkDocument; layout: ClipRegionLayout }> {
  const layout = clipLayoutRegion(kind, id, ratio, [...rows])
  if (!layout) throw new ClipInkError('CLIP_INK_UNSUPPORTED_REGION', `${kind}/${id}`)
  const slots = layout.slots.filter(
    (s) => s.index >= part.offset && s.index < part.offset + part.count,
  )
  const fonts = new Map<string, ClipInkFont>()
  const positioned: {
    line: (typeof slots)[number]['lines'][number]
    role: InkRole
    spec: (typeof slots)[number]['spec']
    x: number
    box: InkBox
  }[] = []
  for (const slot of slots) {
    if (slot.over) throw new ClipInkError('CLIP_INK_COPY_LIMIT', `${kind}/${id}/${slot.index}`)
    const role = slot.type
    const font = clipInkFont(role.face, role.weight)
    fonts.set(font.sha256, font)
    for (const line of slot.lines) {
      inkValidateText(line.text)
      const measured = await rasterizer.measure(line.text, role, false, signal)
      const width = (measured.width * role.size) / 100
      const x = line.align === 'left' ? line.x : line.x - width / 2
      positioned.push({
        line,
        role,
        spec: slot.spec,
        x,
        box: line.arc
          ? slot.box
          : {
              x,
              y: line.baseline + (measured.y * role.size) / 100,
              width,
              height: (measured.height * role.size) / 100,
            },
      })
    }
  }
  if (!positioned.length) return { layout }
  const shapes = layout.shapes.filter((s) =>
    s.slot < 0 ? part.rules : s.slot >= part.offset && s.slot < part.offset + part.count,
  )
  const rules = part.rules ? layout.rules : []
  const doc = document(ratio, `${kind}/${id}`, layout.bounds, 80, [...fonts.values()], (o) => {
    const draw = (rotated: boolean) => {
      const ruleBody =
        rotated === (layout.rotate.deg !== 0)
          ? rules
              .map(
                (r) =>
                  `<rect x="${inkNumber(r.box.x - o.x)}" y="${inkNumber(r.box.y - o.y)}" width="${inkNumber(r.box.width)}" height="${inkNumber(r.box.height)}" fill="${CLIP_DESIGN.color.text_white.hex}" fill-opacity="${r.alpha}"/>`,
              )
              .join('')
          : ''
      const shapeBody = shapes
        .filter((s) => s.rotated === rotated)
        .map((s) => shapeMarkup(s, o))
        .join('')
      const lineBody = positioned
        .filter((p) => p.line.rotated === rotated)
        .map(({ line, role, spec, x }, index) => {
          const fill = spec.outline ? 'none' : token(spec.fill)?.hex || spec.fill
          const alpha = spec.alpha ?? token(spec.fill)?.alpha ?? 1
          const stroked = spec.stroke === 'text' || spec.stroke === 'small'
          const stroke = spec.outline
            ? token(spec.fill)?.hex || spec.fill
            : stroked
              ? CLIP_DESIGN.color.stroke_dark.hex
              : 'none'
          const strokeAlpha = spec.outline
            ? alpha
            : stroked
              ? CLIP_DESIGN.color.stroke_dark.alpha
              : 1
          const width = spec.outline || spec.stroke_width || inkStrokeWidth(spec.stroke)
          const paint = `fill="${fill}" fill-opacity="${alpha}" stroke="${stroke}" stroke-opacity="${strokeAlpha}" stroke-width="${inkNumber(width)}" stroke-linejoin="round" paint-order="stroke fill"${spec.shadow ? ' filter="url(#shadow)"' : ''}`
          if (!line.arc) return text(line.text, role, x - o.x, line.baseline - o.y, paint)
          const a = line.arc,
            arc = `M ${inkNumber(a.cx - a.r - o.x)} ${inkNumber(a.cy - o.y)} A ${inkNumber(a.r)} ${inkNumber(a.r)} 0 0 ${a.lower ? 0 : 1} ${inkNumber(a.cx + a.r - o.x)} ${inkNumber(a.cy - o.y)}`
          return `<defs><path id="arc${index}" d="${arc}"/></defs><text xml:space="preserve" font-family="${clipInkFont(role.face, role.weight).family}" font-size="${role.size.toFixed(0)}" font-weight="${role.weight}" letter-spacing="${(role.tracking * role.size).toFixed(4)}" ${paint} text-anchor="middle"><textPath href="#arc${index}" startOffset="50%">${inkEscape(line.text)}</textPath></text>`
        })
        .join('')
      return ruleBody + shapeBody + lineBody
    }
    const turn = layout.rotate.deg
      ? `<g transform="rotate(${inkNumber(layout.rotate.deg)} ${inkNumber(layout.rotate.cx - o.x)} ${inkNumber(layout.rotate.cy - o.y)})">${draw(true)}</g>`
      : ''
    return `<defs>${shadow('shadow', defaultShadow(), false)}</defs>${draw(false)}${turn}`
  })
  doc.sampledBounds = backgroundBoxUnion(layout.slots.map((slot) => slot.box))
  doc.contrastParts = positioned.map(({ box, spec }) => ({
    box,
    fill: token(spec.fill)?.hex || spec.fill,
    alpha: spec.alpha ?? token(spec.fill)?.alpha ?? 1,
    stroke: !spec.outline && (spec.stroke === 'text' || spec.stroke === 'small'),
  }))
  return { document: doc, layout }
}

export async function inkStaticBadge(
  ratio: ClipRatioId,
  value: string,
  position: string,
  align: string,
  rasterizer: BrowserInkRasterizer,
  signal?: AbortSignal,
): Promise<InkDocument> {
  inkValidateText(value)
  if (!value.trim()) throw new ClipInkError('CLIP_INK_COPY_LIMIT', 'badge')
  const t = CLIP_DESIGN.type.badge
  const role = { face: t.face, weight: t.weight, tracking: t.tracking, size: t.size }
  const m = await rasterizer.measure(value, role, false, signal),
    factor = role.size / 100
  const b = { x: m.x * factor, y: m.y * factor, width: m.width * factor, height: m.height * factor }
  const pad = CLIP_DESIGN.spacing.pad_chip
  const width = Math.ceil(b.width + 2 * pad.h),
    height = Math.ceil(role.size + 2 * pad.v)
  const geometry = CLIP_DESIGN.ratios[ratio]
  const header = position === 'header' || position === 'auto'
  const box = header
    ? { x: geometry.badge.right - width, y: geometry.badge.top, width, height }
    : inkPlace(ratio, position, align, width, height)
  if (header && box.x < geometry.anchor.left) throw new ClipInkError('CLIP_INK_SAFE_AREA', 'badge')
  return document(
    ratio,
    'badge',
    box,
    0,
    [clipInkFont(role.face, role.weight)],
    (o) =>
      `<rect x="0" y="0" width="${width}" height="${height}" rx="${CLIP_DESIGN.spacing.radius_chip}" fill="${CLIP_DESIGN.color.badge_ad.hex}" fill-opacity="${CLIP_DESIGN.color.badge_ad.alpha}"/>` +
      text(
        value,
        role,
        box.x + pad.h - b.x - o.x,
        box.y + (height - b.height) / 2 - b.y - o.y,
        `fill="${CLIP_DESIGN.color.text_white.hex}"`,
      ),
  )
}

export const inkRegionCapacity = (kind: 'intro' | 'outro', id: string) =>
  clipRegionSlots(kind, id).length
export const inkBlack = BLACK

export async function inkStaticInfo(
  ratio: ClipRatioId,
  rows: readonly { role: string; text: string }[],
  position: string,
  rasterizer: BrowserInkRasterizer,
  signal?: AbortSignal,
): Promise<{ document?: InkDocument; box?: InkBox }> {
  const geometry = CLIP_DESIGN.ratios[ratio]
  const measured: { text: string; role: InkRole; box: InkBox; fill: string; alpha: number }[] = []
  for (const row of rows) {
    if (!row.text.trim()) continue
    if (row.role !== 'label' && row.role !== 'caption')
      throw new ClipInkError('CLIP_INK_UNSUPPORTED_ROLE', row.role)
    const t = CLIP_DESIGN.type[row.role]
    const role = {
      face: t.face,
      weight: t.weight,
      size: t.size,
      tracking: row.role === 'label' ? CLIP_DESIGN.information.label_tracking : t.tracking,
    }
    const measure = async (value: string) => {
      inkValidateText(value)
      const box = await rasterizer.measure(value, role, false, signal),
        factor = role.size / 100
      return {
        x: box.x * factor,
        y: box.y * factor,
        width: box.width * factor,
        height: box.height * factor,
      }
    }
    let values = [row.text]
    const first = await measure(row.text)
    if (first.width > geometry.copy_max_width) {
      const words = row.text.trim().split(/\s+/u)
      let found = false
      for (let split = words.length - 1; split > 0; split--) {
        const lines = [words.slice(0, split).join(' '), words.slice(split).join(' ')]
        if (
          (await measure(lines[0]!)).width <= geometry.copy_max_width &&
          (await measure(lines[1]!)).width <= geometry.copy_max_width
        ) {
          values = lines
          found = true
          break
        }
      }
      if (!found) throw new ClipInkError('CLIP_INK_COPY_LIMIT', 'info')
    }
    for (const value of values) {
      const paint =
        row.role === 'label' ? CLIP_DESIGN.color.text_muted : CLIP_DESIGN.color.text_white
      measured.push({
        text: value,
        role,
        box: await measure(value),
        fill: paint.hex,
        alpha: paint.alpha,
      })
    }
  }
  if (!measured.length) return {}
  const width = Math.ceil(Math.max(...measured.map((m) => m.box.width)))
  const height = Math.ceil(
    measured.reduce((n, m) => n + Math.max(m.role.size, m.box.height), 0) +
      (measured.length - 1) * CLIP_DESIGN.spacing.gap_stack,
  )
  const box =
    position === 'header' || position === 'auto'
      ? { x: geometry.anchor.left, y: geometry.badge.top, width, height }
      : inkPlace(ratio, position, 'center', width, height)
  const fonts = [
    ...new Map(
      measured.map((m) => {
        const font = clipInkFont(m.role.face, m.role.weight)
        return [font.sha256, font] as const
      }),
    ).values(),
  ]
  const doc = document(ratio, 'info', box, 40, fonts, (o) => {
    let y = box.y
    return (
      `<defs>${shadow('shadow', defaultShadow(), false)}</defs>` +
      measured
        .map((m) => {
          const value = text(
            m.text,
            m.role,
            box.x + (box.width - m.box.width) / 2 - m.box.x - o.x,
            y - m.box.y - o.y,
            `fill="${m.fill}" fill-opacity="${m.alpha}" stroke="${CLIP_DESIGN.color.stroke_dark.hex}" stroke-opacity="${CLIP_DESIGN.color.stroke_dark.alpha}" stroke-width="${CLIP_DESIGN.spacing.stroke_small}" stroke-linejoin="round" paint-order="stroke fill" filter="url(#shadow)"`,
          )
          y += Math.max(m.role.size, m.box.height) + CLIP_DESIGN.spacing.gap_stack
          return value
        })
        .join('')
    )
  })
  let top = box.y
  doc.contrastParts = []
  doc.sampledBounds = backgroundBoxUnion(
    measured.map((m) => {
      const bounds = {
        x: box.x + (box.width - m.box.width) / 2,
        y: top,
        width: m.box.width,
        height: m.box.height,
      }
      top += Math.max(m.role.size, m.box.height) + CLIP_DESIGN.spacing.gap_stack
      doc.contrastParts!.push({ box: bounds, fill: m.fill, alpha: m.alpha, stroke: true })
      return bounds
    }),
  )
  return { document: doc, box }
}
