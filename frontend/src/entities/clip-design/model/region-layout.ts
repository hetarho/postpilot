/** The region block layout (CDS-86, CDS-87), ported line for line from the Go design
 *  package's `LayoutRegion` so the template preview draws what the renderer draws. Both
 *  sides fit text with the same generated advance table (`clip-metrics.json`), and a Go
 *  test writes `region-layouts.fixture.json` that this port must reproduce. */
import design from '../config/clip-design.json'
import metrics from '../config/clip-metrics.json'

type FaceMetrics = {
  hangul: number
  fallback: number
  ink: { top: number; bottom: number }
  glyphs: Record<string, number>
}
const FACES = metrics.faces as Record<string, FaceMetrics>
const UNITS = metrics.units

type RoleName = keyof typeof design.type
export type ClipRegionKind = 'intro' | 'outro'
export type ClipRegionRatio = keyof typeof design.ratios

export type ClipRegionSlotSpec = {
  role: string
  size?: number
  floor?: number
  face?: string
  weight?: number
  tracking?: number
  fill: string
  alpha?: number
  stroke: string
  shadow: string
  lines?: number
  /** A stroke width other than the two tokens (CDS-94). */
  stroke_width?: number
  /** Drawn as a white outline of this width with no fill (CDS-92). */
  outline?: number
  width?: number
  /** Where a stamp slot sits: upper, centre, lower or below (CDS-99). */
  place?: string
  decor?: {
    flank?: { w: number; gap: number; alpha: number }
    dot?: { r: number; gap: number }
    frame?: { pad_v: number; pad_h: number; stroke: number; alpha: number; min_w: number }
    plate?: { pad_v: number; pad_h: number; radius: number }
  }
}
type RegionChips = {
  gap: number
  row_gap: number
  pill: { pad_v: number; pad_h: number; stroke: number; alpha: number; fill_alpha: number }
  slots: ClipRegionSlotSpec[]
}
type RegionList = {
  gap: number
  square: { size: number; gap: number }
  slots: ClipRegionSlotSpec[]
}
type RegionStamp = {
  r_outer: number
  r_inner: number
  stroke_outer: number
  stroke_inner: number
  fill_alpha: number
  dot_r: number
  arc_len: number
  below_gap: number
  rotate: number
}
type RegionItem =
  | { slot: ClipRegionSlotSpec }
  | { rule: { kind: string; w?: number; alpha?: number } }
  | { gap: number }
  | { chips: RegionChips }
  | { list: RegionList }
type RegionPreset = {
  anchor: { kind: string; y?: number; x?: string; inset?: number }
  width?: number
  scrim?: string
  rotate?: number
  side_bar?: { w: number }
  stamp?: RegionStamp
  items: RegionItem[]
}

function faceOf(face: string, weight: number) {
  return FACES[`${face}:${weight}`]
}

const isHangul = (c: string) => {
  const code = c.codePointAt(0) ?? 0
  return code >= 0xac00 && code <= 0xd7a3
}

function advance(m: FaceMetrics, c: string) {
  const listed = m.glyphs[c]
  if (listed !== undefined) return listed
  if (m.hangul > 0 && isHangul(c)) return m.hangul
  return m.fallback
}

/** One line's advance width at `size`: advances plus tracking between characters. */
export function clipTextWidth(
  face: string,
  weight: number,
  tracking: number,
  size: number,
  text: string,
) {
  const m = faceOf(face, weight)
  if (!m) return Infinity
  let sum = 0
  let n = 0
  for (const c of text) {
    sum += advance(m, c)
    n++
  }
  if (n === 0) return 0
  return (size / UNITS) * (sum + tracking * UNITS * (n - 1))
}

/** Whether the face draws every character (Jua and Paperlogy list what they draw). */
export function clipCovers(face: string, weight: number, text: string) {
  const m = faceOf(face, weight)
  if (!m) return false
  for (const c of text) if (m.glyphs[c] === undefined && !(m.hangul > 0)) return false
  return true
}

export type ClipSlotSpec = {
  role: string
  size: number
  floor: number
  face: string
  weight: number
  tracking: number
  lines: number
}
export type ClipSlotFit = { lines: string[]; size: number; over: boolean }

const WRAPPING = new Set(['display', 'headline', 'hook', 'title'])
const maxLines = (s: ClipSlotSpec) => (s.lines > 0 ? s.lines : WRAPPING.has(s.role) ? 2 : 1)
const width100 = (s: ClipSlotSpec, text: string) =>
  clipTextWidth(s.face, s.weight, s.tracking, 100, text)

function fitSize(spec: ClipSlotSpec, width: number, at100: number) {
  if (at100 <= 0) return spec.size
  return Math.min(spec.size, Math.floor((100 * width) / at100 + 1e-9))
}

/** CDS-86: keep the size, shrink to the floor, then two balanced lines for the large roles. */
export function clipFitRegionSlot(spec: ClipSlotSpec, text: string, width: number): ClipSlotFit {
  if (text.trim() === '') return { lines: [], size: spec.size, over: false }
  let floor = Math.min(spec.floor, spec.size)
  if (floor <= 0) floor = spec.size
  const covered = clipCovers(spec.face, spec.weight, text)
  const one = fitSize(spec, width, width100(spec, text))
  if (one >= floor) return { lines: [text], size: one, over: !covered }
  if (maxLines(spec) >= 2) {
    const words = text.split(/\s+/).filter(Boolean)
    if (words.length >= 2) {
      let best: string[] = []
      let widest = Infinity
      for (let i = 1; i < words.length; i++) {
        const lines = [words.slice(0, i).join(' '), words.slice(i).join(' ')]
        const w = Math.max(width100(spec, lines[0]), width100(spec, lines[1]))
        if (w < widest) {
          best = lines
          widest = w
        }
      }
      const two = fitSize(spec, width, widest)
      if (two >= floor) return { lines: best, size: two, over: !covered }
      return { lines: best, size: floor, over: true }
    }
  }
  return { lines: [text], size: floor, over: true }
}

/** Hangul syllables the slot holds at its floor across its lines (CLIP-116). */
export function clipRegionSlotBudget(spec: ClipSlotSpec, width: number) {
  const m = faceOf(spec.face, spec.weight)
  if (!m) return 0
  let syllable = m.hangul
  if (syllable <= 0)
    for (const [c, w] of Object.entries(m.glyphs))
      if ([...c].length === 1 && isHangul(c) && w > syllable) syllable = w
  let floor = Math.min(spec.floor, spec.size)
  if (floor <= 0) floor = spec.size
  if (syllable <= 0 || floor <= 0) return 0
  const track = spec.tracking * UNITS
  const perLine = Math.floor(((width * UNITS) / floor + track) / (syllable + track))
  return Math.max(0, perLine) * maxLines(spec)
}

export function clipRegionPreset(kind: ClipRegionKind, id: string): RegionPreset | undefined {
  const choices = design.regions[kind] as Record<string, RegionPreset>
  return choices[id]
}

/** The preset's slots in outline order, a chip row's or list's slots in place. */
export function clipRegionSlots(kind: ClipRegionKind, id: string): ClipRegionSlotSpec[] {
  const preset = clipRegionPreset(kind, id)
  return preset ? presetSlots(preset) : []
}

/** The preset's own measure; a left-set block's stops at the safe area's right edge
 *  (CDS-9, CDS-79). */
function presetMeasure(preset: RegionPreset, ratio: ClipRegionRatio) {
  const layout = design.ratios[ratio]
  let w = preset.width && preset.width > 0 ? preset.width : layout.copy_max_width
  if (preset.anchor.x === 'left') {
    w = Math.min(
      w,
      layout.safe.x + layout.safe.width - layout.anchor.left - (preset.anchor.inset ?? 0),
    )
  }
  return w
}

/** The width slot `index` fits and whether it is held to one line (Go `SlotWidth`):
 *  the slot's own measure or the preset's, less what its decoration or group takes. */
function slotWidth(preset: RegionPreset, ratio: ClipRegionRatio, index: number) {
  const measure = presetMeasure(preset, ratio)
  let i = 0
  for (const it of preset.items) {
    if ('slot' in it) {
      if (i++ !== index) continue
      const slot = it.slot
      if (preset.stamp && (slot.place === 'upper' || slot.place === 'lower')) {
        return { width: preset.stamp.arc_len, one: true }
      }
      let w = slot.width && slot.width > 0 ? slot.width : measure
      const d = slot.decor
      if (d?.flank) w -= 2 * (d.flank.w + d.flank.gap)
      else if (d?.dot) w -= 2 * d.dot.r + d.dot.gap
      else if (d?.frame) w -= 2 * d.frame.pad_h
      else if (d?.plate) w -= 2 * d.plate.pad_h
      return { width: w, one: false }
    }
    if ('chips' in it || 'list' in it) {
      const count = 'chips' in it ? it.chips.slots.length : it.list.slots.length
      if (index < i + count) {
        const w =
          'chips' in it
            ? measure - 2 * it.chips.pill.pad_h
            : measure - it.list.square.size - it.list.square.gap
        return { width: w, one: true }
      }
      i += count
    }
  }
  return { width: measure, one: false }
}

/** The slot's effective type on a ratio (CDS-19, CDS-46). */
export function clipRegionSlotType(slot: ClipRegionSlotSpec, ratio: ClipRegionRatio) {
  const role = design.type[slot.role as RoleName]
  let size = slot.role === 'hook' ? design.ratios[ratio].hook_size : role.size
  if (slot.size && slot.size > 0) size = slot.size
  let floor = slot.floor && slot.floor > 0 ? slot.floor : role.floor
  if (floor <= 0 || floor > size) floor = size
  return {
    size,
    floor,
    face: slot.face || role.face,
    weight: slot.weight && slot.weight > 0 ? slot.weight : role.weight,
    tracking: slot.tracking ?? role.tracking,
    lineHeight: role.line_height,
  }
}

type Box = { x: number; y: number; width: number; height: number }
const EMPTY: Box = { x: 0, y: 0, width: 0, height: 0 }
const WHITE = design.color.text_white.hex
// The renderer's own pill and stamp fill (CDS-97, CDS-99), mirrored so the preview draws it.
const BLACK = '#000000' // style-escape: region-v2 decoration paint mirrored from the Go layout, not a UI colour

function union(a: Box, b: Box): Box {
  if (a.width === 0 && a.height === 0) return b
  const x0 = Math.min(a.x, b.x)
  const y0 = Math.min(a.y, b.y)
  const x1 = Math.max(a.x + a.width, b.x + b.width)
  const y1 = Math.max(a.y + a.height, b.y + b.height)
  return { x: x0, y: y0, width: x1 - x0, height: y1 - y0 }
}

function inkOf(face: string, weight: number) {
  return faceOf(face, weight)?.ink ?? { top: 1, bottom: 0 }
}

/** What the slot's fit needs (Go `Spec`); an arc, chip or list slot is one line. */
function slotSpec(slot: ClipRegionSlotSpec, ratio: ClipRegionRatio, one = false): ClipSlotSpec {
  const t = clipRegionSlotType(slot, ratio)
  const arc = slot.place === 'upper' || slot.place === 'lower'
  return {
    role: slot.role,
    size: t.size,
    floor: t.floor,
    face: t.face,
    weight: t.weight,
    tracking: t.tracking,
    lines: arc || one ? 1 : (slot.lines ?? 0),
  }
}

/** The circle an arc line runs along: the upper half, or the lower half read upright. */
export type ClipRegionArc = { cx: number; cy: number; r: number; lower: boolean }
/** One drawn line: `x` is its centre for a centred line and its left edge for a left one. */
export type ClipRegionLine = {
  text: string
  size: number
  baseline: number
  width: number
  x: number
  align: 'centre' | 'left'
  arc?: ClipRegionArc
  rotated: boolean
}
export type ClipPlacedRegionSlot = {
  index: number
  spec: ClipRegionSlotSpec
  type: ReturnType<typeof clipRegionSlotType>
  lines: ClipRegionLine[]
  over: boolean
  box: Box
}
export type ClipPlacedRegionRule = { kind: string; alpha: number; box: Box }
/** Neutral decoration (CDS-88) bound to a slot, or to the block itself (slot -1). */
export type ClipRegionShape = {
  kind: string
  slot: number
  box: Box
  radius: number
  circle: boolean
  fill: string
  fillAlpha: number
  stroke: string
  strokeAlpha: number
  strokeWidth: number
  shadow: boolean
  rotated: boolean
}
export type ClipRegionLayout = {
  anchorX: number
  align: 'centre' | 'left'
  width: number
  slots: ClipPlacedRegionSlot[]
  rules: ClipPlacedRegionRule[]
  shapes: ClipRegionShape[]
  rotate: { deg: number; cx: number; cy: number }
  scrim: string
  bounds: Box
  over: boolean
}

function shape(
  kind: string,
  slot: number,
  box: Box,
  rest: Partial<ClipRegionShape> = {},
): ClipRegionShape {
  return {
    kind,
    slot,
    box,
    radius: 0,
    circle: false,
    fill: '',
    fillAlpha: 0,
    stroke: '',
    strokeAlpha: 0,
    strokeWidth: 0,
    shadow: false,
    rotated: false,
    ...rest,
  }
}

/** One stacked slot fitted, with its height including a frame's or plate's padding. */
function fitStackSlot(
  spec: ClipRegionSlotSpec,
  ratio: ClipRegionRatio,
  index: number,
  text: string,
  width: number,
  one = false,
) {
  const t = clipRegionSlotType(spec, ratio)
  const fit = clipFitRegionSlot(slotSpec(spec, ratio, one), text, width)
  const type = { ...t, size: fit.size }
  const slot: ClipPlacedRegionSlot = {
    index,
    spec,
    type,
    over: fit.over,
    lines: fit.lines.map((line) => ({
      text: line,
      size: fit.size,
      baseline: 0,
      width: clipTextWidth(type.face, type.weight, type.tracking, fit.size, line),
      x: 0,
      align: 'centre' as const,
      rotated: false,
    })),
    box: EMPTY,
  }
  const ink = inkOf(type.face, type.weight)
  const height =
    ink.top * fit.size + (fit.lines.length - 1) * fit.size * type.lineHeight + ink.bottom * fit.size
  const d = spec.decor
  const pad = d?.frame ? d.frame.pad_v : d?.plate ? d.plate.pad_v : 0
  return { slot, height: height + 2 * pad, pad }
}

/** A stacked slot's line positions and its own decoration (Go `decorate`). */
function decorate(slot: ClipPlacedRegionSlot, out: ClipRegionLayout, top: number, height: number) {
  for (const line of slot.lines) {
    line.x = out.anchorX
    line.align = out.align
  }
  const d = slot.spec.decor
  if (!d) return []
  const widest = slot.box.width
  const mid = slot.box.y + slot.box.height * 0.52
  if (d.flank) {
    const w = d.flank
    return [
      shape(
        'flank',
        slot.index,
        { x: out.anchorX - widest / 2 - w.gap - w.w, y: mid - 1, width: w.w, height: 2 },
        { fill: WHITE, fillAlpha: w.alpha },
      ),
      shape(
        'flank',
        slot.index,
        { x: out.anchorX + widest / 2 + w.gap, y: mid - 1, width: w.w, height: 2 },
        { fill: WHITE, fillAlpha: w.alpha },
      ),
    ]
  }
  if (d.dot) {
    const r = d.dot.r
    for (const line of slot.lines) line.x = out.anchorX + 2 * r + d.dot.gap
    slot.box = { ...slot.box, x: out.anchorX, width: 2 * r + d.dot.gap + widest }
    const centre = slot.box.y + slot.box.height / 2
    return [
      shape(
        'dot',
        slot.index,
        { x: out.anchorX, y: centre - r, width: 2 * r, height: 2 * r },
        { circle: true, radius: r, fill: WHITE, fillAlpha: 1, shadow: true },
      ),
    ]
  }
  if (d.frame) {
    const f = d.frame
    const w = Math.max(widest + 2 * f.pad_h, f.min_w)
    return [
      shape(
        'frame',
        slot.index,
        { x: out.anchorX - w / 2, y: top, width: w, height },
        { stroke: WHITE, strokeAlpha: f.alpha, strokeWidth: f.stroke, shadow: true },
      ),
    ]
  }
  if (d.plate) {
    const p = d.plate
    const w = widest + 2 * p.pad_h
    const plate = design.color.badge_ad
    return [
      shape(
        'plate',
        slot.index,
        { x: out.anchorX - w / 2, y: top, width: w, height },
        { radius: p.radius, fill: plate.hex, fillAlpha: plate.alpha },
      ),
    ]
  }
  return []
}

type Group = {
  height: number
  place: (top: number) => { slots: ClipPlacedRegionSlot[]; shapes: ClipRegionShape[] }
  over: boolean
}

/** Chips in centred rows that start anew when the next chip does not fit (CDS-97). */
function layoutChips(
  c: RegionChips,
  ratio: ClipRegionRatio,
  first: number,
  row: (i: number) => string,
  out: ClipRegionLayout,
  width: number,
): Group | undefined {
  const chips: { slot: ClipPlacedRegionSlot; w: number; h: number }[] = []
  let over = false
  c.slots.forEach((spec, j) => {
    const text = row(first + j)
    if (text.trim() === '') return
    const { slot, height } = fitStackSlot(spec, ratio, first + j, text, width, true)
    over = over || slot.over
    chips.push({ slot, w: slot.lines[0].width + 2 * c.pill.pad_h, h: height + 2 * c.pill.pad_v })
  })
  if (!chips.length) return undefined
  const rows: (typeof chips)[] = []
  let cur: typeof chips = []
  let used = 0
  for (const ch of chips) {
    if (cur.length > 0 && used + c.gap + ch.w > out.width) {
      rows.push(cur)
      cur = []
      used = 0
    }
    if (cur.length > 0) used += c.gap
    cur.push(ch)
    used += ch.w
  }
  rows.push(cur)
  const rowHeight = Math.max(0, ...chips.map((ch) => ch.h))
  return {
    over,
    height: rows.length * rowHeight + (rows.length - 1) * c.row_gap,
    place: (top) => {
      const slots: ClipPlacedRegionSlot[] = []
      const shapes: ClipRegionShape[] = []
      rows.forEach((line, r) => {
        let w = 0
        line.forEach((ch, i) => (w += (i > 0 ? c.gap : 0) + ch.w))
        let x = out.anchorX - w / 2
        const y = top + r * (rowHeight + c.row_gap)
        for (const ch of line) {
          const t = ch.slot.type
          const ink = inkOf(t.face, t.weight)
          ch.slot.lines[0].baseline = y + c.pill.pad_v + ink.top * t.size
          ch.slot.lines[0].x = x + ch.w / 2
          ch.slot.lines[0].align = 'centre'
          ch.slot.box = {
            x: x + c.pill.pad_h,
            y: y + c.pill.pad_v,
            width: ch.slot.lines[0].width,
            height: rowHeight - 2 * c.pill.pad_v,
          }
          shapes.push(
            shape(
              'pill',
              ch.slot.index,
              { x, y, width: ch.w, height: rowHeight },
              {
                radius: rowHeight / 2,
                fill: BLACK,
                fillAlpha: c.pill.fill_alpha,
                stroke: WHITE,
                strokeAlpha: c.pill.alpha,
                strokeWidth: c.pill.stroke,
              },
            ),
          )
          slots.push(ch.slot)
          x += ch.w + c.gap
        }
      })
      return { slots, shapes }
    },
  }
}

/** List lines left-aligned as one centred group at their smallest fitted size (CDS-98). */
function layoutList(
  l: RegionList,
  ratio: ClipRegionRatio,
  first: number,
  row: (i: number) => string,
  out: ClipRegionLayout,
  width: number,
): Group | undefined {
  const items: ClipPlacedRegionSlot[] = []
  let size = Infinity
  let over = false
  l.slots.forEach((spec, j) => {
    const text = row(first + j)
    if (text.trim() === '') return
    const { slot } = fitStackSlot(spec, ratio, first + j, text, width, true)
    over = over || slot.over
    size = Math.min(size, slot.type.size)
    items.push(slot)
  })
  if (!items.length) return undefined
  let widest = 0
  let rowHeight = 0
  for (const slot of items) {
    slot.type = { ...slot.type, size }
    const t = slot.type
    slot.lines[0].size = size
    slot.lines[0].width = clipTextWidth(t.face, t.weight, t.tracking, size, slot.lines[0].text)
    widest = Math.max(widest, slot.lines[0].width)
    const ink = inkOf(t.face, t.weight)
    rowHeight = ink.top * size + ink.bottom * size
  }
  const groupW = l.square.size + l.square.gap + widest
  return {
    over,
    height: items.length * rowHeight + (items.length - 1) * l.gap,
    place: (top) => {
      const x = out.anchorX - groupW / 2
      const shapes: ClipRegionShape[] = []
      items.forEach((slot, r) => {
        const y = top + r * (rowHeight + l.gap)
        const ink = inkOf(slot.type.face, slot.type.weight)
        slot.lines[0].baseline = y + ink.top * size
        slot.lines[0].x = x + l.square.size + l.square.gap
        slot.lines[0].align = 'left'
        slot.box = { x: slot.lines[0].x, y, width: slot.lines[0].width, height: rowHeight }
        const mid = y + rowHeight / 2
        shapes.push(
          shape(
            'square',
            slot.index,
            { x, y: mid - l.square.size / 2, width: l.square.size, height: l.square.size },
            { radius: 2, fill: WHITE, fillAlpha: 1, shadow: true },
          ),
        )
      })
      return { slots: items, shapes }
    },
  }
}

/** The four stamp slots around two rings (CDS-99): rings, arcs and the inside turn
 *  together, the slot below does not. */
function layoutStamp(
  out: ClipRegionLayout,
  preset: RegionPreset,
  ratio: ClipRegionRatio,
  row: (i: number) => string,
) {
  const st = preset.stamp!
  const layout = design.ratios[ratio]
  const cy = ((preset.anchor.y ?? 0) * layout.canvas.height) / design.ratios.vertical.canvas.height
  const cx = out.anchorX
  let drawn = false
  presetSlots(preset).forEach((spec, i) => {
    const text = row(i)
    if (text.trim() === '') return
    drawn = true
    const t = clipRegionSlotType(spec, ratio)
    const fit = clipFitRegionSlot(slotSpec(spec, ratio), text, slotWidth(preset, ratio, i).width)
    const type = { ...t, size: fit.size }
    const ink = inkOf(type.face, type.weight)
    const slot: ClipPlacedRegionSlot = {
      index: i,
      spec,
      type,
      over: fit.over,
      box: EMPTY,
      lines: fit.lines.map((line) => ({
        text: line,
        size: fit.size,
        baseline: 0,
        width: clipTextWidth(type.face, type.weight, type.tracking, fit.size, line),
        x: cx,
        align: 'centre' as const,
        rotated: spec.place !== 'below',
      })),
    }
    const h =
      ink.top * type.size +
      (fit.lines.length - 1) * type.size * type.lineHeight +
      ink.bottom * type.size
    const stackAt = (top: number) => {
      let widest = 0
      slot.lines.forEach((line, k) => {
        line.baseline = top + ink.top * type.size + k * type.size * type.lineHeight
        widest = Math.max(widest, line.width)
      })
      slot.box = { x: cx - widest / 2, y: top, width: widest, height: h }
    }
    if (spec.place === 'upper' || spec.place === 'lower') {
      const lower = spec.place === 'lower'
      const r = lower ? st.r_outer - 14 : st.r_inner + 14
      const line = slot.lines[0]
      line.arc = { cx, cy, r, lower }
      if (!lower) {
        line.baseline = cy - r
        const reach = r + ink.top * type.size
        slot.box = { x: cx - reach, y: cy - reach, width: 2 * reach, height: reach }
      } else {
        line.baseline = cy + r
        slot.box = { x: cx - r, y: cy, width: 2 * r, height: r }
      }
    } else if (spec.place === 'centre') {
      stackAt(cy - h / 2)
    } else {
      stackAt(cy + st.r_outer + st.below_gap)
    }
    out.slots.push(slot)
    out.bounds = union(out.bounds, slot.box)
    out.over = out.over || fit.over
  })
  if (!drawn) return out
  const ring = (r: number, width: number, fill: number) =>
    shape(
      'ring',
      -1,
      { x: cx - r, y: cy - r, width: 2 * r, height: 2 * r },
      {
        circle: true,
        radius: r,
        stroke: WHITE,
        strokeAlpha: 1,
        strokeWidth: width,
        rotated: true,
        shadow: true,
        ...(fill > 0 ? { fill: BLACK, fillAlpha: fill } : {}),
      },
    )
  out.shapes.push(
    ring(st.r_outer, st.stroke_outer, st.fill_alpha),
    ring(st.r_inner, st.stroke_inner, 0),
  )
  const mid = (st.r_outer + st.r_inner) / 2
  for (const x of [cx - mid, cx + mid]) {
    out.shapes.push(
      shape(
        'ring_dot',
        -1,
        { x: x - st.dot_r, y: cy - st.dot_r, width: 2 * st.dot_r, height: 2 * st.dot_r },
        { circle: true, radius: st.dot_r, fill: WHITE, fillAlpha: 1, rotated: true },
      ),
    )
  }
  for (const sh of out.shapes) out.bounds = union(out.bounds, sh.box)
  out.rotate = { deg: st.rotate, cx, cy }
  return out
}

function presetSlots(preset: RegionPreset): ClipRegionSlotSpec[] {
  return preset.items.flatMap((it) =>
    'slot' in it ? [it.slot] : 'chips' in it ? it.chips.slots : 'list' in it ? it.list.slots : [],
  )
}

/** One region block laid out for its rows, indexed by slot (Go `LayoutRegion`). */
export function clipLayoutRegion(
  kind: ClipRegionKind,
  id: string,
  ratio: ClipRegionRatio,
  rows: readonly string[],
): ClipRegionLayout | undefined {
  const preset = clipRegionPreset(kind, id)
  const layout = design.ratios[ratio]
  if (!preset || !layout) return undefined
  const left = preset.anchor.x === 'left'
  const out: ClipRegionLayout = {
    width: presetMeasure(preset, ratio),
    align: left ? 'left' : 'centre',
    anchorX: left ? layout.anchor.left + (preset.anchor.inset ?? 0) : layout.anchor.center,
    slots: [],
    rules: [],
    shapes: [],
    rotate: { deg: 0, cx: 0, cy: 0 },
    scrim:
      preset.scrim ||
      (preset.anchor.kind === 'top' || preset.anchor.kind === 'bottom'
        ? preset.anchor.kind
        : 'radial'),
    bounds: EMPTY,
    over: false,
  }
  const row = (i: number) => (i < rows.length ? rows[i] : '')
  if (preset.stamp) return layoutStamp(out, preset, ratio, row)
  type Step = {
    slot?: ClipPlacedRegionSlot
    rule?: ClipPlacedRegionRule
    group?: Group
    height: number
    gap: number
    pad: number
  }
  const steps: Step[] = []
  let pending = 0
  let index = 0
  let drawn = false
  for (const it of preset.items) {
    if ('gap' in it) {
      if (it.gap > 0) pending = it.gap
    } else if ('slot' in it) {
      const i = index++
      const text = row(i)
      if (text.trim() === '') {
        pending = 0
        continue
      }
      const fitted = fitStackSlot(it.slot, ratio, i, text, slotWidth(preset, ratio, i).width)
      steps.push({ slot: fitted.slot, height: fitted.height, gap: pending, pad: fitted.pad })
      pending = 0
      drawn = true
      out.over = out.over || fitted.slot.over
    } else if ('rule' in it) {
      const token = design.rule[it.rule.kind as keyof typeof design.rule]
      const w = it.rule.w && it.rule.w > 0 ? it.rule.w : token.w
      steps.push({
        rule: {
          kind: it.rule.kind,
          alpha: it.rule.alpha ?? token.alpha,
          box: { x: 0, y: 0, width: w, height: token.h },
        },
        height: token.h,
        gap: pending,
        pad: 0,
      })
      pending = 0
    } else {
      const first = index
      const count = 'chips' in it ? it.chips.slots.length : it.list.slots.length
      index += count
      const width = slotWidth(preset, ratio, first).width
      const group =
        'chips' in it
          ? layoutChips(it.chips, ratio, first, row, out, width)
          : layoutList(it.list, ratio, first, row, out, width)
      if (!group) {
        pending = 0
        continue
      }
      steps.push({ group, height: group.height, gap: pending, pad: 0 })
      pending = 0
      drawn = true
      out.over = out.over || group.over
    }
  }
  if (!drawn) return out
  let total = 0
  steps.forEach((s, i) => {
    if (i > 0) total += s.gap
    total += s.height
  })
  const scale = layout.canvas.height / design.ratios.vertical.canvas.height
  const y0 = preset.anchor.y ?? 0
  const top =
    preset.anchor.kind === 'top'
      ? y0 * scale
      : preset.anchor.kind === 'bottom'
        ? layout.anchor.bottom - total
        : y0 * scale - total / 2
  const leftOf = (w: number) => (out.align === 'left' ? out.anchorX : out.anchorX - w / 2)
  let y = top
  steps.forEach((s, i) => {
    if (i > 0) y += s.gap
    if (s.slot) {
      const t = s.slot.type
      const ink = inkOf(t.face, t.weight)
      let widest = 0
      s.slot.lines.forEach((line, k) => {
        line.baseline = y + s.pad + ink.top * t.size + k * t.size * t.lineHeight
        widest = Math.max(widest, line.width)
      })
      s.slot.box = { x: leftOf(widest), y: y + s.pad, width: widest, height: s.height - 2 * s.pad }
      const shapes = decorate(s.slot, out, y, s.height)
      out.slots.push(s.slot)
      out.shapes.push(...shapes)
      out.bounds = union(out.bounds, s.slot.box)
      for (const sh of shapes) out.bounds = union(out.bounds, sh.box)
    } else if (s.group) {
      const placed = s.group.place(y)
      for (const slot of placed.slots) {
        out.slots.push(slot)
        out.bounds = union(out.bounds, slot.box)
      }
      for (const sh of placed.shapes) {
        out.shapes.push(sh)
        out.bounds = union(out.bounds, sh.box)
      }
    } else if (s.rule) {
      s.rule.box = { ...s.rule.box, x: leftOf(s.rule.box.width), y }
      out.rules.push(s.rule)
      out.bounds = union(out.bounds, s.rule.box)
    }
    y += s.height
  })
  if (preset.side_bar) {
    const bar = shape(
      'side_bar',
      -1,
      { x: layout.anchor.left, y: top, width: preset.side_bar.w, height: total },
      { fill: WHITE, fillAlpha: 1 },
    )
    out.shapes.push(bar)
    out.bounds = union(out.bounds, bar.box)
  }
  if (preset.rotate) {
    const cx = out.align === 'left' ? out.bounds.x + out.bounds.width / 2 : out.anchorX
    out.rotate = { deg: preset.rotate, cx, cy: top + total / 2 }
    for (const slot of out.slots) for (const line of slot.lines) line.rotated = true
    for (const sh of out.shapes) sh.rotated = true
  }
  return out
}

/** One preset slot's fit spec and the width it fits on a ratio (Go `RegionSlotAt`). */
export function clipRegionSlotAt(
  kind: ClipRegionKind,
  id: string,
  ratio: ClipRegionRatio,
  index: number,
): { spec: ClipSlotSpec; width: number } | undefined {
  const preset = clipRegionPreset(kind, id)
  const slot = clipRegionSlots(kind, id)[index]
  if (!preset || !slot) return undefined
  const { width, one } = slotWidth(preset, ratio, index)
  return { spec: slotSpec(slot, ratio, one), width }
}
