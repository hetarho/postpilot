import type { WritingTestPlan } from '@/entities/writing-test'
import { POST_TARGET_LENGTH_DEFAULT, POST_TAG_COUNT_DEFAULT } from '@/entities/post'
import { activeLocale } from '@/shared/lib'
import type { PhotoTestSearch } from './search'

/** An observation plan: observe-stage model contestants over the test's own photos, with no
 *  writing material, setting or fixed model (MODEL-97). */
export function photoTestPlan(seed: PhotoTestSearch): WritingTestPlan {
  return {
    output: 'observation',
    factor: 'model',
    modelStage: 'observe',
    count: seed.count,
    entrants: [],
    context: {
      sourcePostSlug: '',
      expectedInputRevision: 0n,
      expectedContentRevision: 0n,
      material: { text: '', fictional: false, attachmentIds: [], templateAnswers: [] },
      voiceId: '',
      templateId: '',
      guidelineSlotId: '',
      targetLanguage: activeLocale(),
      targetLength: POST_TARGET_LENGTH_DEFAULT,
      tagCount: POST_TAG_COUNT_DEFAULT,
      useMemory: false,
      qualityRules: [],
      photoIds: [],
    },
  }
}
