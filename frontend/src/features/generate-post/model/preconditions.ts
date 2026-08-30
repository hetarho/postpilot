import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import type { ModelRef } from '@/entities/model-catalog'
import { DELETED_VOICE_AI_REASON, type VoiceRef } from '@/entities/voice'

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
  if (voice?.deleted) return { ok: false, reason: DELETED_VOICE_AI_REASON }
  if (activeJob && activeJob.status !== 'done' && activeJob.status !== 'failed') {
    return { ok: false, reason: '이미 생성 중이에요.' }
  }
  if (images.length === 0) return { ok: true, reason: '' }
  if (!observeSelection) return { ok: false, reason: '관찰 모델을 선택하세요.' }
  if (!observeSelection.vision) {
    return { ok: false, reason: '사진을 볼 수 있는 관찰 모델을 선택하세요.' }
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
  if (!writeSelection) return { ok: false, reason: '활성 작성 모델을 선택하세요.' }
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
    return { ok: false, reason: '작성 A/B 모델 두 개를 선택하세요.' }
  if (
    writeSelectionA.ref.providerId === writeSelectionB.ref.providerId &&
    writeSelectionA.ref.modelId === writeSelectionB.ref.modelId
  )
    return { ok: false, reason: '서로 다른 작성 모델을 선택하세요.' }
  return { ok: true, reason: '' }
}
