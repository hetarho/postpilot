import { expect, it } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { clipRenderNeedsAudio } from '@/entities/clip-preview'
import { browserAudioPreflight } from './audio-preflight'
function readyBrowserSpeech() {
  const plan = clipTimelineFixture().plan
  plan.sourceAudio = plan.cuts.map((c) => ({
    sourceId: c.sourceId,
    fingerprint: c.fingerprint,
    retainOriginalAudio: false,
  }))
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 0,
    segments: [
      {
        id: 'spoken-1',
        text: 'sentence',
        textRevision: 1,
        inputHash: 'input',
        startMs: 1000,
        endMs: 2000,
        speech: {
          assetId: 'asset',
          voiceId: 'voice',
          bindingDigest: 'binding',
          inputHash: 'input',
          settingsHash: 'settings',
          audioHash: 'audio',
          profileId: 'profile',
          profileRevision: 1,
          samples: 44100,
          sampleRate: 44100,
          channels: 2,
          timing: [],
        },
      },
    ],
  }
  return plan
}
it('requires AAC for source-off narration, including saved gain zero', () => {
  const p = readyBrowserSpeech()
  expect(clipRenderNeedsAudio(p)).toBe(true)
  expect(browserAudioPreflight(p).schedule).toMatchObject({
    cuts: [],
    speech: [{ start: 1, duration: 1 }],
    narrationVolume: 0,
  })
})
it.each(['missing', 'stale', 'conflict', 'voice'] as const)(
  'refuses %s requested speech before decoding or admission',
  (kind) => {
    const p = readyBrowserSpeech(),
      s = p.narration!.segments[0]!
    if (kind === 'missing') s.speech = undefined
    if (kind === 'stale') s.inputHash = 'changed'
    if (kind === 'conflict') s.endMs = 1500
    if (kind === 'voice') p.narration!.confirmedVoiceId = ''
    expect(() => browserAudioPreflight(p)).toThrow('CLIP_BROWSER_AUDIO_SPEECH')
  },
)
it('refuses canonical samples beyond the rounded video endpoint', () => {
  const p = readyBrowserSpeech()
  p.cuts = [{ ...p.cuts[0]!, startMs: 0, endMs: 15010, transitionMs: 0 }]
  p.durationMs = 15010
  Object.assign(p.narration!.segments[0]!, { startMs: 14010, endMs: 15010 })
  expect(() => browserAudioPreflight(p)).toThrow('CLIP_BROWSER_AUDIO_SPEECH')
})
it('refuses an excessive PCM budget before native allocation', () => {
  const p = readyBrowserSpeech()
  p.narration = undefined
  p.cuts = [{ ...p.cuts[0]!, startMs: 0, endMs: 300000, transitionMs: 0 }]
  expect(() => browserAudioPreflight(p)).toThrow('CLIP_BROWSER_AUDIO_MEMORY')
})
