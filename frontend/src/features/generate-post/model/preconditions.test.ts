import { describe, expect, it } from 'vitest'
import { comparisonGenerationPreconditions, ordinaryGenerationPreconditions, type GenerationModelSelection } from './preconditions'

const image = { id: 'image-1' }
const vision: GenerationModelSelection = {
  ref: { providerId: 'openrouter', modelId: 'vision' },
  vision: true,
}
const text: GenerationModelSelection = {
  ref: { providerId: 'openrouter', modelId: 'writer' },
  vision: false,
}
const textB: GenerationModelSelection = {
  ref: { providerId: 'openrouter', modelId: 'writer-b' },
  vision: false,
}

describe('generationPreconditions', () => {
  it.each([
    {
      name: 'no write model',
      images: [],
      observe: undefined,
      write: undefined,
      active: undefined,
      ok: false,
    },
    {
      name: 'photos and no observe model',
      images: [image],
      observe: undefined,
      write: text,
      active: undefined,
      ok: false,
    },
    {
      name: 'photos and a non-vision observe model',
      images: [image],
      observe: text,
      write: text,
      active: undefined,
      ok: false,
    },
    {
      name: 'no photos and no observe model',
      images: [],
      observe: undefined,
      write: text,
      active: undefined,
      ok: true,
    },
    {
      name: 'all required models',
      images: [image],
      observe: vision,
      write: text,
      active: undefined,
      ok: true,
    },
    {
      name: 'an active job',
      images: [],
      observe: undefined,
      write: text,
      active: { status: 'running' },
      ok: false,
    },
  ])('$name → $ok', ({ images, observe, write, active, ok }) => {
    expect(ordinaryGenerationPreconditions(images, observe, write, active).ok).toBe(ok)
  })

  it('does not require a comparison pair for ordinary generation', () => {
    expect(ordinaryGenerationPreconditions([], undefined, text, undefined).ok).toBe(true)
  })

  it('requires two distinct candidates only for A/B generation', () => {
    expect(comparisonGenerationPreconditions([], undefined, text, undefined, undefined).ok).toBe(false)
    expect(comparisonGenerationPreconditions([], undefined, text, text, undefined).ok).toBe(false)
    expect(comparisonGenerationPreconditions([], undefined, text, textB, undefined).ok).toBe(true)
  })
})
