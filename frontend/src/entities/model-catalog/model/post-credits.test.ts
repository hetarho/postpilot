import { describe, expect, it } from 'vitest'
import { pairPostFigure } from './post-credits'
import type { CatalogModel, ModelStageAccess, PostCreditFigure, StageName } from './types'

const model = (
  modelId: string,
  postCredits: Partial<Record<StageName, PostCreditFigure>> = {},
  access: Partial<Record<StageName, Partial<ModelStageAccess>>> = {},
): CatalogModel => ({
  ref: { providerId: 'openrouter', modelId },
  label: modelId,
  vision: true,
  videoInput: false,
  structuredOutput: false,
  stages: ['observe', 'write'],
  levels: {},
  access: access as CatalogModel['access'],
  postCredits,
  disabled: false,
  disabledReason: '',
  contextTokens: 0n,
  requiredCredits: 5,
  affordable: true,
})

const eyes = model('eyes', { observe: { credits: 30, basis: 'recent' } })
const pen = model('pen', { write: { credits: 12, basis: 'recent' } })

// QUOTA-64: one post is the write figure plus, with photos, the observe figure; the sum is an
// estimate when either part is.
describe('pairPostFigure', () => {
  it('prices a post without photos on the write model alone', () => {
    expect(pairPostFigure({ observe: eyes, write: pen, withPhotos: false })).toEqual({
      credits: 12,
      basis: 'recent',
    })
  })

  it('adds the observe part for a post with photos', () => {
    expect(pairPostFigure({ observe: eyes, write: pen, withPhotos: true })).toEqual({
      credits: 42,
      basis: 'recent',
    })
    const estimatedEyes = model('eyes', { observe: { credits: 30, basis: 'estimate' } })
    expect(pairPostFigure({ observe: estimatedEyes, write: pen, withPhotos: true })?.basis).toBe(
      'estimate',
    )
  })

  // A free stage costs no credits, so it adds nothing; a pair of free models has no number to
  // show at all, since a zero would read as a promise of unlimited use.
  it('adds nothing for a free stage and shows nothing for an all-free pair', () => {
    const freeEyes = model('free-eyes', {}, { observe: { grade: 'free' } })
    expect(pairPostFigure({ observe: freeEyes, write: pen, withPhotos: true })).toEqual({
      credits: 12,
      basis: 'recent',
    })
    const freePen = model('free-pen', {}, { write: { grade: 'free' } })
    expect(pairPostFigure({ observe: freeEyes, write: freePen, withPhotos: true })).toBeUndefined()
  })

  it('shows nothing when a paid stage has no figure or no write model is chosen', () => {
    expect(
      pairPostFigure({ observe: model('unpriced'), write: pen, withPhotos: true }),
    ).toBeUndefined()
    expect(
      pairPostFigure({ observe: eyes, write: model('unpriced'), withPhotos: false }),
    ).toBeUndefined()
    expect(pairPostFigure({ observe: eyes, write: undefined, withPhotos: true })).toBeUndefined()
  })
})
