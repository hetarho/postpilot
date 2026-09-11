import { CLIP_FACTS, CLIP_GUARDS, CLIP_TIMING, clipStyle } from '@/shared/config'
import {
  CLIP_ACCENTS,
  COPY_STYLES,
  type ClipAccent,
  type CopyStyle,
} from '@/entities/clip-template/@x/clip-project'

/** A caption's VERTICAL anchor (CDS-12): the proto field is still called
 *  `position`, and its three former values top/center/bottom are gone. */
export const COPY_ANCHORS = ['top', 'upper_mid', 'lower_mid', 'bottom'] as const
/** Horizontal alignment against that anchor. */
export const COPY_ALIGNS = ['center', 'left', 'right'] as const
export interface ClipCaption {
  text: string
  anchor: (typeof COPY_ANCHORS)[number]
  align: (typeof COPY_ALIGNS)[number]
  /** The one word 크게 강조 colours and 형광펜 highlights; a substring of `text`. */
  keyword: string
  style: CopyStyle
  accent: ClipAccent
  startMs: number
  endMs: number
}
export interface ClipEditCut {
  id: string
  sourceId: string
  fingerprint: string
  startMs: number
  endMs: number
  copy: ClipCaption
  /** Reserved fact labels whose chips belong on this cut, at most two. */
  chips: string[]
  volumePermille: number
}
export interface ClipEditPlan {
  durationMs: number
  cuts: ClipEditCut[]
}
export interface RetainedClipSource {
  id: string
  fingerprint: string
  filename: string
  durationMs: number
  width: number
  height: number
}
export interface ClipEditingState {
  plan: ClipEditPlan
  sources: RetainedClipSource[]
  copyStyles: CopyStyle[]
  fadeMs: number
  maxCuts: number
  maxCopyRunes: number
  minDurationMs: number
  maxDurationMs: number
}
export function copyClipPlan(plan: ClipEditPlan): ClipEditPlan {
  return {
    ...plan,
    cuts: plan.cuts.map((c) => ({ ...c, chips: [...c.chips], copy: { ...c.copy } })),
  }
}
export type ClipEdit =
  | { type: 'move'; from: number; to: number }
  | { type: 'remove'; id: string }
  | {
      type: 'cut'
      id: string
      patch: Partial<Pick<ClipEditCut, 'startMs' | 'endMs' | 'volumePermille'>>
    }
  | { type: 'copy'; id: string; patch: Partial<ClipCaption> }
  | { type: 'chips'; id: string; chips: string[] }
export function editClipPlan(plan: ClipEditPlan, edit: ClipEdit, fadeMs: number): ClipEditPlan {
  const next = copyClipPlan(plan)
  if (edit.type === 'move') {
    if (
      !Number.isInteger(edit.from) ||
      !Number.isInteger(edit.to) ||
      edit.from < 0 ||
      edit.to < 0 ||
      edit.from >= next.cuts.length ||
      edit.to >= next.cuts.length
    )
      return plan
    const [cut] = next.cuts.splice(edit.from, 1)
    next.cuts.splice(edit.to, 0, cut!)
  } else if (edit.type === 'remove') next.cuts = next.cuts.filter((c) => c.id !== edit.id)
  else
    next.cuts = next.cuts.map((c) =>
      c.id !== edit.id
        ? c
        : edit.type === 'cut'
          ? { ...c, ...edit.patch }
          : edit.type === 'chips'
            ? { ...c, chips: [...edit.chips] }
            : { ...c, copy: { ...c.copy, ...edit.patch } },
    )
  next.durationMs =
    next.cuts.reduce((sum, c) => sum + c.endMs - c.startMs, 0) -
    fadeMs * Math.max(0, next.cuts.length - 1)
  return next
}
/** Korean syllables, spaces and punctuation excluded — the same count the
 *  design system makes (CDS's constraints). */
export function copyChars(text: string): number {
  return Array.from(text).filter((c) => !/[\s\p{P}\p{S}]/u.test(c)).length
}
/** CDS-41's minimum exposure for a copy of this length. */
export function minExposureMs(text: string): number {
  return CLIP_TIMING.sub_min_base_ms + CLIP_TIMING.sub_min_per_char_ms * copyChars(text)
}
/** The window a copy actually gets: its own, or CDS-27's default inset. */
function copyWindow(cut: ClipEditCut, state: ClipEditingState) {
  const whole = cut.copy.startMs === 0 && cut.copy.endMs === 0
  const lead = CLIP_TIMING.copy_lead_ms
  const start = whole ? lead : cut.copy.startMs
  const end = whole ? cut.endMs - cut.startMs - lead : cut.copy.endMs
  void state
  return { start, end, length: end - start }
}
/** The style's own line and character limits (CDS-20, CDS-23..26). */
function withinStyleLimits(text: string, style: ClipCaption['style']): boolean {
  const rule = clipStyle(style)
  const lines = text.split('\n')
  return lines.length <= rule.lines && lines.every((line) => copyChars(line) <= rule.chars)
}
/** CDS-38: consecutive cuts move at most one anchor step, measured only between
 *  cuts that share a style — a style change is a deliberate visual change. */
function withinAnchorStep(cuts: readonly ClipEditCut[], cut: ClipEditCut): boolean {
  const placed = cuts.filter((c) => c.copy.text.trim() !== '')
  const index = placed.findIndex((c) => c.id === cut.id)
  if (index <= 0) return true
  const previous = placed[index - 1]!
  if (previous.copy.style !== cut.copy.style) return true
  const order = COPY_ANCHORS as readonly string[]
  return Math.abs(order.indexOf(cut.copy.anchor) - order.indexOf(previous.copy.anchor)) <= 1
}
export function requiredClipSources(plan: ClipEditPlan, sources: readonly RetainedClipSource[]) {
  const needed = new Set(plan.cuts.map((c) => c.fingerprint))
  return sources.filter((s) => needed.has(s.fingerprint))
}
export function validateClipPlan(plan: ClipEditPlan, state: ClipEditingState) {
  const integer = Number.isSafeInteger
  const cuts = plan.cuts.map((c) => {
    const source = state.sources.find((s) => s.id === c.sourceId && s.fingerprint === c.fingerprint)
    const duration = c.endMs - c.startMs
    const whole = c.copy.startMs === 0 && c.copy.endMs === 0
    return {
      identity: !source || !c.id || plan.cuts.filter((v) => v.id === c.id).length !== 1,
      start: !integer(c.startMs) || c.startMs < 0 || c.startMs >= c.endMs,
      end: !integer(c.endMs) || c.endMs > (source?.durationMs ?? 0) || c.endMs <= c.startMs,
      duration: !integer(duration) || duration <= 2 * state.fadeMs,
      // The rune ceiling is the contract's; the per-style line and character
      // limits below are the design system's, and both are mirrored here for
      // immediacy only — the server's verifier stays the authority.
      text:
        Array.from(c.copy.text).length > state.maxCopyRunes ||
        (c.copy.text.trim() !== '' && !withinStyleLimits(c.copy.text, c.copy.style)),
      exposure:
        c.copy.text.trim() !== '' && copyWindow(c, state).length < minExposureMs(c.copy.text),
      chips:
        c.chips.length > 2 ||
        c.chips.some((label) => !(CLIP_FACTS.chips as readonly string[]).includes(label)),
      copyStart:
        !integer(c.copy.startMs) ||
        c.copy.startMs < 0 ||
        (!whole && c.copy.startMs >= c.copy.endMs),
      copyEnd:
        !integer(c.copy.endMs) ||
        (!whole && (c.copy.endMs <= c.copy.startMs || c.copy.endMs > duration)),
      // A cut whose copy the composer dropped carries no placement at all, and
      // that is a valid plan — the cut simply shows its footage.
      anchor:
        c.copy.text.trim() !== '' &&
        (!COPY_ANCHORS.includes(c.copy.anchor) ||
          !COPY_ALIGNS.includes(c.copy.align) ||
          // 메모 is LEFT-aligned at TOP or BOTTOM (CDS-24), and consecutive
          // cuts move at most one anchor step when they share a style (CDS-38).
          (c.copy.style === 'memo' &&
            (c.copy.align !== 'left' || !['top', 'bottom'].includes(c.copy.anchor))) ||
          !withinAnchorStep(plan.cuts, c)),
      keyword: c.copy.keyword !== '' && !c.copy.text.includes(c.copy.keyword),
      style:
        c.copy.text.trim() !== '' &&
        (!COPY_STYLES.includes(c.copy.style) || !state.copyStyles.includes(c.copy.style)),
      accent: !CLIP_ACCENTS.includes(c.copy.accent),
      volume: !integer(c.volumePermille) || c.volumePermille < 0 || c.volumePermille > 1000,
    }
  })
  // CDS-40's per-clip guards: 크게 강조 at most twice, and no style four times
  // in a row.
  const placed = plan.cuts.filter((c) => c.copy.text.trim() !== '')
  const frequency =
    placed.filter((c) => c.copy.style === 'bold').length > CLIP_GUARDS.bold_max ||
    // A run LONGER than run_max is the violation: the fourth consecutive use of
    // one style must have alternated (CDS-40).
    placed.some(
      (cut, i) =>
        i >= CLIP_GUARDS.run_max &&
        placed.slice(i - CLIP_GUARDS.run_max, i + 1).every((v) => v.copy.style === cut.copy.style),
    )
  const duration =
    plan.cuts.reduce((sum, c) => sum + c.endMs - c.startMs, 0) -
    state.fadeMs * Math.max(0, plan.cuts.length - 1)
  const timeline =
    !integer(plan.durationMs) ||
    plan.durationMs !== duration ||
    duration < state.minDurationMs ||
    duration > state.maxDurationMs
  const count = plan.cuts.length === 0 || plan.cuts.length > state.maxCuts
  return {
    cuts,
    timeline,
    count,
    frequency,
    valid:
      !count && !timeline && !frequency && cuts.every((c) => Object.values(c).every((v) => !v)),
  }
}
