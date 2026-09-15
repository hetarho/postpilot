import {
  CLIP_COPY,
  CLIP_RAPID,
  type ClipCaptionPace,
  CLIP_FACTS,
  CLIP_TIMING,
  CLIP_TRANSITION,
  CLIP_PLAYBACK,
  CLIP_RATES,
  CLIP_TYPE,
  CLIP_VOICE,
  clipCaption,
} from '@/shared/config'
import {
  CLIP_ACCENTS,
  COPY_STYLES,
  type ClipAccent,
  type CopyStyle,
} from '@/entities/clip-template/@x/clip-project'

import { copyChars, splitRapid, isRapidCut, canAddRapid } from './caption-pace'
import type { ClipSourceAssociation } from './composition'
export { copyChars } from './caption-pace'

/** A caption's VERTICAL anchor (CDS-12): the proto field is still called
 *  `position`, and its three former values top/center/bottom are gone. */
export const COPY_ANCHORS = ['top', 'upper_mid', 'lower_mid', 'bottom'] as const
/** Horizontal alignment against that anchor. */
export const COPY_ALIGNS = ['center', 'left', 'right'] as const
export interface ClipCaption {
  pace?: ClipCaptionPace
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
/** CDS-36's three transitions. The value is the one INTO the cut, so the first
 *  cut of a plan always carries 0: a clip does not fade in from nothing. 300 is
 *  the fade-through-black no control offers yet. */
export const CLIP_TRANSITIONS = [0, CLIP_TRANSITION.fade_ms, CLIP_TRANSITION.black_ms] as const
/** What step 2 offers: a hard cut or the fade a scene change earns. */
export const CLIP_TRANSITION_CHOICES = [0, CLIP_TRANSITION.fade_ms] as const
/** One source's owner-controlled original-sound retention (CLIP-18). Read only
 *  here: the owner changes it through its own action, never by saving a plan. */
export interface ClipSourceAudioSetting {
  sourceId: string
  fingerprint: string
  retainOriginalAudio: boolean
}
/** CLIP-98's six fixed rates, as integer permille. */
export const CLIP_PLAYBACK_RATES = CLIP_RATES
/** The pre-transition OUTPUT length of a source span at a fixed rate (CDS-62),
 *  to the nearest millisecond. The same formula the server computes, so preview
 *  and export cannot disagree by a millisecond. */
export function transformedDurationMs(spanMs: number, ratePermille: number): number {
  if (!Number.isSafeInteger(spanMs) || spanMs <= 0 || !CLIP_RATES.includes(ratePermille)) return 0
  const numerator = spanMs * CLIP_PLAYBACK.unit_permille + Math.floor(ratePermille / 2)
  return Number.isSafeInteger(numerator) ? Math.floor(numerator / ratePermille) : 0
}
/** A cut's fixed rate, reading a draft written before rates existed as 1x. */
export function cutRate(cut: Pick<ClipEditCut, 'playbackRatePermille'>): number {
  return cut.playbackRatePermille ?? CLIP_PLAYBACK.unit_permille
}
/** How long the cut occupies the edited output timeline, before its transition
 *  overlap is taken off. */
export function cutOutputMs(cut: ClipEditCut): number {
  return transformedDurationMs(cut.endMs - cut.startMs, cutRate(cut))
}
export interface ClipTimelineCut {
  cut: ClipEditCut
  index: number
  startMs: number
  endMs: number
}
/** Keep invalid coordinates visible to the editor; playback validates them first. */
export function timelineCuts(plan: Pick<ClipEditPlan, 'cuts'>): ClipTimelineCut[] {
  let offset = 0
  return plan.cuts.map((cut, index) => {
    const startMs = offset - (index ? cut.transitionMs : 0)
    const endMs = startMs + cutOutputMs(cut)
    offset = endMs
    return { cut, index, startMs, endMs }
  })
}
/** Convert only at the boundary. Evidence and authored output windows never move. */
export function outputToSourceMs(item: ClipTimelineCut, outputMs: number): number {
  return item.cut.startMs + Math.round(((outputMs - item.startMs) * cutRate(item.cut)) / 1000)
}
export function sourceToOutputMs(item: ClipTimelineCut, sourceMs: number): number {
  return item.startMs + Math.round(((sourceMs - item.cut.startMs) * 1000) / cutRate(item.cut))
}
export function sourceAudioEnabled(plan: ClipEditPlan, cut: ClipEditCut): boolean {
  if (plan.sourceAudio !== undefined)
    return plan.sourceAudio.some(
      (s) =>
        s.sourceId === cut.sourceId && s.fingerprint === cut.fingerprint && s.retainOriginalAudio,
    )
  // Only legacy plans lack the snapshot; match the server's legacy inference.
  return plan.cuts.some(
    (c) => c.sourceId === cut.sourceId && c.fingerprint === cut.fingerprint && c.volumePermille > 0,
  )
}
export interface ClipEditCut {
  focal?: { x: number; y: number }
  id: string
  sourceId: string
  fingerprint: string
  startMs: number
  endMs: number
  /** The transition into this cut (CDS-36): 0 is a hard cut. */
  transitionMs: number
  /** Sentence captions follow CDS-43; rapid phrases follow CDS-59. */
  copies: ClipCaption[]
  /** Reserved fact labels whose chips belong on this cut, at most two. */
  chips: string[]
  volumePermille: number
  /** The ONE constant rate this cut plays at, as permille (CLIP-98).
   * Legacy absence is normalized to 1x by the API mapper; explicit zero is invalid. */
  playbackRatePermille: number
  /** One-time provenance for a cut the owner is creating: it authorizes an id
   *  the saved plan does not contain, and the server drops it on acceptance.
   *  An id the server already knows must not carry it. */
  creation?: ClipCutCreation
}
/** The two operations that create footage (CLIP-98). Both name the approved cut
 *  the new one inherits its place in the template from. */
export interface ClipCutCreation {
  kind: 'add' | 'split'
  originCutId: string
}
/** The one id shape a cut the owner created may take. The draft proposes the id
 *  and the server validates its shape before admitting the cut. */
export function ownerCutId(): string {
  return `owner-${crypto.randomUUID()}`
}
export interface ClipEditableText {
  effectiveStartMs?: number
  effectiveEndMs?: number
  phrases?: { text: string; startMs: number; endMs: number }[]
  staleEvidence?: boolean
  evidenceReviewed?: boolean
  evidence?: { sourceId: string; fingerprint: string; startMs: number; endMs: number }[]
  fallbackReason?: string
  instanceId: string
  elementId: string
  cutId: string
  kind: string
  role: string
  text: string
  rows: { role: string; text: string }[]
  style: string
  position: string
  align: string
  basis: string
  startMs?: number
  endMs?: number
  pace: string
  accent: string
  keyword: string
  resolvedStartMs: number
  resolvedEndMs: number
  groupId: string
  itemId: string
}
export interface ClipEditPlan {
  /** The server's complete per-source original-sound snapshot. Absent is a plan
   *  written before the setting existed. */
  sourceAudio?: ClipSourceAudioSetting[]
  associations?: ClipSourceAssociation[]
  nativeComposition?: boolean
  elements?: ClipEditableText[]
  durationMs: number
  cuts: ClipEditCut[]
  /** The opening card's one sentence (CDS-28). Empty renders no hook card. */
  hook: string
}
export interface RetainedClipSource {
  id: string
  fingerprint: string
  filename: string
  durationMs: number
  width: number
  height: number
  /** The rates this source may actually be cut at, computed by the server from
   *  its verified original cadence. A slow rate is absent when the source cannot
   *  reach the output cadence without invented frames (CDS-68). */
  allowedRatePermille: number[]
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
    ...(plan.associations ? { associations: plan.associations.map((a) => ({ ...a })) } : {}),
    ...(plan.elements
      ? {
          elements: plan.elements.map((t) => ({
            ...t,
            rows: t.rows.map((row) => ({ ...row })),
            ...(t.phrases ? { phrases: t.phrases.map((p) => ({ ...p })) } : {}),
            ...(t.evidence ? { evidence: t.evidence.map((e) => ({ ...e })) } : {}),
          })),
        }
      : {}),
    ...(plan.sourceAudio ? { sourceAudio: plan.sourceAudio.map((v) => ({ ...v })) } : {}),
    cuts: plan.cuts.map((c) => ({
      ...c,
      ...(c.focal ? { focal: { ...c.focal } } : {}),
      chips: [...c.chips],
      copies: c.copies.map((copy) => ({ ...copy })),
    })),
  }
}
/** The copy a caller that knows nothing of CDS-43 means: the one a cut has
 *  always had. */
export function firstCopy(cut: ClipEditCut): ClipCaption {
  return cut.copies[0]!
}
/** CDS-43: a cut of 4 s or more may carry a second copy. */
export function allowsSecondCopy(cut: ClipEditCut): boolean {
  return cutOutputMs(cut) >= CLIP_COPY.second_min_cut_s * 1000
}
export type ClipEdit =
  | {
      type: 'addCut'
      id: string
      originCutId: string
      sourceId: string
      fingerprint: string
      startMs: number
      endMs: number
      focal: { x: number; y: number }
      /** Read from the retained owner lease, never a cut-authoring control. */
      retainedSound?: boolean
    }
  | { type: 'splitCut'; id: string; newId: string; sourceMs: number }
  | { type: 'rate'; id: string; ratePermille: number }
  | { type: 'move'; from: number; to: number }
  | { type: 'remove'; id: string }
  | {
      type: 'cut'
      id: string
      patch: Partial<
        Pick<
          ClipEditCut,
          'startMs' | 'endMs' | 'volumePermille' | 'transitionMs' | 'playbackRatePermille'
        >
      >
    }
  | { type: 'copy'; id: string; index?: number; patch: Partial<ClipCaption> }
  | { type: 'addCopy'; id: string }
  | { type: 'removeCopy'; id: string; index?: number }
  | { type: 'pace'; id: string; pace: ClipCaptionPace }
  | { type: 'chips'; id: string; chips: string[] }
  | { type: 'hook'; hook: string }
/** The clip is its footage less what each cut's own transition overlaps
 *  (CDS-36) — never one fade times the boundaries. */
export function clipPlanDuration(cuts: readonly ClipEditCut[]): number {
  return timelineCuts({ cuts: [...cuts] }).at(-1)?.endMs ?? 0
}
export function editClipPlan(plan: ClipEditPlan, edit: ClipEdit): ClipEditPlan {
  const next = copyClipPlan(plan)
  if (edit.type === 'addCut') {
    const index = next.cuts.findIndex((c) => c.id === edit.originCutId)
    if (!next.nativeComposition || index < 0 || next.cuts.some((c) => c.id === edit.id)) return plan
    next.cuts.splice(index + 1, 0, {
      id: edit.id,
      sourceId: edit.sourceId,
      fingerprint: edit.fingerprint,
      startMs: edit.startMs,
      endMs: edit.endMs,
      focal: { ...edit.focal },
      playbackRatePermille: 1000,
      transitionMs: 0,
      volumePermille: 1000,
      copies: [],
      chips: [],
      creation: { kind: 'add', originCutId: edit.originCutId },
    })
    if (
      edit.retainedSound !== undefined &&
      next.sourceAudio &&
      !next.sourceAudio.some(
        (s) => s.sourceId === edit.sourceId && s.fingerprint === edit.fingerprint,
      )
    )
      next.sourceAudio.push({
        sourceId: edit.sourceId,
        fingerprint: edit.fingerprint,
        retainOriginalAudio: edit.retainedSound,
      })
  } else if (edit.type === 'splitCut') {
    const index = next.cuts.findIndex((c) => c.id === edit.id),
      parent = next.cuts[index]
    if (
      !next.nativeComposition ||
      !parent ||
      parent.creation ||
      next.cuts.some((c) => c.id === edit.newId) ||
      !Number.isSafeInteger(edit.sourceMs) ||
      edit.sourceMs <= parent.startMs ||
      edit.sourceMs >= parent.endMs
    )
      return plan
    next.cuts.splice(
      index,
      1,
      { ...parent, endMs: edit.sourceMs },
      {
        ...parent,
        id: edit.newId,
        startMs: edit.sourceMs,
        transitionMs: 0,
        copies: [],
        chips: [],
        creation: { kind: 'split', originCutId: parent.id },
      },
    )
  } else if (edit.type === 'move') {
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
  else if (edit.type === 'hook') next.hook = edit.hook
  else
    next.cuts = next.cuts.map((c) => {
      if (c.id !== edit.id) return c
      if (edit.type === 'rate') return { ...c, playbackRatePermille: edit.ratePermille }
      if (edit.type === 'cut') return { ...c, ...edit.patch }
      if (edit.type === 'chips') return { ...c, chips: [...edit.chips] }
      if (edit.type === 'pace') {
        if (isRapidCut(c) === (edit.pace === 'rapid')) return c
        const seed = c.copies[0]!
        const text = c.copies
          .map((copy) => copy.text.trim())
          .filter(Boolean)
          .join(' ')
        const merged = { ...seed, text, keyword: '', pace: 'steady' as const, startMs: 0, endMs: 0 }
        if (edit.pace === 'steady') return { ...c, copies: [merged] }
        const copies = splitRapid(
          merged,
          CLIP_TIMING.copy_lead_ms,
          cutOutputMs(c) - CLIP_TIMING.copy_lead_ms,
        )
        return copies ? { ...c, copies } : c
      }
      // CDS-43: the second copy starts a clear 120 ms after the first leaves and
      // runs to the end of the cut's own window, so adding one never puts two
      // sentences on screen together.
      if (edit.type === 'addCopy') {
        if (isRapidCut(c)) {
          if (!canAddRapid(c)) return c
          const last = c.copies.at(-1)!
          return {
            ...c,
            copies: [
              ...c.copies,
              {
                ...last,
                text: '',
                keyword: '',
                startMs: last.endMs,
                endMs: Math.min(
                  last.endMs + CLIP_RAPID.medium_ms,
                  cutOutputMs(c) - CLIP_TIMING.copy_lead_ms,
                ),
              },
            ],
          }
        }
        if (c.copies.length > 1 || !allowsSecondCopy(c)) return c
        const lead = CLIP_TIMING.copy_lead_ms
        const window = copyWindow(c, 0)
        const half = lead + Math.round((window.end - window.start) / 2)
        return {
          ...c,
          copies: [
            { ...c.copies[0]!, startMs: lead, endMs: half },
            { ...c.copies[0]!, text: '', keyword: '', startMs: half + lead, endMs: window.end },
          ],
        }
      }
      // Removing the second copy gives the first CDS-27's default window back.
      if (edit.type === 'removeCopy') {
        if (isRapidCut(c))
          return c.copies.length > 1
            ? { ...c, copies: c.copies.filter((_, i) => i !== (edit.index ?? c.copies.length - 1)) }
            : c
        return { ...c, copies: [{ ...c.copies[0]!, startMs: 0, endMs: 0 }] }
      }
      const index = edit.index ?? 0
      return {
        ...c,
        copies: c.copies.map((copy, i) => (i === index ? { ...copy, ...edit.patch } : copy)),
      }
    })
  // A reorder carries each cut's transition with it, so whichever cut ends up
  // in front leads in from nothing — the same normalisation the server makes.
  if (next.cuts[0]) next.cuts[0] = { ...next.cuts[0], transitionMs: 0 }
  next.durationMs = clipPlanDuration(next.cuts)
  if (
    edit.type === 'move' ||
    edit.type === 'remove' ||
    edit.type === 'addCut' ||
    edit.type === 'splitCut' ||
    edit.type === 'rate' ||
    (edit.type === 'cut' &&
      ['startMs', 'endMs', 'playbackRatePermille', 'transitionMs'].some((key) => key in edit.patch))
  )
    next.elements = next.elements?.map((text) => ({
      ...text,
      effectiveStartMs: undefined,
      effectiveEndMs: undefined,
    }))
  return next
}
/** CDS-41's minimum exposure for a copy of this length. */
export function minExposureMs(text: string): number {
  return CLIP_TIMING.sub_min_base_ms + CLIP_TIMING.sub_min_per_char_ms * copyChars(text)
}
/** The window a copy actually gets: its own, or CDS-27's default inset. */
function copyWindow(cut: ClipEditCut, index: number) {
  const copy = cut.copies[index] ?? { startMs: 0, endMs: 0 }
  const whole = copy.startMs === 0 && copy.endMs === 0
  const lead = CLIP_TIMING.copy_lead_ms
  const start = whole ? lead : copy.startMs
  // Cut-relative windows are OUTPUT time: the default inset is measured against
  // the transformed length, not the source span (CDS-27, CDS-62).
  const end = whole ? cutOutputMs(cut) - lead : copy.endMs
  return { start, end, length: end - start }
}
/** The style's own line and character limits (CDS-20, CDS-23..26). */
function withinCaptionLimits(text: string): boolean {
  const rule = clipCaption()
  const lines = text.split('\n')
  return lines.length <= rule.lines && lines.every((line) => copyChars(line) <= rule.chars)
}
/** The hook card's sentence: two lines of nine at most (CDS-28), grounded in the
 *  owner's own answers (CDS-42). A hook the field refuses would be dropped by
 *  the compiler rather than shown, so it is refused here where it is typed. */
export function withinHookLimits(hook: string): boolean {
  const lines = hook.split('\n')
  return lines.length <= 2 && copyChars(hook) <= 2 * CLIP_TYPE.hook.chars
}

/** CDS-42: a sentence may only state numbers and Latin names the owner's own
 *  answers already carry, and never the voice the design system refuses. The
 *  server checks the same thing on the hook the model writes; this is what lets
 *  the field refuse one the owner types. */
export function groundedInAnswers(
  text: string,
  answers: ReadonlyArray<{ label: string; text: string }>,
): boolean {
  if (text.trim() === '') return true
  if (CLIP_VOICE.banned.some((token: string) => text.includes(token))) return false
  if (/\p{Extended_Pictographic}/u.test(text)) return false
  const haystack = answers.map((a) => a.text).join(' ')
  const digits = (value: string) => value.replace(/\D/gu, '')
  for (const number of text.match(/[\d,]*\d/gu) ?? [])
    if (!digits(haystack).includes(digits(number))) return false
  // A capitalised Latin run is a name the owner has to have given (CDS-42);
  // lowercase words are ordinary prose.
  for (const token of text.match(/\p{Lu}[\p{L}']+/gu) ?? [])
    if (!haystack.toLowerCase().includes(token.toLowerCase())) return false
  return true
}

/** Every copy that actually shows, in clip order: a cut's second copy follows
 *  its first, and CDS-38's step and CDS-40's run are read over that sequence. */
function placedCopies(cuts: readonly ClipEditCut[]) {
  return cuts.flatMap((cut, i) =>
    cut.copies.flatMap((copy, j) => (copy.text.trim() === '' ? [] : [{ cut: i, index: j, copy }])),
  )
}
/** CDS-38: consecutive copies move at most one anchor step, measured only
 *  between copies that share a style — a style change is a deliberate change. */
function withinAnchorStep(cuts: readonly ClipEditCut[], cut: number, index: number): boolean {
  const placed = placedCopies(cuts)
  const at = placed.findIndex((p) => p.cut === cut && p.index === index)
  if (at <= 0) return true
  const previous = placed[at - 1]!.copy
  const copy = placed[at]!.copy
  if (previous.style !== copy.style) return true
  const order = COPY_ANCHORS as readonly string[]
  return Math.abs(order.indexOf(copy.anchor) - order.indexOf(previous.anchor)) <= 1
}
export function requiredClipSources(plan: ClipEditPlan, sources: readonly RetainedClipSource[]) {
  const needed = new Set(plan.cuts.map((c) => c.fingerprint))
  return sources.filter((s) => needed.has(s.fingerprint))
}
/** Every pair of cuts taken from the SAME source that shares footage, and how
 *  many milliseconds each pair shares. Source ranges are half-open, so touching
 *  endpoints are adjacent rather than overlapping (CLIP-98). */
export function sourceOverlaps(cuts: readonly ClipEditCut[]): Map<string, number> {
  const out = new Map<string, number>()
  cuts.forEach((a, i) =>
    cuts.slice(i + 1).forEach((b) => {
      if (a.sourceId !== b.sourceId || a.fingerprint !== b.fingerprint) return
      const shared = Math.min(a.endMs, b.endMs) - Math.max(a.startMs, b.startMs)
      if (shared > 0) out.set([a.id, b.id].sort().join('\u0000'), shared)
    }),
  )
  return out
}
export function validateClipPlan(plan: ClipEditPlan, state: ClipEditingState) {
  const integer = Number.isSafeInteger
  // A plan saved before the rule may keep the overlap it already has, but an
  // edit may neither create a new one nor enlarge it.
  const grandfathered = sourceOverlaps(state.plan.cuts)
  const overlaps = sourceOverlaps(plan.cuts)
  const cuts = plan.cuts.map((c, index) => {
    const source = state.sources.find((s) => s.id === c.sourceId && s.fingerprint === c.fingerprint)
    // Every limit below is OUTPUT time; the source span keeps its own ms.
    const duration = cutOutputMs(c)
    const rapid = isRapidCut(c)
    // One entry per copy: a cut of 4 s or more may carry two (CDS-43), and every
    // per-caption rule is the copy's, not the cut's.
    const copies = c.copies.map((copy, j) => {
      const whole = copy.startMs === 0 && copy.endMs === 0
      const placed = copy.text.trim() !== ''
      const previous = j === 0 ? null : copyWindow(c, j - 1)
      return {
        // The rune ceiling is the contract's; the per-style line and character
        // limits below are the design system's, and both are mirrored here for
        // immediacy only — the server's verifier stays the authority.
        text:
          Array.from(copy.text).length > state.maxCopyRunes ||
          (rapid &&
            (!placed || copy.text.includes('\n') || copyChars(copy.text) > CLIP_RAPID.max_chars)) ||
          (placed && !withinCaptionLimits(copy.text)),
        exposure:
          placed &&
          (rapid
            ? whole ||
              copyWindow(c, j).length < CLIP_RAPID.min_ms ||
              copyWindow(c, j).length > CLIP_RAPID.max_ms
            : copyWindow(c, j).length < minExposureMs(copy.text)),
        pace:
          (copy.pace !== undefined && !['steady', 'rapid'].includes(copy.pace)) ||
          (copy.pace === 'rapid' && !rapid),
        copyStart:
          !integer(copy.startMs) ||
          copy.startMs < 0 ||
          (!whole && copy.startMs >= copy.endMs) ||
          // Never both at once: the second copy starts a clear 120 ms after the
          // first has left (CDS-43).
          (previous !== null &&
            copy.startMs - previous.end < (rapid ? 0 : CLIP_TIMING.copy_lead_ms)),
        copyEnd:
          !integer(copy.endMs) || (!whole && (copy.endMs <= copy.startMs || copy.endMs > duration)),
        // A cut whose copy the composer dropped carries no placement at all, and
        // that is a valid plan — the cut simply shows its footage.
        anchor:
          placed &&
          (!COPY_ANCHORS.includes(copy.anchor) ||
            !COPY_ALIGNS.includes(copy.align) ||
            !withinAnchorStep(plan.cuts, index, j)),
        keyword: copy.keyword !== '' && !copy.text.includes(copy.keyword),
        style:
          placed && (!COPY_STYLES.includes(copy.style) || !state.copyStyles.includes(copy.style)),
        accent: !CLIP_ACCENTS.includes(copy.accent),
      }
    })
    return {
      identity: !source || !c.id || plan.cuts.filter((v) => v.id === c.id).length !== 1,
      start: !integer(c.startMs) || c.startMs < 0 || c.startMs >= c.endMs,
      end: !integer(c.endMs) || c.endMs > (source?.durationMs ?? 0) || c.endMs <= c.startMs,
      duration: !integer(duration) || duration <= 2 * CLIP_TRANSITION.fade_ms,
      // CDS-36 admits three transitions, and the first cut takes none.
      transition:
        !(CLIP_TRANSITIONS as readonly number[]).includes(c.transitionMs) ||
        (plan.cuts[0] === c && c.transitionMs !== 0),
      // CDS-43: one copy, or two only on a cut of 4 s or more, and then a
      // description followed by the number it leads to.
      copyCount:
        c.copies.length === 0 ||
        c.copies.length > (rapid ? CLIP_RAPID.max_per_cut : CLIP_COPY.max_per_cut) ||
        (!rapid && c.copies.length > 1 && !allowsSecondCopy(c)),
      copyClasses: false,
      chips:
        c.chips.length > 2 ||
        c.chips.some((label) => !(CLIP_FACTS.chips as readonly string[]).includes(label)),
      volume: !integer(c.volumePermille) || c.volumePermille < 0 || c.volumePermille > 1000,
      // Exactly one CLIP-98 rate, and only one this source's verified cadence
      // actually admits (CDS-68). An unsupported rate is named, never replaced.
      rate:
        !integer(c.playbackRatePermille) ||
        !CLIP_RATES.includes(cutRate(c)) ||
        duration <= 0 ||
        !(source?.allowedRatePermille ?? []).includes(cutRate(c)),
      overlap: plan.cuts.some((other) => {
        if (other === c || other.sourceId !== c.sourceId || other.fingerprint !== c.fingerprint)
          return false
        const key = [c.id, other.id].sort().join('\u0000')
        const shared = overlaps.get(key)
        return shared !== undefined && shared > (grandfathered.get(key) ?? 0)
      }),
      copies,
    }
  })
  const frequency = false
  const duration = clipPlanDuration(plan.cuts)
  const timeline =
    !integer(plan.durationMs) ||
    plan.durationMs !== duration ||
    duration < state.minDurationMs ||
    duration > state.maxDurationMs
  const count = plan.cuts.length === 0 || plan.cuts.length > state.maxCuts
  const hook = !withinHookLimits(plan.hook)
  return {
    cuts,
    timeline,
    count,
    frequency,
    hook,
    valid:
      !count &&
      !timeline &&
      !frequency &&
      !hook &&
      cuts.every(
        (c) =>
          Object.entries(c).every(([key, v]) => key === 'copies' || !v) &&
          c.copies.every((copy) => Object.values(copy).every((v) => !v)),
      ),
  }
}
