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
  stroke_width?: number
  outline?: number
  width?: number
  place?: string
  decor?: {
    flank?: { w: number; gap: number }
    dot?: { r: number; gap: number }
    frame?: { pad_h: number }
    plate?: { pad_h: number }
  }
}
type RegionItem =
  | { slot: ClipRegionSlotSpec }
  | { rule: { kind: string; w?: number; alpha?: number } }
  | { gap: number }
  | { chips: { pill: { pad_h: number }; slots: ClipRegionSlotSpec[] } }
  | { list: { square: { size: number; gap: number }; slots: ClipRegionSlotSpec[] } }
type RegionPreset = {
  anchor: { kind: string; y?: number; x?: string; inset?: number }
  width?: number
  rotate?: number
  side_bar?: unknown
  stamp?: { arc_len: number }
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
  return (clipRegionPreset(kind, id)?.items ?? []).flatMap((it) =>
    'slot' in it ? [it.slot] : 'chips' in it ? it.chips.slots : 'list' in it ? it.list.slots : [],
  )
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

/** Whether this port draws the preset: slots, rules and gaps with plain paint. Rotation,
 *  groups, stamps, bars and slot decoration are the renderer's alone. */
function portable(preset: RegionPreset) {
  return (
    !preset.rotate &&
    !preset.side_bar &&
    !preset.stamp &&
    preset.items.every(
      (it) =>
        !('chips' in it) &&
        !('list' in it) &&
        (!('slot' in it) ||
          (!it.slot.decor && !it.slot.outline && !it.slot.stroke_width && !it.slot.width)),
    )
  )
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

export type ClipRegionLine = { text: string; size: number; baseline: number; width: number }
export type ClipPlacedRegionSlot = {
  index: number
  spec: ClipRegionSlotSpec
  type: ReturnType<typeof clipRegionSlotType>
  lines: ClipRegionLine[]
  over: boolean
  box: { x: number; y: number; width: number; height: number }
}
export type ClipPlacedRegionRule = {
  kind: string
  alpha: number
  box: { x: number; y: number; width: number; height: number }
}
export type ClipRegionLayout = {
  anchorX: number
  align: 'centre' | 'left'
  width: number
  slots: ClipPlacedRegionSlot[]
  rules: ClipPlacedRegionRule[]
  over: boolean
}

/** One region block laid out for its rows, indexed by slot (Go `LayoutRegion`), or
 *  undefined for a preset only the renderer draws. */
export function clipLayoutRegion(
  kind: ClipRegionKind,
  id: string,
  ratio: ClipRegionRatio,
  rows: readonly string[],
): ClipRegionLayout | undefined {
  const preset = clipRegionPreset(kind, id)
  const layout = design.ratios[ratio]
  if (!preset || !layout || !portable(preset)) return undefined
  const out: ClipRegionLayout = {
    width: presetMeasure(preset, ratio),
    align: preset.anchor.x === 'left' ? 'left' : 'centre',
    anchorX:
      preset.anchor.x === 'left'
        ? layout.anchor.left + (preset.anchor.inset ?? 0)
        : layout.anchor.center,
    slots: [],
    rules: [],
    over: false,
  }
  type Step = {
    slot?: ClipPlacedRegionSlot
    rule?: ClipPlacedRegionRule
    height: number
    gap: number
  }
  const steps: Step[] = []
  let pending = 0
  let index = 0
  let drawn = false
  for (const it of preset.items) {
    if ('gap' in it) {
      if (it.gap > 0) pending = it.gap
    } else if ('slot' in it) {
      const text = index < rows.length ? rows[index] : ''
      const i = index++
      if (text.trim() === '') {
        pending = 0
        continue
      }
      const type = clipRegionSlotType(it.slot, ratio)
      const fit = clipFitRegionSlot(
        { role: it.slot.role, ...type, lines: it.slot.lines ?? 0 },
        text,
        out.width,
      )
      const fitted = { ...type, size: fit.size }
      const ink = faceOf(fitted.face, fitted.weight)?.ink ?? { top: 1, bottom: 0 }
      const height =
        ink.top * fit.size +
        (fit.lines.length - 1) * fit.size * fitted.lineHeight +
        ink.bottom * fit.size
      steps.push({
        slot: {
          index: i,
          spec: it.slot,
          type: fitted,
          over: fit.over,
          lines: fit.lines.map((line) => ({
            text: line,
            size: fit.size,
            baseline: 0,
            width: clipTextWidth(fitted.face, fitted.weight, fitted.tracking, fit.size, line),
          })),
          box: { x: 0, y: 0, width: 0, height: 0 },
        },
        height,
        gap: pending,
      })
      pending = 0
      drawn = true
      out.over = out.over || fit.over
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
      })
      pending = 0
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
  let top =
    preset.anchor.kind === 'top'
      ? y0 * scale
      : preset.anchor.kind === 'bottom'
        ? layout.anchor.bottom - total
        : y0 * scale - total / 2
  const left = (w: number) => (out.align === 'left' ? out.anchorX : out.anchorX - w / 2)
  steps.forEach((s, i) => {
    if (i > 0) top += s.gap
    if (s.slot) {
      const t = s.slot.type
      const ink = faceOf(t.face, t.weight)?.ink ?? { top: 1, bottom: 0 }
      let widest = 0
      s.slot.lines.forEach((line, k) => {
        line.baseline = top + ink.top * t.size + k * t.size * t.lineHeight
        widest = Math.max(widest, line.width)
      })
      s.slot.box = { x: left(widest), y: top, width: widest, height: s.height }
      out.slots.push(s.slot)
    } else if (s.rule) {
      s.rule.box.x = left(s.rule.box.width)
      s.rule.box.y = top
      out.rules.push(s.rule)
    }
    top += s.height
  })
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
  const type = clipRegionSlotType(slot, ratio)
  const { width, one } = slotWidth(preset, ratio, index)
  return { spec: { role: slot.role, ...type, lines: one ? 1 : (slot.lines ?? 0) }, width }
}
