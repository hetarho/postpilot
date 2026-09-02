import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import type { ModelRef } from '@/entities/model-catalog'
import { deletedVoiceAIReason, type VoiceRef } from '@/entities/voice'

export interface GenerationModelSelection {
  ref: ModelRef
  vision: boolean
}

/** Why an action cannot run, as a value the UI can branch on. String-matching a translated
 *  sentence is not a branch, and the editor now has to tell "the models are not set up yet" —
 *  which it answers with a way to go and set them up — from "a job is already running", which it
 *  answers by waiting. */
export type GenerationBlocker =
  'voiceDeleted' | 'activeJob' | 'observe' | 'vision' | 'write' | 'pair' | 'different'

/** The blockers a route out of this screen can fix. `observe` · `vision` · `write` are the active
 *  selections and `pair` · `different` are the A/B candidates; all five are set in the writing
 *  brief. Everything else resolves on its own or belongs to another surface. */
const SETUP_BLOCKERS = new Set<GenerationBlocker>([
  'observe',
  'vision',
  'write',
  'pair',
  'different',
])

export function isSetupBlocker(blocker: GenerationBlocker | undefined): boolean {
  return blocker !== undefined && SETUP_BLOCKERS.has(blocker)
}

/** Where the user has to go to clear a setup blocker. One destination since the A/B candidates
 *  joined the brief: sending someone off to the AI 모델 page for a pair they can now set two taps
 *  away, without leaving the draft, is a longer road to the same two dropdowns. */
export function setupBlockerTarget(blocker: GenerationBlocker | undefined): 'brief' | undefined {
  return isSetupBlocker(blocker) ? 'brief' : undefined
}

export type GenerationPreconditions =
  | { ok: true; reason: ''; blocker?: undefined }
  | { ok: false; reason: string; blocker: GenerationBlocker }

/** Mirrors the server gate so an impossible generation never looks clickable. The voice comes
 *  first: a deleted voice refuses every machine result before any model is even asked about
 *  (spec/policy/generation.md). */
function sharedPreconditions(
  images: readonly Pick<PostImage, 'id'>[],
  observeSelection: GenerationModelSelection | undefined,
  activeJob: Pick<GenerationJob, 'status'> | undefined,
  voice: Pick<VoiceRef, 'deleted'> | undefined,
): GenerationPreconditions {
  if (voice?.deleted) return { ok: false, reason: deletedVoiceAIReason(), blocker: 'voiceDeleted' }
  if (activeJob && activeJob.status !== 'done' && activeJob.status !== 'failed') {
    return {
      ok: false,
      reason: i18next.t('generation.blocked.active', { ns: 'posts' }),
      blocker: 'activeJob',
    }
  }
  if (images.length === 0) return { ok: true, reason: '' }
  if (!observeSelection)
    return {
      ok: false,
      reason: i18next.t('generation.blocked.observe', { ns: 'posts' }),
      blocker: 'observe',
    }
  if (!observeSelection.vision) {
    return {
      ok: false,
      reason: i18next.t('generation.blocked.vision', { ns: 'posts' }),
      blocker: 'vision',
    }
  }
  return { ok: true, reason: '' }
}

export function ordinaryGenerationPreconditions(
  images: readonly Pick<PostImage, 'id'>[],
  observeSelection: GenerationModelSelection | undefined,
  writeSelection: GenerationModelSelection | undefined,
  activeJob: Pick<GenerationJob, 'status'> | undefined,
  voice?: Pick<VoiceRef, 'deleted'>,
): GenerationPreconditions {
  const shared = sharedPreconditions(images, observeSelection, activeJob, voice)
  if (!shared.ok) return shared
  if (!writeSelection)
    return {
      ok: false,
      reason: i18next.t('generation.blocked.write', { ns: 'posts' }),
      blocker: 'write',
    }
  return { ok: true, reason: '' }
}

export function comparisonGenerationPreconditions(
  images: readonly Pick<PostImage, 'id'>[],
  observeSelection: GenerationModelSelection | undefined,
  writeSelectionA: GenerationModelSelection | undefined,
  writeSelectionB: GenerationModelSelection | undefined,
  activeJob: Pick<GenerationJob, 'status'> | undefined,
  voice?: Pick<VoiceRef, 'deleted'>,
): GenerationPreconditions {
  const shared = sharedPreconditions(images, observeSelection, activeJob, voice)
  if (!shared.ok) return shared
  if (!writeSelectionA || !writeSelectionB)
    return {
      ok: false,
      reason: i18next.t('generation.blocked.pair', { ns: 'posts' }),
      blocker: 'pair',
    }
  if (
    writeSelectionA.ref.providerId === writeSelectionB.ref.providerId &&
    writeSelectionA.ref.modelId === writeSelectionB.ref.modelId
  )
    return {
      ok: false,
      reason: i18next.t('generation.blocked.different', { ns: 'posts' }),
      blocker: 'different',
    }
  return { ok: true, reason: '' }
}
import i18next from 'i18next'
