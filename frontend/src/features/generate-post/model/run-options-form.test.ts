import { describe, expect, it } from 'vitest'
import type { GenerationOptionsSet } from '@/entities/post'
import {
  changedFrom,
  draftFromSet,
  lengthValid,
  setFromDraft,
  tagsValid,
  type RunOptionsDraft,
} from './run-options-form'

const NATURAL: GenerationOptionsSet = { tagCount: 4, useMemory: false, qualityRules: [], field: '' }
const STORED: GenerationOptionsSet = {
  targetLength: 1500,
  tagCount: 7,
  useMemory: true,
  qualityRules: ['title_saturation', 'composition'],
  field: 'cafe',
}

describe('draftFromSet', () => {
  it('seeds natural length as an unticked box with an empty length', () => {
    expect(draftFromSet(NATURAL)).toEqual({
      lengthOn: false,
      length: '',
      tags: '4',
      useMemory: false,
      qualityRules: [],
      field: '',
    })
  })

  it('seeds a stored length as a ticked box holding it', () => {
    expect(draftFromSet(STORED)).toEqual({
      lengthOn: true,
      length: '1500',
      tags: '7',
      useMemory: true,
      qualityRules: ['title_saturation', 'composition'],
      field: 'cafe',
    })
  })
})

describe('changedFrom', () => {
  it('is unchanged at its seed and again once a change is undone', () => {
    const seed = draftFromSet(STORED)
    expect(changedFrom(seed, STORED)).toBe(false)
    // A length ticked off and on again, a number retyped and the ticks in another order are the
    // same set.
    const undone: RunOptionsDraft = {
      ...seed,
      length: '1500',
      tags: '07',
      qualityRules: ['composition', 'title_saturation'],
    }
    expect(changedFrom(undone, STORED)).toBe(false)
    expect(changedFrom({ ...draftFromSet(NATURAL), length: '1200' }, NATURAL)).toBe(false)
  })

  it('is changed by any one member', () => {
    const seed = draftFromSet(STORED)
    for (const patch of [
      { lengthOn: false },
      { length: '1600' },
      { tags: '8' },
      { useMemory: false },
      { qualityRules: ['title_saturation'] as const },
      { qualityRules: ['title_saturation', 'composition', 'cross_post_phrases'] as const },
      { field: '' as const },
    ] satisfies Array<Partial<RunOptionsDraft>>) {
      expect(changedFrom({ ...seed, ...patch }, STORED), JSON.stringify(patch)).toBe(true)
    }
  })

  it('refuses a length or a count out of range', () => {
    const seed = draftFromSet(STORED)
    for (const length of ['', '99', '10001', '1500.5', 'abc']) {
      const draft = { ...seed, length }
      expect(lengthValid(draft), length).toBe(false)
      expect(changedFrom(draft, STORED), length).toBe(false)
    }
    for (const tags of ['', '0', '11', '2.5']) {
      const draft = { ...seed, tags }
      expect(tagsValid(draft), tags).toBe(false)
      expect(changedFrom(draft, STORED), tags).toBe(false)
    }
    // Natural length is valid whatever the hidden field holds.
    expect(lengthValid({ ...seed, lengthOn: false, length: 'abc' })).toBe(true)
    expect(lengthValid({ ...seed, length: '100' })).toBe(true)
    expect(tagsValid({ ...seed, tags: '10' })).toBe(true)
  })
})

describe('setFromDraft', () => {
  it('maps the form back to the set, natural length when the box is off', () => {
    expect(setFromDraft(draftFromSet(STORED))).toEqual(STORED)
    expect(setFromDraft({ ...draftFromSet(STORED), lengthOn: false })).toEqual({
      ...STORED,
      targetLength: undefined,
    })
  })
})
