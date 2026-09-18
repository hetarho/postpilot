import { expect, it } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { browserAudioPlan } from './browser-audio-plan'

it('keeps only retained sources while the silent cut still occupies its video interval', () => {
  const plan = clipTimelineFixture().plan
  plan.sourceAudio = plan.cuts.map((cut, i) => ({
    sourceId: cut.sourceId,
    fingerprint: cut.fingerprint,
    retainOriginalAudio: i === 0,
  }))
  plan.cuts[0].volumePermille = 250
  plan.cuts[0].playbackRatePermille = 2000
  plan.durationMs = 14800
  const schedule = browserAudioPlan(plan)
  expect(schedule.cuts).toHaveLength(1)
  expect(schedule.cuts[0]).toMatchObject({
    fingerprint: 'a'.repeat(64),
    frames: 240000,
    rate: 2,
    volume: 0.25,
    start: 0,
    end: 5,
    fadeIn: 0,
    fadeOut: 0.2,
  })
  expect(schedule.sampleFrames).toBe(14800 * 48)
  plan.sourceAudio[0].retainOriginalAudio = false
  expect(browserAudioPlan(plan).cuts).toEqual([])
})
it('fades each side of a hard cut without consuming output time', () => {
  const plan = clipTimelineFixture().plan
  plan.cuts[1].transitionMs = 0
  const schedule = browserAudioPlan(plan)
  expect(schedule.cuts.map((cut) => [cut.start, cut.end, cut.fadeIn, cut.fadeOut])).toEqual([
    [0, 10, 0, 0.06],
    [10, 20, 0.06, 0],
  ])
  expect(schedule.sampleFrames).toBe(20 * 48000)
})

it('dips only visible intro text on its declared output interval', () => {
  const plan = clipTimelineFixture().plan
  const hook = {
    ...plan.elements![0],
    role: 'hook',
    cutId: '',
    basis: 'output-start',
    startMs: 3000,
    endMs: 4500,
    text: '',
  }
  plan.elements = [hook]
  expect(browserAudioPlan(plan).dip).toEqual([])
  hook.text = 'visible intro'
  expect(browserAudioPlan(plan).dip).toEqual([{ start: 3, end: 4.5 }])
})
