import { describe, expect, it } from 'vitest'
import { kitParts, kitSummary, type KitInput } from './readiness'

const settled = { isPending: false, isError: false }
const EMPTY: KitInput = {
  voices: { ...settled, active: [] },
  templates: { ...settled, count: 0, drafts: 0 },
  guidelines: { ...settled, own: 0, defaultsInUse: 0, suggestions: 0 },
  memories: { ...settled, count: 0 },
  model: { isPending: false, selectedLabel: null, unusable: false },
  spoken: { ...settled, count: 0, drafts: 0 },
}
const voice = (made: boolean, isDefault: boolean, readinessPercent = 0) => ({
  name: '내 말투',
  made,
  isDefault,
  readinessPercent,
})

describe('the writing kit readiness', () => {
  it('reads a new account as empty, with 말투 next — the product defaults are not the owner’s', () => {
    const parts = kitParts({
      ...EMPTY,
      guidelines: { ...settled, own: 0, defaultsInUse: 3, suggestions: 0 },
      model: { isPending: false, selectedLabel: 'Gemini', unusable: false },
    })
    expect(parts.guideline).toMatchObject({ state: 'partial', status: { kind: 'defaultsOnly' } })
    expect(kitSummary(parts)).toEqual({
      ready: 1,
      total: 5,
      settled: true,
      next: 'voice',
      phase: 'empty',
    })
  })

  it('fills the voice ring with the voice’s own readiness until it is made and the default', () => {
    expect(
      kitParts({ ...EMPTY, voices: { ...settled, active: [voice(false, true, 62)] } }).voice,
    ).toMatchObject({ state: 'partial', progress: 0.62, status: { kind: 'learning', percent: 62 } })
    expect(
      kitParts({ ...EMPTY, voices: { ...settled, active: [voice(true, false)] } }).voice,
    ).toMatchObject({ state: 'partial', progress: 1, status: { kind: 'noDefault' } })
    expect(
      kitParts({ ...EMPTY, voices: { ...settled, active: [voice(true, true)] } }).voice,
    ).toMatchObject({ state: 'ready', status: { kind: 'named', name: '내 말투' } })
  })

  it('counts an unsaved AI draft as a template in progress, and a saved one as ready', () => {
    expect(
      kitParts({ ...EMPTY, templates: { ...settled, count: 0, drafts: 1 } }).template,
    ).toMatchObject({ state: 'partial', status: { kind: 'draft' } })
    expect(
      kitParts({ ...EMPTY, templates: { ...settled, count: 2, drafts: 1 } }).template,
    ).toMatchObject({ state: 'ready', status: { kind: 'count', count: 2 } })
  })

  it('is ready only when every writing part is, and keeps the video voice out of the count', () => {
    const parts = kitParts({
      voices: { ...settled, active: [voice(true, true)] },
      templates: { ...settled, count: 1, drafts: 0 },
      guidelines: { ...settled, own: 1, defaultsInUse: 2, suggestions: 2 },
      memories: { ...settled, count: 3 },
      model: { isPending: false, selectedLabel: 'Gemini', unusable: false },
      spoken: { ...settled, count: 0, drafts: 0 },
    })
    expect(parts.guideline.attention).toBe(2)
    expect(parts.spoken.state).toBe('empty')
    expect(kitSummary(parts)).toMatchObject({ ready: 5, next: undefined, phase: 'ready' })
  })

  it('judges nothing while a list is loading or failed', () => {
    const parts = kitParts({
      ...EMPTY,
      voices: { isPending: true, isError: false, active: [] },
      memories: { isPending: false, isError: true, count: 0 },
    })
    expect(parts.voice.state).toBe('loading')
    expect(parts.memory).toMatchObject({ state: 'failed', status: { kind: 'failed' } })
    expect(kitSummary(parts)).toMatchObject({ settled: false, next: undefined, phase: 'loading' })
  })
})
