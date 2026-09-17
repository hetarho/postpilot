import type { ClipNotice } from './notices'
import type { ClipProjectRequest } from './revision'
import type { GenerationJob } from '@/entities/generation-job/@x/clip-project'
import type { ClipCTAId, ClipDisclosureId } from '@/shared/config'
import type { ClipProjectComposition, ClipCompositionInputs } from './composition'
import type { ClipEditingState } from './edit-plan'
import type { ClipObservations } from './observations'
import {
  compositionCharacters,
  type ClipAccent,
  type ClipComposition,
} from '@/entities/clip-template/@x/clip-project'
import { emptyCompositionInputs, validCompositionInputs } from './composition-inputs'

export const CLIP_RATIOS = ['vertical', 'horizontal', 'square'] as const
/** The five campaign types and three CTAs, read from the design system: the
 *  phrases are code-owned and only these ids ever travel (CDS-29, CDS-31). */
export { CLIP_DISCLOSURES, CLIP_CTAS } from '@/shared/config'
import { CLIP_CTAS, CLIP_DISCLOSURES } from '@/shared/config'
export type { ClipDisclosureId, ClipCTAId } from '@/shared/config'
export type ClipRatio = (typeof CLIP_RATIOS)[number]
export const CLIP_PROJECT_LIMITS = {
  title: 100,
  answer: 500,
  /** The project instruction's maximum, counted CDS-20's way (CLIP-121). */
  instruction: 1000,
  minSeconds: 15,
  maxSeconds: 90,
} as const
export interface ClipProjectDraft {
  compositionInputs?: ClipCompositionInputs
  title: string
  videoTemplateId: string
  ratio: ClipRatio
  targetDurationMs: number
  answers: Array<{ label: string; text: string }>
  /** The campaign type the disclosure badge shows. Empty is allowed while the
   *  clip is being set up; generation refuses it (CDS-5, CDS-31). */
  disclosure: ClipDisclosureId | ''
  hideDisclosure?: boolean
  /** The closing call to action, or empty for the template preset's (CDS-29). */
  cta: ClipCTAId | ''
  /** The owner's own instruction for this clip (CLIP-121). Optional, and an
   *  empty string is indistinguishable from never having written one. */
  instruction?: string
  /** How the captions of THIS clip are paced and which colour one word takes
   *  (CLIP-139). Empty is not chosen: the clip falls back to what its template
   *  said. Changing either re-renders the same plan and costs no writing. */
  captionPace?: '' | 'steady' | 'rapid'
  accent?: ClipAccent
}
export interface ClipProject extends ClipProjectDraft {
  notices?: ClipNotice[]
  /** What the owner asked the AI for, newest first (CLIP-133). */
  requests?: ClipProjectRequest[]
  language?: 'ko' | 'en'
  finalized?: { at: string; planRevision: number; resultId: string }
  canEdit?: boolean
  canFinalize?: boolean
  finalizationRefusal?:
    'finalized' | 'busy' | 'missing_render' | 'stale_render' | 'invalid_plan' | 'unavailable'

  composition?: ClipProjectComposition
  id: string
  createdAt: string
  updatedAt: string
  editPlanRevision: number
  renderedPlanRevision: number
  latestJob?: GenerationJob
  latestAttempt?: { jobId: string; batchId: string; quoteId: string }
  accounting?: ClipAccounting
  editing?: ClipEditingState
  attemptInspection?: ClipAttemptInspection
  observations?: ClipObservations
  result?: {
    id?: string
    contentType: string
    bytes: number
    durationMs: number
    createdAt: string
    viewUrl?: string
    downloadUrl?: string
  }
}
export interface ClipAccounting {
  nominalReservedCredits?: number
  confirmedChargeCredits?: number
  cancellationFeeCredits?: number
  shadowConfirmedChargeCredits?: number
  shadowCancellationFeeCredits?: number
  settlementReason?: 'succeeded' | 'failed' | 'cancelled'
  cancellationPolicyVersion?: number
  jobId: string
  status: 'not_reserved' | 'reserved' | 'settling' | 'settled' | 'exempt' | 'unavailable'
  approvedMaxCredits?: number
  reservedCredits?: number
  finalChargeCredits?: number
  refundCredits?: number
  shadowChargeCredits?: number
  settled: boolean
}
/** One priced line of the quote. A generation makes two writing calls on the
 *  same model, so the label — not the stage — says which line is which. */
export interface ClipPricedCall {
  label: 'observe' | 'flow' | 'narration'
  calls: number
}
export interface ClipQuote {
  calls: ClipPricedCall[]
  recovery?: {
    reusedChunks: number
    remainingChunks: number
    renderOnly: boolean
    responseRetries: number
  }
  cancellationPolicy?: { version: number; numerator: number; denominator: number; rounding: 'ceil' }
  quoteId: string
  maxCredits: number
  expiresAt: string
  // Runtime approval binding contains only primitive settings/metadata, never media.
  binding: string
}
export interface ClipSourceMetadata {
  filename: string
  contentType: string
  bytes: number
  durationMs: number
  width: number
  height: number
  fingerprint: string
}
export type ClipSourceAvailability =
  'uploading' | 'available' | 'active' | 'expired' | 'missing' | 'cleanup_pending'
export interface ClipSourceBatch {
  current?: boolean
  id: string
  projectId: string
  state: 'uploading' | 'ready' | 'consuming' | 'cleanup_pending'
  expiresAt: string
  sources: Array<{
    id: string
    state: 'pending' | 'ready'
    retentionExpiresAt?: string
    availability?: ClipSourceAvailability
    actualBytes: number
    /** The owner's 원본 소리 유지 choice (CLIP-18). Off for every new source, and
     *  changed only through its own owner action — never by saving a plan. */
    retainOriginalAudio: boolean
    metadata: ClipSourceMetadata
  }>
}
export type ReadyClipBatch = ClipSourceBatch & { state: 'ready' }
export function emptyClipProject(): ClipProjectDraft {
  return {
    title: '',
    videoTemplateId: '',
    ratio: 'vertical',
    targetDurationMs: 0,
    answers: [],
    disclosure: '',
    hideDisclosure: false,
    cta: '',
    instruction: '',
    captionPace: '',
    accent: '',
  }
}
export function projectDraft(value: ClipProjectDraft): ClipProjectDraft {
  return {
    ...(value.compositionInputs
      ? { compositionInputs: structuredClone(value.compositionInputs) }
      : {}),
    title: value.title,
    videoTemplateId: value.videoTemplateId,
    ratio: value.ratio,
    targetDurationMs: value.targetDurationMs,
    answers: value.answers.map((a) => ({ ...a })),
    disclosure: value.disclosure,
    hideDisclosure: value.hideDisclosure ?? false,
    cta: value.cta,
    instruction: value.instruction ?? '',
    captionPace: value.captionPace ?? '',
    accent: value.accent ?? '',
  }
}
/** Key order is not part of what a draft SAYS, but it is part of what `JSON.stringify` writes.
 *  The server holds these as maps, so two reads of the same project can name the same fields in
 *  a different order, and the form compares serialized drafts to decide whether it is in sync
 *  (CLIP-39). Without this an edit stayed 'unsaved' for good: the settings saved, the comparison
 *  kept disagreeing, and ①'s approval panel never came back until the page was reloaded.
 *
 *  Only maps are ordered here. The item list and the associations keep the order they were given,
 *  because there the order is the owner's. */
const byKey = <T>(value: Record<string, T>): Record<string, T> =>
  Object.fromEntries(Object.entries(value).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)))

export function normalizeClipProject(value: ClipProjectDraft): ClipProjectDraft {
  const draft = projectDraft(value)
  const inputs = draft.compositionInputs
  return {
    ...draft,
    ...(inputs
      ? {
          compositionInputs: {
            values: byKey(inputs.values),
            items: Object.fromEntries(
              Object.entries(byKey(inputs.items)).map(([group, list]) => [
                group,
                list.map((item) => ({ id: item.id, values: byKey(item.values) })),
              ]),
            ),
            associations: inputs.associations,
          },
        }
      : {}),
    answers: [...draft.answers].sort((a, b) =>
      a.label < b.label ? -1 : a.label > b.label ? 1 : 0,
    ),
    title: value.title.trim(),
  }
}
/** `/clips/new` settles the ratio and nothing else (CLIP-130): the answers, the instruction and
 *  the length are written in ① beside the sources they describe, so minting asks only for what a
 *  project cannot exist without — and for the one value it can never change again (CLIP-9). */
export function validNewClipProject(value: ClipProjectDraft): boolean {
  const title = value.title.trim()
  return (
    !!title &&
    Array.from(title).length <= CLIP_PROJECT_LIMITS.title &&
    !!value.videoTemplateId &&
    CLIP_RATIOS.includes(value.ratio)
  )
}
/** What the server will accept for an EXISTING project: every bound it enforces on a patch, and
 *  nothing it only enforces at generation. ① is where the setup is completed now (CLIP-130), so a
 *  half-written project is an ordinary state that must keep saving and keep accepting sources —
 *  the missing answers and the missing length are refused at approval instead (CLIP-102, CLIP-7). */
export function savableClipProject(value: ClipProjectDraft): boolean {
  const length = (s: string) => Array.from(s).length
  const title = value.title.trim()
  return (
    !!title &&
    length(title) <= CLIP_PROJECT_LIMITS.title &&
    !!value.videoTemplateId &&
    CLIP_RATIOS.includes(value.ratio) &&
    (value.targetDurationMs === 0 ||
      (Number.isInteger(value.targetDurationMs) &&
        value.targetDurationMs >= CLIP_PROJECT_LIMITS.minSeconds * 1000 &&
        value.targetDurationMs <= CLIP_PROJECT_LIMITS.maxSeconds * 1000)) &&
    (value.cta === '' || CLIP_CTAS.includes(value.cta as ClipCTAId)) &&
    value.answers.every((a) => length(a.text) <= CLIP_PROJECT_LIMITS.answer) &&
    compositionCharacters(value.instruction ?? '') <= CLIP_PROJECT_LIMITS.instruction
  )
}
export function validClipProject(
  value: ClipProjectDraft,
  fields: readonly { label: string }[] | undefined,
  composition?: ClipComposition,
): boolean {
  const length = (s: string) => Array.from(s).length
  return (
    !!value.title.trim() &&
    length(value.title.trim()) <= CLIP_PROJECT_LIMITS.title &&
    !!value.videoTemplateId &&
    (!!composition || !!fields) &&
    // Campaign identity is required independently of badge visibility.
    (!!composition || CLIP_DISCLOSURES.includes(value.disclosure as ClipDisclosureId)) &&
    (value.cta === '' || CLIP_CTAS.includes(value.cta as ClipCTAId)) &&
    CLIP_RATIOS.includes(value.ratio) &&
    Number.isInteger(value.targetDurationMs) &&
    value.targetDurationMs >= CLIP_PROJECT_LIMITS.minSeconds * 1000 &&
    value.targetDurationMs <= CLIP_PROJECT_LIMITS.maxSeconds * 1000 &&
    value.answers.every((a) => length(a.text) <= CLIP_PROJECT_LIMITS.answer) &&
    compositionCharacters(value.instruction ?? '') <= CLIP_PROJECT_LIMITS.instruction &&
    (composition
      ? validCompositionInputs(composition, value.compositionInputs ?? emptyCompositionInputs())
      : fields!.every((field) =>
          value.answers.some((a) => a.label === field.label && !!a.text.trim()),
        ))
  )
}

export interface ClipAttemptInspection {
  evidenceLimited?: boolean
  jobId: string
  status: 'available' | 'missing' | 'unavailable'
  stage: string
  completedChunks: number
  totalChunks: number
  completedSources: number
  totalSources: number
  observations: ClipObservations
  ranges: Array<{ cut: number; source: number; startMs: number; endMs: number; valid: boolean }>
  validationCheck: string
  validationPhase: string
  measurements: Record<string, number>
}
