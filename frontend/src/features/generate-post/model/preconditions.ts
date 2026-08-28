import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import type { ModelRef } from '@/entities/model-catalog'

export interface GenerationModelSelection {
  ref: ModelRef
  vision: boolean
}

export type GenerationPreconditions = { ok: true; reason: '' } | { ok: false; reason: string }

/** Mirrors the server gate so an impossible generation never looks clickable. */
export function generationPreconditions(
  images: readonly Pick<PostImage, 'id'>[],
  observeSelection: GenerationModelSelection | undefined,
  writeSelection: GenerationModelSelection | undefined,
  activeJob: Pick<GenerationJob, 'status'> | undefined,
): GenerationPreconditions {
  if (activeJob && activeJob.status !== 'done' && activeJob.status !== 'failed') {
    return { ok: false, reason: '이미 생성 중이에요.' }
  }
  if (!writeSelection) return { ok: false, reason: '작성 모델을 선택하세요.' }
  if (images.length === 0) return { ok: true, reason: '' }
  if (!observeSelection) return { ok: false, reason: '관찰 모델을 선택하세요.' }
  if (!observeSelection.vision) {
    return { ok: false, reason: '사진을 볼 수 있는 관찰 모델을 선택하세요.' }
  }
  return { ok: true, reason: '' }
}
