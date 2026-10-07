import { expect, it } from 'vitest'
import { writingTestHistorySearchSchema, writingTestSearchSchema } from './search'

it('defaults a fresh writing test to the model A/B writing axis and preserves explicit valid formats', () => {
  expect(writingTestSearchSchema({})).toMatchObject({ factor: 'model', stage: 'write', count: 2 })
  for (const count of [2, 4, 8, 16])
    expect(writingTestSearchSchema({ factor: 'template', count: String(count) })).toMatchObject({
      factor: 'template',
      stage: 'write',
      count,
    })
  expect(writingTestSearchSchema({ factor: 'model', stage: 'observe', count: 16 })).toMatchObject({
    factor: 'model',
    stage: 'observe',
    count: 16,
  })
})
it('rejects retired nonbinary counts and unsupported axes instead of silently reducing or defaulting them', () => {
  for (const count of [0, 1, 3, 5, 12, 32, 2.5, 'three'])
    expect(() => writingTestSearchSchema({ count })).toThrow('Unsupported writing test selection')
  expect(() => writingTestSearchSchema({ factor: 'video' })).toThrow()
  expect(() => writingTestSearchSchema({ stage: 'ranking' })).toThrow()
  for (const factor of ['voice', 'template', 'guideline'])
    expect(() => writingTestSearchSchema({ factor, stage: 'observe' })).toThrow()
})
it('rejects decoded search arrays and objects before they can become cast domain enums or counts', () => {
  for (const raw of [
    { factor: ['model'] },
    { stage: ['write'] },
    { count: [4] },
    { factor: {} },
    { stage: {} },
    { count: {} },
  ])
    expect(() => writingTestSearchSchema(raw)).toThrow()
})
it('retains named source and safe internal return filters while refusing external or script return targets', () => {
  expect(
    writingTestSearchSchema({
      sourcePostSlug: 'owned-source',
      voiceId: 'voice',
      templateId: 'template',
      guidelineId: 'guideline',
      entry: '/settings?tab=templates',
      draft: 'new-draft',
    }),
  ).toMatchObject({
    source: 'owned-source',
    voiceId: 'voice',
    templateId: 'template',
    guidelineId: 'guideline',
    entry: '/settings?tab=templates',
    draft: 'new-draft',
  })
  for (const entry of [
    'https://example.com/path',
    '//example.com/path',
    'javascript:alert(1)',
    '/\\example.com',
  ])
    expect(writingTestSearchSchema({ entry }).entry).toBeUndefined()
  expect(writingTestSearchSchema({ source: 42, voiceId: ['a', 'b'] })).toMatchObject({
    source: undefined,
    voiceId: undefined,
  })
  expect(writingTestHistorySearchSchema({ voiceId: ['a', 'b'] })).toEqual({ voiceId: undefined })
})
