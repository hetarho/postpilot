import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ClipEditingStateSchema } from '@/shared/api'
import { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
import { copyClipPlan, type ClipEditPlan } from './edit-plan'
import { spokenState, type ClipNarration } from './spoken'

function narration(): ClipNarration {
  return {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 700,
    segments: [
      {
        id: 'spoken-1',
        text: '더빙은 자막과 다릅니다',
        textRevision: 2,
        inputHash: 'input',
        startMs: 123,
        endMs: 3000,
        speech: {
          assetId: 'private',
          voiceId: 'voice',
          bindingDigest: 'binding',
          inputHash: 'input',
          settingsHash: 'settings',
          audioHash: 'audio',
          profileId: 'profile',
          profileRevision: 3,
          samples: 44101,
          sampleRate: 44100,
          channels: 2,
          timing: [{ text: '더빙', startMs: 0, endMs: 800 }],
        },
      },
    ],
  }
}
describe('independent spoken script', () => {
  it('retains independent caption origin and edit protection on the wire', () => {
    const original = create(ClipEditingStateSchema, {
      plan: {
        nativeComposition: true,
        elements: [
          {
            narration: true,
            text: '별도 표시 문구',
            derivedCaption: {
              segmentId: 'spoken-1',
              textRevision: 3,
              textEdited: true,
              timingEdited: false,
            },
          },
        ],
      },
    })
    const plan = toClipEditingState(original).plan
    const copy = copyClipPlan(plan)
    copy.elements![0]!.derivedCaption!.timingEdited = true
    expect(plan.elements![0]!.derivedCaption!.timingEdited).toBe(false)
    const restored = toClipEditingState(
      create(ClipEditingStateSchema, { plan: clipPlanToProto(plan) }),
    ).plan
    expect(restored.elements![0]!.derivedCaption).toEqual({
      segmentId: 'spoken-1',
      textRevision: 3,
      textEdited: true,
      timingEdited: false,
    })
    expect(restored.narration).toBeUndefined()
  })
  it('reads legacy narration-caption flags without introducing generated audio', () => {
    const state = toClipEditingState(
      create(ClipEditingStateSchema, {
        plan: { nativeComposition: true, elements: [{ narration: true, text: '기존 자막' }] },
      }),
    )
    expect(state.plan.elements?.[0]?.narration).toBe(true)
    expect(state.plan.narration).toBeUndefined()
    expect(state.plan.sourceVolumePermille).toBeUndefined()
  })
  it('roundtrips separate gains, script revision, provenance and measured timing', () => {
    const plan: ClipEditPlan = {
      durationMs: 15000,
      cuts: [],
      narration: narration(),
      sourceVolumePermille: 0,
    }
    const got = toClipEditingState(
      create(ClipEditingStateSchema, { plan: clipPlanToProto(plan) }),
    ).plan
    expect(got.narration).toEqual(plan.narration)
    expect(got.sourceVolumePermille).toBe(0)
  })
  it('keeps edit copies independent, including speech timing and caption link flags', () => {
    const plan: ClipEditPlan = { durationMs: 15000, cuts: [], narration: narration() }
    const copy = copyClipPlan(plan)
    copy.narration!.segments[0]!.speech!.timing[0]!.text = '변경'
    copy.narration!.segments[0]!.text = '독립 대본'
    expect(plan.narration!.segments[0]!.text).toBe('더빙은 자막과 다릅니다')
    expect(plan.narration!.segments[0]!.speech!.timing[0]!.text).toBe('더빙')
  })
  it('distinguishes absent, changed input, changed voice and measured fit', () => {
    const n = narration(),
      segment = n.segments[0]!
    expect(spokenState(n, segment)).toBe('ready')
    expect(spokenState(n, { ...segment, speech: undefined })).toBe('missing')
    expect(spokenState(n, { ...segment, inputHash: 'changed' })).toBe('stale')
    expect(spokenState({ ...n, confirmedVoiceId: 'another' }, segment)).toBe('stale')
    expect(spokenState(n, { ...segment, endMs: 1123 })).toBe('conflict')
  })
  it('defaults absent gain to unity and preserves an explicit mute', () => {
    for (const [volumePermille, expected] of [
      [undefined, 1000],
      [0, 0],
    ] as const) {
      const got = toClipEditingState(
        create(ClipEditingStateSchema, { plan: { narration: { enabled: true, volumePermille } } }),
      )
      expect(got.plan.narration!.volumePermille).toBe(expected)
    }
  })
})
