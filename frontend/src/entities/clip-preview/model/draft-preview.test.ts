import { describe, expect, it } from 'vitest'
import {
  flowAssets,
  flowCut,
  flowInstant,
  frameLayers,
  frameTimeline,
  previewCrop,
  previewElementIDs,
  previewFrame,
  previewMotion,
  previewTimeline,
  transitionWeights,
} from './draft-preview'
import fixture from './cut-timeline.fixture.json'
import type { ClipEditPlan } from '@/entities/clip-plan'

const plan: ClipEditPlan = {
  durationMs: 19800,
  cuts: [
    {
      id: 'a',
      sourceId: 'a',
      fingerprint: 'a',
      startMs: 2000,
      endMs: 12000,
      transitionMs: 0,
      copies: [],
      volumePermille: 1000,
      playbackRatePermille: 1000,
    },
    {
      id: 'b',
      sourceId: 'b',
      fingerprint: 'b',
      startMs: 3000,
      endMs: 13000,
      transitionMs: 200,
      copies: [],
      volumePermille: 1000,
      playbackRatePermille: 1000,
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
  it('fades through black on the curve the server draws, not two straight halves', () => {
    const changed = {
      ...plan,
      cuts: plan.cuts.map((c, i) => ({ ...c, transitionMs: i ? 300 : 0 })),
    }
    // Half-way through, xfade has taken the outgoing cut all the way to black while the
    // incoming one is already a third of the way back.
    const [outgoing, incoming] = previewFrame(previewTimeline(changed), 9850).map((f) => f.opacity)
    const [, weight] = transitionWeights('fadeblack', 0.5)
    expect(outgoing).toBe(0)
    expect(incoming).toBeCloseTo(weight, 12)
    expect(weight).toBeCloseTo(0.341796875, 9)
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
      { instanceId: 'intro', role: 'hook', cutId: '' },
      { instanceId: 'outro', role: 'ending', cutId: '' },
      { instanceId: 'a-copy', cutId: 'a' },
      { instanceId: 'b-copy', cutId: 'b' },
    ] as NonNullable<ClipEditPlan['elements']>
    const draft = { ...plan, elements }
    expect(previewElementIDs(draft, previewTimeline(draft), 0)).toEqual([
      'intro',
      'outro',
      'a-copy',
      'b-copy',
    ])
    expect(previewElementIDs(draft, previewTimeline(draft), 12000)).toEqual([
      'intro',
      'outro',
      'b-copy',
    ])
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

it.each([500, 750, 1000, 1250, 1500, 2000])(
  'shares cumulative geometry and seek inverses at %i permille',
  async (rate) => {
    const { timelineCuts, clipPlanDuration, outputToSourceMs, sourceToOutputMs } =
      await import('@/entities/clip-plan/model/edit-plan')
    const cuts = plan.cuts.map((c) => ({ ...c, playbackRatePermille: rate }))
    const changed = { ...plan, cuts, durationMs: clipPlanDuration(cuts) }
    const timeline = previewTimeline(changed)
    expect(timeline).toEqual(timelineCuts(changed))
    expect(timeline[0].endMs).toBe(Math.floor((10000 * 1000 + rate / 2) / rate))
    expect(timeline[1].startMs).toBe(timeline[0].endMs - 200)
    expect(timeline[1].endMs).toBe(changed.durationMs)
    for (const item of timeline)
      for (const delta of [0, 100, 300, 1000]) {
        const output = item.startMs + delta
        expect(
          Math.abs(sourceToOutputMs(item, outputToSourceMs(item, output)) - output),
        ).toBeLessThanOrEqual(1)
      }
    const frames = previewFrame(timeline, timeline[1].startMs + 100)
    expect(frames.map((f) => f.audioGain)).toEqual([0.5, 0.5])
    expect(frames[1].sourceMs).toBe(3000 + rate / 10)
    expect(previewFrame(timeline, timeline[0].endMs).map((f) => f.cut.id)).toEqual(['b'])
  },
)

it('refuses explicit invalid rates and checked-integer overflow instead of previewing 1x', async () => {
  const { transformedDurationMs } = await import('@/entities/clip-plan/model/edit-plan')
  for (const rate of [0, 600, NaN])
    expect(
      previewTimeline({ ...plan, cuts: [{ ...plan.cuts[0], playbackRatePermille: rate }] }),
    ).toEqual([])
  expect(transformedDurationMs(Number.MAX_SAFE_INTEGER, 500)).toBe(0)
})

// CLIP-173, CLIP-175: the flow simulation stands one cut per instant and the overlay by its
// intervals, and its end is the last frame.
describe('the flow simulation', () => {
  const timeline = previewTimeline(plan)
  it('stands the cut whose interval holds the playhead, the incoming one in a transition', () => {
    expect(flowCut(timeline, 0)?.cut.id).toBe('a')
    expect(flowCut(timeline, 9700)?.cut.id).toBe('a')
    // b fades in from 9800 over a's last 200 ms: a still has no fade, so it is b.
    expect(flowCut(timeline, 9900)?.cut.id).toBe('b')
    expect(flowCut(timeline, 15000)?.cut.id).toBe('b')
  })
  it('draws the last instant at the scrubber’s end, never the empty one after it', () => {
    expect(flowInstant(timeline, 19800)).toBe(19799)
    expect(flowCut(timeline, 19800)?.cut.id).toBe('b')
    expect(flowCut(timeline, 99999)?.cut.id).toBe('b')
    expect(flowCut([], 0)).toBeUndefined()
  })
  it('shows each overlay asset while its interval holds the playhead', () => {
    const assets = [
      { id: 'intro', startMs: 0, endMs: 2500 },
      { id: 'caption', startMs: 2500, endMs: 6000 },
      { id: 'outro', startMs: 16800, endMs: 19800 },
      { id: 'badge', startMs: 0, endMs: 19800 },
    ]
    const at = (ms: number) => flowAssets(assets, timeline, ms).map((a) => a.id)
    expect(at(0)).toEqual(['intro', 'badge'])
    expect(at(2500)).toEqual(['caption', 'badge'])
    expect(at(10000)).toEqual(['badge'])
    expect(at(19800)).toEqual(['outro', 'badge'])
  })
})

/** A case of the timeline Go records (backend/internal/clip/media/testdata/cut-timeline.json). */
interface TimelineCase {
  name: string
  fps: number
  cuts: {
    id: string
    startMs: number
    endMs: number
    transitionMs: number
    playbackRatePermille: number
  }[]
  frames: { cut: number; sourceMs: number; weight: number }[][]
}
function fixturePlan(c: TimelineCase): ClipEditPlan {
  return {
    durationMs: 0,
    cuts: c.cuts.map((cut) => ({
      ...cut,
      sourceId: cut.id,
      fingerprint: cut.id.repeat(64),
      copies: [],
      volumePermille: 1000,
    })),
  }
}

// CLIP-192: the browser walks the server's frames. Every output frame of the plans Go records
// — a hard cut, a dissolve, a fade through black and cuts at other rates — is made of the
// same cuts, at the same source instants and the same xfade weights.
describe('the server frame timeline', () => {
  const cases = fixture as TimelineCase[]
  it('covers every join the fixture claims', () => {
    expect(cases.map((c) => c.name)).toEqual([
      'hard-cut',
      'fade-200',
      'fade-through-black-300',
      'rate-changed',
    ])
  })
  it.each(cases.map((c) => [c.name, c] as const))('%s matches frame for frame', (_name, c) => {
    const timeline = frameTimeline(fixturePlan(c), c.fps)
    expect(timeline.total).toBe(c.frames.length)
    c.frames.forEach((want, frame) => {
      const got = frameLayers(timeline, frame)
      expect(got.map((layer) => [layer.index, layer.sourceMs])).toEqual(
        want.map((layer) => [layer.cut, layer.sourceMs]),
      )
      got.forEach((layer, i) =>
        expect(Math.abs(layer.weight - want[i]!.weight)).toBeLessThanOrEqual(1 / 255),
      )
    })
    expect(frameLayers(timeline, timeline.total)).toEqual([])
  })
})
