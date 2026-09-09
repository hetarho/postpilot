import { afterEach, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { initializeI18n } from '@/app/providers/i18n'
import { GenerationJobSchema } from '@/shared/api'
import { toGenerationJob } from '../api/job-mappers'
import { CLIP_STAGES, progressLabel, progressRatio } from './types'

afterEach(() => initializeI18n('ko'))
it.each(['ko', 'en'] as const)('labels every clip stage in %s without numeric prose', (locale) => {
  initializeI18n(locale)
  for (const stage of CLIP_STAGES) {
    const label = progressLabel({ stage, kind: 'generate_clip' })
    expect(label).not.toMatch(/generation\.|\d/)
    expect(label.length).toBeGreaterThan(2)
    const ratio = progressRatio({ stage, kind: 'generate_clip', progressDone: 1, progressTotal: 3 })
    expect(ratio).toEqual(
      ['prepare', 'analyze'].includes(stage) ? { done: 1, total: 3 } : undefined,
    )
  }
  expect(
    progressRatio({ kind: 'generate_clip', stage: 'prepare', progressDone: 0, progressTotal: 0 }),
  ).toBeUndefined()
})
it('keeps clip target, failed stage and strict credit refusal through the wire mapper', () => {
  const job = toGenerationJob(
    create(GenerationJobSchema, {
      id: 'clip-job',
      kind: 'generate_clip',
      clipProjectId: 'clip',
      status: 'failed',
      stage: 'prepare',
      failure: {
        reason: 'INSUFFICIENT_CREDITS',
        params: { required: '79', balance: '12', renews_at: '2026-09-30T15:00:00Z' },
      },
    }),
  )
  expect(job.clipProjectId).toBe('clip')
  expect(job.stage).toBe('prepare')
  expect(job.failure?.reason).toBe('INSUFFICIENT_CREDITS')
})
