import { describe, expect, it } from 'vitest'
import { readWritingKit, readyCount, type KitInputs } from './readiness'

const settled = { pending: false, failed: false }
const empty: KitInputs = {
  voices: { ...settled, voices: [] },
  templates: { ...settled, saved: 0, unsavedDrafts: 0 },
  guidelines: { ...settled, own: 0, defaultsInUse: 0 },
  memories: { ...settled, count: 0 },
}
const byKey = (inputs: KitInputs) =>
  Object.fromEntries(readWritingKit(inputs).map((reading) => [reading.key, reading]))

// The kit reads the owner's own lists and invents no progress: a ring is one mark per thing made,
// or a voice's own readiness share while it is being made.
describe('the writing kit readiness', () => {
  it('reads a new account as four empty elements', () => {
    const readings = readWritingKit(empty)
    expect(readings.map((reading) => [reading.key, reading.state])).toEqual([
      ['voices', 'empty'],
      ['templates', 'empty'],
      ['guidelines', 'empty'],
      ['memories', 'empty'],
    ])
    expect(readyCount(readings)).toBe(0)
  })

  it('reads stand-ins as partial: a voice being made, an unsaved draft, only 기본 지침', () => {
    const kit = byKey({
      ...empty,
      voices: { ...settled, voices: [{ made: false, readinessPercent: 62.4 }] },
      templates: { ...settled, saved: 0, unsavedDrafts: 1 },
      guidelines: { ...settled, own: 0, defaultsInUse: 9 },
    })
    expect(kit.voices).toMatchObject({ state: 'partial', marks: { kind: 'share', percent: 62 } })
    expect(kit.templates).toMatchObject({
      state: 'partial',
      marks: { kind: 'count', own: 0, assumed: 1 },
    })
    expect(kit.guidelines).toMatchObject({
      state: 'partial',
      marks: { kind: 'count', own: 0, assumed: 9 },
    })
    expect(kit.memories.state).toBe('empty')
  })

  it('reads what the owner made as ready, one mark per item', () => {
    const readings = readWritingKit({
      voices: {
        ...settled,
        voices: [
          { made: true, readinessPercent: 0 },
          { made: true, readinessPercent: 0 },
          { made: false, readinessPercent: 30 },
        ],
      },
      templates: { ...settled, saved: 3, unsavedDrafts: 1 },
      guidelines: { ...settled, own: 2, defaultsInUse: 5 },
      memories: { ...settled, count: 12 },
    })
    expect(readings.every((reading) => reading.state === 'ready')).toBe(true)
    expect(readyCount(readings)).toBe(4)
    expect(readings[0].marks).toEqual({ kind: 'count', own: 2, assumed: 0 })
    expect(readings[2].marks).toEqual({ kind: 'count', own: 2, assumed: 5 })
  })

  it('keeps a read that has not answered or failed out of the ready count', () => {
    const kit = byKey({
      ...empty,
      memories: { pending: true, failed: false, count: 0 },
      templates: { pending: false, failed: true, saved: 0, unsavedDrafts: 0 },
    })
    expect(kit.memories.pending).toBe(true)
    expect(kit.templates.failed).toBe(true)
    expect(readyCount(Object.values(kit))).toBe(0)
  })
})
