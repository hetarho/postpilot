import { CLIP_COMPOSITION_LIMITS, CLIP_RAPID, CLIP_TIMELINE } from '@/shared/config'
import { COPY_STYLES, CLIP_ACCENTS } from '@/entities/clip-template/@x/clip-project'
import {
  copyClipPlan,
  editClipPlan,
  validateClipPlan,
  type ClipEdit,
  type ClipEditPlan,
  type ClipEditableText,
  type ClipEditingState,
} from './edit-plan'
import type { ClipSourceAssociation } from './composition'
import { splitRapid } from './caption-pace'

export type ClipSelection =
  { kind: 'cut'; id: string } | { kind: 'text'; id: string; phrase?: number }
export type TimelineEdit =
  | ClipEdit
  | {
      type: 'text'
      id: string
      patch: Partial<
        Pick<
          ClipEditableText,
          | 'text'
          | 'rows'
          | 'style'
          | 'position'
          | 'align'
          | 'basis'
          | 'startMs'
          | 'endMs'
          | 'pace'
          | 'accent'
          | 'keyword'
          | 'phrases'
          | 'evidenceReviewed'
        >
      >
    }
  | { type: 'removeText'; id: string }
  | { type: 'associations'; associations: ClipSourceAssociation[] }

/** A draft may be invalid. Preserve its coordinates so the owner can repair it. */
export function timelineCuts(plan: ClipEditPlan) {
  let offset = 0
  return plan.cuts.map((cut, index) => {
    const startMs = offset - (index ? cut.transitionMs : 0)
    const endMs = startMs + cut.endMs - cut.startMs
    offset = endMs
    return { cut, index, startMs, endMs }
  })
}

export function textInterval(plan: ClipEditPlan, text: ClipEditableText) {
  const cut = timelineCuts(plan).find((c) => c.cut.id === text.cutId)
  const length = cut ? cut.endMs - cut.startMs : 0
  let startMs = text.startMs ?? text.effectiveStartMs ?? CLIP_COMPOSITION_LIMITS.autoInsetMs
  let endMs = text.endMs ?? text.effectiveEndMs ?? length - CLIP_COMPOSITION_LIMITS.autoInsetMs
  if (text.basis === 'whole') {
    startMs = 0
    endMs = plan.durationMs
  } else if (text.basis === 'output-end') {
    startMs = plan.durationMs + (text.startMs ?? NaN)
    endMs = plan.durationMs + (text.endMs ?? NaN)
  } else if (text.basis === 'output-start') {
    startMs = text.startMs ?? NaN
    endMs = text.endMs ?? NaN
  } else if (text.basis === 'cut') {
    startMs += cut?.startMs ?? NaN
    endMs += cut?.startMs ?? NaN
  } else startMs = endMs = NaN
  const explicit = text.startMs !== undefined || text.endMs !== undefined
  const valid =
    Number.isSafeInteger(startMs) &&
    Number.isSafeInteger(endMs) &&
    startMs >= 0 &&
    endMs > startMs &&
    endMs <= plan.durationMs &&
    (!explicit || (text.startMs !== undefined && text.endMs !== undefined)) &&
    (text.basis !== 'whole' || !explicit) &&
    (text.basis !== 'output-end' || ((text.startMs ?? 1) <= 0 && (text.endMs ?? 1) <= 0)) &&
    (text.basis !== 'cut' || (!!cut && startMs >= cut.startMs && endMs <= cut.endMs))
  return { startMs, endMs, valid, cutOffsetMs: cut?.startMs ?? 0 }
}

export function nativeTextErrors(plan: ClipEditPlan, state: ClipEditingState) {
  return (plan.elements ?? []).map((text) => {
    const interval = textInterval(plan, text)
    const phrases = text.phrases ?? []
    const length = (value: string) => Array.from(value).length
    return {
      id: text.instanceId,
      interval: !interval.valid,
      identity:
        !text.instanceId ||
        (plan.elements ?? []).filter((t) => t.instanceId === text.instanceId).length !== 1,
      text:
        length(text.text) > CLIP_COMPOSITION_LIMITS.copyChars ||
        text.rows.some((r) => length(r.text) > CLIP_COMPOSITION_LIMITS.copyChars),
      style:
        text.style !== 'auto' &&
        (!COPY_STYLES.some((s) => s === text.style) ||
          !state.copyStyles.some((s) => s === text.style)),
      position: !['auto', 'header', 'top', 'upper_mid', 'lower_mid', 'bottom', 'center'].includes(
        text.position,
      ),
      align: !['left', 'center', 'right'].includes(text.align),
      keyword: !!text.keyword && !text.text.includes(text.keyword),
      accent: !CLIP_ACCENTS.some((a) => a === text.accent),
      stale: !!text.staleEvidence && !text.evidenceReviewed,
      phrases:
        phrases.length > CLIP_RAPID.max_per_cut ||
        phrases.some(
          (p, i) =>
            !p.text.trim() ||
            p.text.includes('\n') ||
            length(p.text.replace(/[\s\p{P}\p{S}]/gu, '')) > CLIP_RAPID.max_chars ||
            !Number.isSafeInteger(p.startMs) ||
            !Number.isSafeInteger(p.endMs) ||
            p.endMs - p.startMs < CLIP_RAPID.min_ms ||
            p.endMs - p.startMs > CLIP_RAPID.max_ms ||
            p.startMs + interval.cutOffsetMs < interval.startMs ||
            p.endMs + interval.cutOffsetMs > interval.endMs ||
            (i > 0 && p.startMs < phrases[i - 1].endMs),
        ),
    }
  })
}

export function validateTimelinePlan(plan: ClipEditPlan, state: ClipEditingState) {
  const legacy = validateClipPlan(plan, state)
  if (!plan.nativeComposition)
    return {
      ...legacy,
      saveable: legacy.valid,
      elements: [] as ReturnType<typeof nativeTextErrors>,
    }
  const cuts = legacy.cuts.map((cut) => ({
    ...cut,
    copyCount: false,
    copyClasses: false,
    chips: false,
    copies: [],
  }))
  const elements = nativeTextErrors(plan, state)
  const geometry = timelineCuts(plan)
  const invalidSeam = geometry.some(
    ({ cut }, i) =>
      cut.endMs - cut.startMs <=
      Math.max(400, cut.transitionMs + (geometry[i + 1]?.cut.transitionMs ?? 0)),
  )
  const saveable =
    !legacy.count &&
    !legacy.timeline &&
    !invalidSeam &&
    cuts.every((c) => Object.entries(c).every(([key, value]) => key === 'copies' || !value)) &&
    elements.every((e) =>
      Object.entries(e).every(([key, value]) => key === 'id' || key === 'stale' || !value),
    )
  return {
    ...legacy,
    cuts,
    elements,
    frequency: false,
    hook: false,
    saveable,
    valid: saveable && elements.every((e) => !e.stale),
  }
}

export function associationAffectsText(text: ClipEditableText, changed: ClipSourceAssociation[]) {
  return (
    text.kind === 'ai' &&
    changed.some(
      (a) =>
        (a.groupId === text.groupId && a.itemId === text.itemId) ||
        (text.evidence ?? []).some(
          (e) =>
            e.sourceId === a.sourceId &&
            e.fingerprint === a.fingerprint &&
            e.startMs < a.endMs &&
            e.endMs > a.startMs,
        ),
    )
  )
}

export function applyTimelineEdit(plan: ClipEditPlan, edit: TimelineEdit): ClipEditPlan {
  const next = copyClipPlan(plan)
  if (edit.type === 'text') {
    next.elements = next.elements?.map((text) => {
      if (text.instanceId !== edit.id) return text
      const changedContent =
        edit.patch.text !== undefined ||
        edit.patch.rows !== undefined ||
        edit.patch.phrases !== undefined
      return {
        ...text,
        ...edit.patch,
        ...(Object.keys(edit.patch).some((key) => key !== 'evidenceReviewed')
          ? { effectiveStartMs: undefined, effectiveEndMs: undefined }
          : {}),
        ...(edit.patch.phrases?.length
          ? { text: edit.patch.phrases.map((p) => p.text).join(' ') }
          : {}),
        ...(changedContent ? { staleEvidence: false, evidenceReviewed: true } : {}),
        // Automatic rapid splitting is regenerated for a newly typed sentence.
        ...(edit.patch.text !== undefined && edit.patch.phrases === undefined
          ? { phrases: [] }
          : {}),
      }
    })
  } else if (edit.type === 'removeText') {
    next.elements = next.elements?.filter((text) => text.instanceId !== edit.id)
  } else if (edit.type === 'associations') {
    const before = next.associations ?? []
    const key = (a: ClipSourceAssociation) => JSON.stringify(a)
    const changed = [
      ...before.filter((a) => !edit.associations.some((b) => key(a) === key(b))),
      ...edit.associations.filter((a) => !before.some((b) => key(a) === key(b))),
    ]
    next.associations = edit.associations.map((a) => ({ ...a }))
    next.elements = next.elements?.map((text) =>
      associationAffectsText(text, changed)
        ? { ...text, staleEvidence: true, evidenceReviewed: false }
        : text,
    )
  } else {
    const result = editClipPlan(next, edit)
    if (edit.type === 'remove') {
      result.elements = result.elements?.filter(
        (text) => text.basis !== 'cut' || text.cutId !== edit.id,
      )
    }
    return result
  }
  return next
}

export function clipDraftKey(plan: ClipEditPlan) {
  return JSON.stringify(plan, (key, value: unknown) => {
    if (
      [
        'resolvedStartMs',
        'resolvedEndMs',
        'effectiveStartMs',
        'effectiveEndMs',
        'evidence',
        'fallbackReason',
      ].includes(key)
    )
      return undefined
    return typeof value === 'number' && !Number.isFinite(value) ? String(value) : value
  })
}

export function selectedTime(plan: ClipEditPlan, selection: ClipSelection) {
  if (selection.kind === 'cut')
    return timelineCuts(plan).find((c) => c.cut.id === selection.id)?.startMs ?? 0
  const text = plan.elements?.find((t) => t.instanceId === selection.id)
  if (!text) return 0
  const interval = textInterval(plan, text)
  const phrase = selection.phrase === undefined ? undefined : text.phrases?.[selection.phrase]
  return phrase ? interval.cutOffsetMs + phrase.startMs : interval.startMs
}

export const snapClipTime = (ms: number) =>
  Math.round(
    (Math.round((ms * CLIP_TIMELINE.framesPerSecond) / 1000) * 1000) /
      CLIP_TIMELINE.framesPerSecond,
  )
export const clipSeconds = (ms: number) =>
  Number.isFinite(ms) ? String(Number((ms / 1000).toFixed(3))) : '—'

export function splitTextPhrases(plan: ClipEditPlan, text: ClipEditableText) {
  const interval = textInterval(plan, text)
  if (!interval.valid) return null
  return (
    splitRapid(
      {
        text: text.text,
        anchor: 'bottom',
        align: 'center',
        keyword: text.keyword,
        style: 'clean',
        accent: '',
        startMs: 0,
        endMs: 0,
      },
      interval.startMs - interval.cutOffsetMs,
      interval.endMs - interval.cutOffsetMs,
    )?.map(({ text, startMs, endMs }) => ({ text, startMs, endMs })) ?? null
  )
}

export interface ClipTextBar {
  id: string
  phrase?: number
  text: string
  startMs: number
  endMs: number
  invalid: boolean
}
/** Reuse lanes across disjoint intervals, so 100 sequential cuts need one
 * caption row rather than 100 vertically stacked rows. */
export function clipTextTracks(plan: ClipEditPlan): ClipTextBar[][] {
  const bars = (plan.elements ?? [])
    .flatMap((text): ClipTextBar[] => {
      const interval = textInterval(plan, text)
      const phrases =
        text.pace === 'rapid'
          ? text.phrases?.length
            ? text.phrases
            : splitTextPhrases(plan, text)
          : undefined
      if (phrases?.length)
        return phrases.map((p, phrase) => ({
          id: text.instanceId,
          phrase,
          text: p.text,
          startMs: interval.cutOffsetMs + p.startMs,
          endMs: interval.cutOffsetMs + p.endMs,
          invalid: !interval.valid,
        }))
      return [
        {
          id: text.instanceId,
          text: text.text || text.rows.map((r) => r.text).join(' · ') || text.elementId,
          startMs: interval.startMs,
          endMs: interval.endMs,
          invalid: !interval.valid,
        },
      ]
    })
    .map((bar) =>
      bar.invalid
        ? { ...bar, startMs: 0, endMs: Math.min(1000, Math.max(1, plan.durationMs || 1)) }
        : bar,
    )
  bars.sort((a, b) => a.startMs - b.startMs || b.endMs - a.endMs)
  const tracks: ClipTextBar[][] = []
  for (const bar of bars) {
    const track = tracks.find((row) => row.at(-1)!.endMs <= bar.startMs)
    if (track) track.push(bar)
    else tracks.push([bar])
  }
  return tracks
}

interface TimelineSnapshot {
  plan: ClipEditPlan
  selection?: ClipSelection
}
export interface ClipTimelineState extends TimelineSnapshot {
  past: TimelineSnapshot[]
  future: TimelineSnapshot[]
  timeMs: number
  group?: { key: string; at: number }
}
export function createClipTimeline(plan: ClipEditPlan): ClipTimelineState {
  return {
    plan: copyClipPlan(plan),
    selection: plan.cuts[0] ? { kind: 'cut', id: plan.cuts[0].id } : undefined,
    past: [],
    future: [],
    timeMs: 0,
  }
}
export type ClipTimelineAction =
  | { type: 'edit'; edit: TimelineEdit; at: number; group?: string }
  | { type: 'select'; selection: ClipSelection }
  | { type: 'seek'; timeMs: number }
  | { type: 'undo' }
  | { type: 'redo' }
  | { type: 'endTransaction' }
  | { type: 'adopt'; plan: ClipEditPlan; clearHistory?: boolean }

function survivingSelection(
  before: ClipEditPlan,
  next: ClipEditPlan,
  selection?: ClipSelection,
): ClipSelection | undefined {
  if (selection?.kind === 'text' && next.elements?.some((t) => t.instanceId === selection.id))
    return selection
  if (selection?.kind === 'cut' && next.cuts.some((c) => c.id === selection.id)) return selection
  const cutId =
    selection?.kind === 'cut'
      ? selection.id
      : before.elements?.find((t) => t.instanceId === selection?.id)?.cutId
  const index = before.cuts.findIndex((c) => c.id === cutId)
  const cut = next.cuts[Math.max(0, Math.min(index, next.cuts.length - 1))]
  return cut ? { kind: 'cut', id: cut.id } : undefined
}

/** Saves never enter history. Playback changes time only, preserving focused selection. */
export function clipTimelineReducer(
  state: ClipTimelineState,
  action: ClipTimelineAction,
): ClipTimelineState {
  if (action.type === 'seek') return { ...state, timeMs: action.timeMs }
  if (action.type === 'select') {
    const time = selectedTime(state.plan, action.selection)
    return {
      ...state,
      selection: action.selection,
      timeMs: Number.isFinite(time) ? Math.max(0, time) : state.timeMs,
      group: undefined,
    }
  }
  if (action.type === 'endTransaction') return { ...state, group: undefined }
  if (action.type === 'adopt')
    return {
      ...state,
      plan: copyClipPlan(action.plan),
      selection: survivingSelection(state.plan, action.plan, state.selection),
      group: undefined,
      ...(action.clearHistory ? { past: [], future: [] } : {}),
    }
  if (action.type === 'undo' || action.type === 'redo') {
    const stack = action.type === 'undo' ? state.past : state.future
    const next = stack.at(-1)
    if (!next) return state
    const current = { plan: state.plan, selection: state.selection }
    return {
      ...state,
      ...next,
      group: undefined,
      past: action.type === 'undo' ? state.past.slice(0, -1) : [...state.past, current],
      future: action.type === 'undo' ? [...state.future, current] : state.future.slice(0, -1),
    }
  }
  const plan = applyTimelineEdit(state.plan, action.edit)
  if (clipDraftKey(plan) === clipDraftKey(state.plan)) return state
  const coalesced =
    !!action.group &&
    state.group !== undefined &&
    state.group.key === action.group &&
    action.at - state.group.at <= CLIP_TIMELINE.coalesceMs
  return {
    ...state,
    plan,
    selection: survivingSelection(state.plan, plan, state.selection),
    past: coalesced
      ? state.past
      : [...state.past, { plan: state.plan, selection: state.selection }].slice(
          -CLIP_TIMELINE.history,
        ),
    future: [],
    group: action.group ? { key: action.group, at: action.at } : undefined,
  }
}
