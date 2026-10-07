import { describe, expect, it } from 'vitest'
import type { CatalogModel, ComparisonPair, StageName } from './types'
import { eligibleTestPair } from './test-pair'

const ref = (modelId: string) => ({ providerId: 'openrouter', modelId })
function model(modelId: string, changes: Partial<CatalogModel> = {}): CatalogModel {
  return {
    ref: ref(modelId),
    label: modelId,
    vision: true,
    videoInput: false,
    structuredOutput: true,
    disabled: false,
    disabledReason: '',
    stages: ['observe', 'write'],
    contextTokens: 0n,
    requiredCredits: 5,
    affordable: true,
    levels: {},
    ...changes,
  }
}
function pair(stage: StageName = 'write'): ComparisonPair {
  return {
    stage,
    candidateA: { stage, slot: 'candidateA', ref: ref('a'), missing: false },
    candidateB: { stage, slot: 'candidateB', ref: ref('b'), missing: false },
    extraCandidates: [{ stage, slot: 'candidateC', ref: ref('c'), missing: false }],
  }
}
describe('eligibleTestPair', () => {
  it('takes exactly the saved stage A/B pair, ignores extras and preserves choices at zero balance', () => {
    const saved = pair()
    const models = [model('a', { affordable: false }), model('b'), model('c')]
    const before = structuredClone(saved)
    expect(eligibleTestPair([saved], models, 'write')).toEqual([ref('a'), ref('b')])
    expect(eligibleTestPair([saved], models, 'observe')).toBeUndefined()
    expect(saved).toEqual(before)
  })
  it.each([
    'duplicate',
    'missing',
    'unregistered',
    'wrong-stage',
    'disabled',
    'locked',
    'unpriced',
  ] as const)(
    'leaves %s saved pairs untouched instead of preselecting unusable contenders',
    (kind) => {
      const saved = pair()
      const models = [model('a'), model('b')]
      if (kind === 'duplicate') saved.candidateB!.ref = ref('a')
      if (kind === 'missing') saved.candidateA!.missing = true
      if (kind === 'unregistered') models.splice(0, 1)
      if (kind === 'wrong-stage') models[0]!.stages = ['observe']
      if (kind === 'disabled') models[0]!.disabled = true
      if (kind === 'locked') {
        models[0]!.access = {
          write: {
            grade: 'premium',
            requiredPlan: 'pro',
            entitled: false,
            freePathAvailable: false,
            unavailableReason: 'MODEL_PLAN_REQUIRED',
          },
        }
        saved.candidateA!.unavailableReason = 'MODEL_PLAN_REQUIRED'
        saved.candidateA!.requiredPlan = 'pro'
      }
      if (kind === 'unpriced') models[0]!.priceUnavailable = true
      const before = structuredClone(saved)
      expect(eligibleTestPair([saved], models, 'write')).toBeUndefined()
      expect(saved).toEqual(before)
    },
  )
})
