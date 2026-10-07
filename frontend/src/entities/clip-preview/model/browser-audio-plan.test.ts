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

it('uses native cumulative frame/sample joins and integer-ms hard fades for fractional rate spans', () => {
  const plan = clipTimelineFixture().plan
  plan.cuts = [
    {
      ...plan.cuts[0],
      id: 'first',
      startMs: 0,
      endMs: 1234,
      transitionMs: 0,
      playbackRatePermille: 1000,
    },
    {
      ...plan.cuts[0],
      id: 'second',
      startMs: 2000,
      endMs: 3234,
      transitionMs: 0,
      playbackRatePermille: 750,
    },
  ]
  const schedule = browserAudioPlan(plan)
  expect(schedule.cuts.map((c) => [c.startSample, c.frames])).toEqual([
    [0, 59200],
    [59200, 78400],
  ])
  expect(schedule.sampleFrames).toBe(137600)
  expect(schedule.cuts[0].fadeOutStart).toBe(1.173)
  expect(schedule.cuts[1].sourceStart).toBe(96000)
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

it('keeps speech on the output clock independently of captions, cut rates and disabled sources', () => {
  const plan = clipTimelineFixture().plan
  plan.sourceAudio = plan.cuts.map((cut) => ({
    sourceId: cut.sourceId,
    fingerprint: cut.fingerprint,
    retainOriginalAudio: false,
  }))
  plan.sourceVolumePermille = 250
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 700,
    segments: [
      {
        id: 'spoken',
        text: 'independent speech',
        textRevision: 1,
        inputHash: 'input',
        startMs: 9000,
        endMs: 14000,
        speech: {
          assetId: 'asset',
          voiceId: 'voice',
          bindingDigest: 'binding',
          inputHash: 'input',
          settingsHash: 'settings',
          audioHash: 'audio',
          profileId: 'profile',
          profileRevision: 1,
          samples: 176400,
          sampleRate: 44100,
          channels: 2,
          timing: [],
        },
      },
    ],
  }
  const before = browserAudioPlan(plan)
  expect(before.cuts).toEqual([])
  expect(before.speech[0]).toMatchObject({ start: 9, duration: 4 })
  expect(before.narrationVolume).toBe(0.7)
  plan.elements![0]!.text = 'changed caption'
  expect(browserAudioPlan(plan).speech).toEqual(before.speech)
  plan.narration.segments[0]!.inputHash = 'changed'
  expect(browserAudioPlan(plan).speech).toEqual([])
  expect(browserAudioPlan(plan).speechIssues).toEqual([
    { segmentId: 'spoken', state: 'stale', previous: true },
  ])
  plan.narration.enabled = false
  expect(browserAudioPlan(plan).speechIssues).toEqual([])
})
