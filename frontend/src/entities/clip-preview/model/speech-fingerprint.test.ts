import { expect, it } from 'vitest'
import { speechRenderFingerprint } from './speech-fingerprint'
import { clipTimelineFixture } from '@/test/clip-editing'
it('matches the server UTF-8 audio/placement fingerprint and includes independent gain', async () => {
  const plan = clipTimelineFixture().plan
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 700,
    segments: [
      {
        id: 'spoken-1',
        text: 'script',
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
          timing: [{ text: '한글', startMs: 0, endMs: 1000 }],
        },
      },
    ],
  }
  expect(await speechRenderFingerprint(plan)).toBe(
    '79a6051cecf7e220ba310bb6fe4705314f2d9cbf8de6b071118abaee9e40f23f',
  )
  plan.narration.volumePermille++
  expect(await speechRenderFingerprint(plan)).not.toBe(
    '79a6051cecf7e220ba310bb6fe4705314f2d9cbf8de6b071118abaee9e40f23f',
  )
  plan.narration.enabled = false
  expect(await speechRenderFingerprint(plan)).toBe('')
})
