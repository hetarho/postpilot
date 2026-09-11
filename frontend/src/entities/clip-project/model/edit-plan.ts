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
  return { ...plan, cuts: plan.cuts.map((c) => ({ ...c, copy: { ...c.copy } })) }
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
          : { ...c, copy: { ...c.copy, ...edit.patch } },
    )
  next.durationMs =
    next.cuts.reduce((sum, c) => sum + c.endMs - c.startMs, 0) -
    fadeMs * Math.max(0, next.cuts.length - 1)
  return next
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
      text: Array.from(c.copy.text).length > state.maxCopyRunes,
      copyStart:
        !integer(c.copy.startMs) ||
        c.copy.startMs < 0 ||
        (!whole && c.copy.startMs >= c.copy.endMs),
      copyEnd:
        !integer(c.copy.endMs) ||
        (!whole && (c.copy.endMs <= c.copy.startMs || c.copy.endMs > duration)),
      anchor: !COPY_ANCHORS.includes(c.copy.anchor) || !COPY_ALIGNS.includes(c.copy.align),
      style: !COPY_STYLES.includes(c.copy.style) || !state.copyStyles.includes(c.copy.style),
      accent: !CLIP_ACCENTS.includes(c.copy.accent),
      volume: !integer(c.volumePermille) || c.volumePermille < 0 || c.volumePermille > 1000,
    }
  })
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
    valid: !count && !timeline && cuts.every((c) => Object.values(c).every((v) => !v)),
  }
}
