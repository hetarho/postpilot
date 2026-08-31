import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import type { ModelRef } from '@/entities/model-catalog'
import { deletedVoiceAIReason, type VoiceRef } from '@/entities/voice'

export interface GenerationModelSelection {
  ref: ModelRef
  vision: boolean
}

export type GenerationPreconditions = { ok: true; reason: '' } | { ok: false; reason: string }

/** Mirrors the server gate so an impossible generation never looks clickable. The voice comes
 *  first: a deleted voice refuses every machine result before any model is even asked about
 *  (spec/policy/generation.md). */
function sharedPreconditions(
  images: readonly Pick<PostImage, 'id'>[],
  observeSelection: GenerationModelSelection | undefined,
  activeJob: Pick<GenerationJob, 'status'> | undefined,
  voice: Pick<VoiceRef, 'deleted'> | undefined,
): GenerationPreconditions {
  if (voice?.deleted) return { ok: false, reason: deletedVoiceAIReason() }
  if (activeJob && activeJob.status !== 'done' && activeJob.status !== 'failed') {
    return { ok: false, reason: i18next.t('generation.blocked.active', { ns: 'posts' }) }
  }
  if (images.length === 0) return { ok: true, reason: '' }
  if (!observeSelection)
    return { ok: false, reason: i18next.t('generation.blocked.observe', { ns: 'posts' }) }
  if (!observeSelection.vision) {
    return { ok: false, reason: i18next.t('generation.blocked.vision', { ns: 'posts' }) }
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
    return { ok: false, reason: i18next.t('generation.blocked.write', { ns: 'posts' }) }
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
    return { ok: false, reason: i18next.t('generation.blocked.pair', { ns: 'posts' }) }
  if (
    writeSelectionA.ref.providerId === writeSelectionB.ref.providerId &&
    writeSelectionA.ref.modelId === writeSelectionB.ref.modelId
  )
    return { ok: false, reason: i18next.t('generation.blocked.different', { ns: 'posts' }) }
  return { ok: true, reason: '' }
}
import i18next from 'i18next'
