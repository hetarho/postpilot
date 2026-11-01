import { describe, expect, it } from 'vitest'
import { emptyVoice, type Voice } from '@/entities/voice'
import { clipOfferMissing, postOfferMissing } from './offer'

const voice = (id: string, made: boolean, createdAt = '2026-11-01T00:00:00Z'): Voice => ({
  ...emptyVoice(),
  id,
  name: id,
  made,
  createdAt,
})

describe('postOfferMissing', () => {
  it('names both kinds for an account with no voice and no template', () => {
    expect(postOfferMissing([], 0)).toEqual({ voice: { action: 'create' }, template: true })
  })

  it('names only the template when a made voice exists', () => {
    expect(postOfferMissing([voice('made', true)], 0)).toEqual({
      voice: undefined,
      template: true,
    })
  })

  it('names only the voice when a template exists', () => {
    expect(postOfferMissing([], 1)).toEqual({ voice: { action: 'create' }, template: false })
  })

  it('continues the newest voice still being made rather than creating another', () => {
    const active = [
      voice('older', false, '2026-10-01T00:00:00Z'),
      voice('newest', false, '2026-10-30T00:00:00Z'),
      voice('middle', false, '2026-10-15T00:00:00Z'),
    ]
    expect(postOfferMissing(active, 1)?.voice).toEqual({ action: 'continue', voiceId: 'newest' })
  })

  it('counts a voice being made beside a made one as no gap at all', () => {
    expect(postOfferMissing([voice('half', false), voice('made', true)], 2)).toBeUndefined()
  })
})

describe('clipOfferMissing', () => {
  it('offers a template to an account with none and never a voice', () => {
    expect(clipOfferMissing(0)).toEqual({ template: true })
    expect(clipOfferMissing(0)).not.toHaveProperty('voice')
  })

  it('offers nothing once a template exists', () => {
    expect(clipOfferMissing(3)).toBeUndefined()
  })
})
