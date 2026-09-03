import { describe, expect, it } from 'vitest'
import { type CatalogModel, filterForStage, refKey, sameRef } from './types'

const model = (
  modelId: string,
  stages: CatalogModel['stages'],
  disabled = false,
): CatalogModel => ({
  ref: { providerId: 'openrouter', modelId },
  label: modelId,
  vision: stages.includes('observe'),
  structuredOutput: false,
  stages,
  disabled,
  disabledReason: disabled ? 'API key not configured' : '',
  contextTokens: 0n,
  inputUsdPerMillion: '',
  outputUsdPerMillion: '',
  pricingCheckedAt: '',
  requiredCredits: 5,
  affordable: true,
})

describe('filterForStage', () => {
  // Change 20: a stage lists exactly its purpose's registrations. `generation-only`
  // mirrors a model registered only to image/video generation — no stage lists it.
  const models = [
    model('free', ['observe', 'write', 'analyze']),
    model('text-only', ['write', 'analyze']),
    model('paid', ['observe'], true),
    model('generation-only', []),
  ]

  it('lists only the models registered to the stage', () => {
    expect(filterForStage(models, 'observe').map((m) => m.ref.modelId)).toEqual(['free', 'paid'])
    expect(filterForStage(models, 'write').map((m) => m.ref.modelId)).toEqual(['free', 'text-only'])
    expect(filterForStage(models, 'analyze')).toHaveLength(2)
  })

  it('keeps disabled models in the list so the reason can be shown', () => {
    expect(filterForStage(models, 'observe').some((m) => m.disabled)).toBe(true)
  })
})

describe('refKey / sameRef', () => {
  it('identifies a model by provider and id', () => {
    expect(refKey({ providerId: 'a', modelId: 'b/c' })).toBe('a/b/c')
    expect(sameRef({ providerId: 'a', modelId: 'b' }, { providerId: 'a', modelId: 'b' })).toBe(true)
    expect(sameRef({ providerId: 'a', modelId: 'b' }, { providerId: 'x', modelId: 'b' })).toBe(
      false,
    )
  })
})
