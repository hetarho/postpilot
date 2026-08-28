import { describe, expect, it } from 'vitest'
import { type CatalogModel, filterForStage, refKey, sameRef } from './types'

const model = (modelId: string, vision: boolean, disabled = false): CatalogModel => ({
  ref: { providerId: 'openrouter', modelId },
  label: modelId,
  vision,
  structuredOutput: false,
  disabled,
  disabledReason: disabled ? 'API key not configured' : '',
})

describe('filterForStage', () => {
  const models = [model('free', true), model('text-only', false), model('paid', true, true)]

  // Plan 04 AC3.
  it('lists vision models only for observe, everything for write and analyze', () => {
    expect(filterForStage(models, 'observe').map((m) => m.ref.modelId)).toEqual(['free', 'paid'])
    expect(filterForStage(models, 'write')).toHaveLength(3)
    expect(filterForStage(models, 'analyze')).toHaveLength(3)
  })

  it('keeps disabled models in the list so the reason can be shown', () => {
    expect(filterForStage(models, 'observe').some((m) => m.disabled)).toBe(true)
  })
})

describe('refKey / sameRef', () => {
  it('identifies a model by provider and id', () => {
    expect(refKey({ providerId: 'a', modelId: 'b/c' })).toBe('a/b/c')
    expect(sameRef({ providerId: 'a', modelId: 'b' }, { providerId: 'a', modelId: 'b' })).toBe(true)
    expect(sameRef({ providerId: 'a', modelId: 'b' }, { providerId: 'x', modelId: 'b' })).toBe(false)
  })
})
