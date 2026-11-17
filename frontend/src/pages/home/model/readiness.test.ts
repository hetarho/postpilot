import { describe, expect, it } from 'vitest'
import { kitReadiness, kitSummary, type KitInputs } from './readiness'

const settled = { pending: false, failed: false }
const empty: KitInputs = {
  voices: { ...settled, active: [] },
  templates: { ...settled, count: 0 },
  guidelines: { ...settled, own: 0, defaultsInUse: 0 },
  memories: { ...settled, count: 0 },
}
const level = (inputs: KitInputs) =>
  Object.fromEntries(kitReadiness(inputs).map((item) => [item.element, item.level]))

describe('the writing kit readiness', () => {
  it('reads an empty account as four empty layers, the voice offered first', () => {
    expect(level(empty)).toEqual({
      template: 'empty',
      voice: 'empty',
      memory: 'empty',
      guideline: 'empty',
    })
    expect(kitSummary(kitReadiness(empty))).toEqual({
      ready: 0,
      total: 4,
      settled: true,
      next: 'voice',
    })
  })

  it('counts a voice ready only when a made voice is the default', () => {
    const voice = (active: NonNullable<KitInputs['voices']>['active']) =>
      kitReadiness({ ...empty, voices: { ...settled, active } }).find(
        (item) => item.element === 'voice',
      )
    expect(voice([{ made: true, isDefault: true, readinessPercent: 0 }])).toEqual({
      element: 'voice',
      level: 'ready',
    })
    // Made but not the default: partial, with no meter to show.
    expect(voice([{ made: true, isDefault: false, readinessPercent: 0 }])).toEqual({
      element: 'voice',
      level: 'partial',
      percent: undefined,
    })
    // Still being made: partial, carrying the server's own meter share (no invented number).
    expect(
      voice([
        { made: false, isDefault: false, readinessPercent: 40 },
        { made: false, isDefault: false, readinessPercent: 70 },
      ]),
    ).toEqual({ element: 'voice', level: 'partial', percent: 70 })
  })

  it('counts guidelines ready on an owner rule, and only partial on built-in ones', () => {
    const guideline = (own: number, defaultsInUse: number) =>
      level({ ...empty, guidelines: { ...settled, own, defaultsInUse } }).guideline
    expect(guideline(1, 0)).toBe('ready')
    expect(guideline(0, 5)).toBe('partial')
    expect(guideline(0, 0)).toBe('empty')
  })

  it('reports loading and failure per layer, and waits to offer a next step', () => {
    const kit = kitReadiness({
      ...empty,
      templates: { pending: true, failed: false, count: 0 },
      memories: { pending: false, failed: true, count: 0 },
    })
    expect(
      level({ ...empty, templates: { pending: true, failed: false, count: 0 } }).template,
    ).toBe('loading')
    expect(kit.find((item) => item.element === 'memory')?.level).toBe('failed')
    expect(kitSummary(kit).settled).toBe(false)
  })

  it('reads a full kit as ready with nothing next', () => {
    const full: KitInputs = {
      voices: { ...settled, active: [{ made: true, isDefault: true, readinessPercent: 0 }] },
      templates: { ...settled, count: 2 },
      guidelines: { ...settled, own: 1, defaultsInUse: 3 },
      memories: { ...settled, count: 4 },
    }
    expect(kitSummary(kitReadiness(full))).toEqual({
      ready: 4,
      total: 4,
      settled: true,
      next: undefined,
    })
  })
})
