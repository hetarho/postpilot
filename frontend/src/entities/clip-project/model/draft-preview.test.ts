import { describe, expect, it } from 'vitest'
import {
  previewCrop,
  previewElementIDs,
  previewFrame,
  previewMotion,
  previewTimeline,
} from './draft-preview'
import type { ClipEditPlan } from './edit-plan'

const plan: ClipEditPlan = {
  durationMs: 19800,
  hook: '',
  cuts: [
    {
      id: 'a',
      sourceId: 'a',
      fingerprint: 'a',
      startMs: 2000,
      endMs: 12000,
      transitionMs: 0,
      copies: [],
      chips: [],
      volumePermille: 1000,
    },
    {
      id: 'b',
      sourceId: 'b',
      fingerprint: 'b',
      startMs: 3000,
      endMs: 13000,
      transitionMs: 200,
      copies: [],
      chips: [],
      volumePermille: 1000,
    },
  ],
}

describe('output preview geometry and timing', () => {
  it('keeps an incomplete precise-entry draft away from the native media clock', () => {
    expect(
      previewTimeline({
        ...plan,
        cuts: plan.cuts.map((c, i) => ({ ...c, startMs: i ? c.startMs : NaN })),
      }),
    ).toEqual([])
    expect(
      previewTimeline({ ...plan, cuts: plan.cuts.map((c) => ({ ...c, endMs: c.startMs })) }),
    ).toEqual([])
    expect(
      previewTimeline({ ...plan, cuts: plan.cuts.map((c) => ({ ...c, endMs: c.startMs + 100 })) }),
    ).toEqual([])
  })
  it('maps overlapping fades to two independent source clocks and no third video', () => {
    const timeline = previewTimeline(plan)
    const frames = previewFrame(timeline, 9900)
    expect(frames.map((f) => [f.cut.id, f.sourceMs, f.opacity])).toEqual([
      ['a', 11900, 1],
      ['b', 3100, 0.5],
    ])
    expect(frames.map((f) => f.audioGain)).toEqual([0.5, 0.5])
    expect(previewFrame(timeline, 10000).map((f) => f.cut.id)).toEqual(['b'])
    expect(previewFrame(timeline, -100)[0]?.sourceMs).toBe(2000)
    expect(previewFrame(timeline, 19800)[0]?.sourceMs).toBeLessThan(13000)
  })
  it('shows black at the midpoint of a fade through black', () => {
    const changed = {
      ...plan,
      cuts: plan.cuts.map((c, i) => ({ ...c, transitionMs: i ? 300 : 0 })),
    }
    expect(previewFrame(previewTimeline(changed), 9850).map((f) => f.opacity)).toEqual([0, 0])
  })
  it('has no exposure leak across a 300 ms rapid cue boundary', () => {
    const cue = { startMs: 120, endMs: 420, inMs: 0, outMs: 0, dy: 0 }
    expect(previewMotion(cue, 119).opacity).toBe(0)
    expect(previewMotion(cue, 120)).toEqual({ opacity: 1, dy: 0 })
    expect(previewMotion(cue, 419).opacity).toBe(1)
    expect(previewMotion(cue, 420).opacity).toBe(0)
  })
  it('keeps output-level elements in every selected and next cut page', () => {
    const elements = [
      { instanceId: 'global', cutId: '' },
      { instanceId: 'a-copy', cutId: 'a' },
      { instanceId: 'b-copy', cutId: 'b' },
    ] as NonNullable<ClipEditPlan['elements']>
    const draft = { ...plan, elements }
    expect(previewElementIDs(draft, previewTimeline(draft), 0)).toEqual([
      'global',
      'a-copy',
      'b-copy',
    ])
    expect(previewElementIDs(draft, previewTimeline(draft), 12000)).toEqual(['global', 'b-copy'])
  })
  it.each([
    [1080, 1920],
    [1920, 1080],
    [1080, 1080],
  ])('covers the %i × %i canvas and clamps a focal point at the source edge', (width, height) => {
    const crop = previewCrop(1920, 1080, width, height, { x: 0, y: 0 })
    expect(crop.width).toBeGreaterThanOrEqual(100)
    expect(crop.height).toBeGreaterThanOrEqual(100)
    expect(crop.left).toBeCloseTo(0)
    expect(crop.top).toBeCloseTo(0)
    const opposite = previewCrop(1920, 1080, width, height, { x: 1, y: 1 })
    expect(opposite.left + opposite.width).toBeCloseTo(100)
    expect(opposite.top + opposite.height).toBeCloseTo(100)
  })
})
