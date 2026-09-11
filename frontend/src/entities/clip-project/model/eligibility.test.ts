import { describe, expect, it } from 'vitest'
import { clipEligibilityOf, isClipEligibilityStatus } from './eligibility'

describe('clip eligibility lookup', () => {
  const list = [
    { ref: { providerId: 'openrouter', modelId: 'google/gemini' }, status: 'eligible' as const },
    { ref: { providerId: 'openrouter', modelId: 'qwen/vl' }, status: 'eligible' as const },
    {
      ref: { providerId: 'openrouter', modelId: 'amazon/nova' },
      status: 'required_parameters_unsupported' as const,
    },
    { ref: { providerId: 'openrouter', modelId: 'twice' }, status: 'eligible' as const },
    {
      ref: { providerId: 'openrouter', modelId: 'twice' },
      status: 'price_ceiling_unavailable' as const,
    },
  ]
  it('answers by exact provider/model ref only', () => {
    expect(clipEligibilityOf(list, { providerId: 'openrouter', modelId: 'qwen/vl' })).toBe(
      'eligible',
    )
    expect(clipEligibilityOf(list, { providerId: 'openrouter', modelId: 'amazon/nova' })).toBe(
      'required_parameters_unsupported',
    )
    // The same model id under another provider is a different model.
    expect(clipEligibilityOf(list, { providerId: 'other', modelId: 'qwen/vl' })).toBeUndefined()
  })
  it('never reads a missing or duplicated answer as eligible', () => {
    expect(clipEligibilityOf(list, { providerId: 'openrouter', modelId: 'absent' })).toBeUndefined()
    expect(clipEligibilityOf(list, { providerId: 'openrouter', modelId: 'twice' })).toBeUndefined()
    expect(clipEligibilityOf([], { providerId: 'openrouter', modelId: 'qwen/vl' })).toBeUndefined()
  })
  it('knows exactly the five statuses', () => {
    for (const s of [
      'eligible',
      'video_input_absent',
      'inline_endpoint_unavailable',
      'required_parameters_unsupported',
      'price_ceiling_unavailable',
    ])
      expect(isClipEligibilityStatus(s)).toBe(true)
    expect(isClipEligibilityStatus('')).toBe(false)
    expect(isClipEligibilityStatus('unspecified')).toBe(false)
  })
})
