import { expect, it } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  clipBrowserEncoderConfig,
  clipBrowserRenderCapability,
  clipRenderNeedsAudio,
} from './browser-render-capability'

it('refuses exactly the missing capability with sufficient memory, including AAC alone', () => {
  expect(
    clipBrowserRenderCapability({ video: false, audio: true, deviceMemoryGB: 8 }, true),
  ).toEqual({ available: false, reason: 'capability' })
  expect(
    clipBrowserRenderCapability({ video: true, audio: false, deviceMemoryGB: 8 }, true),
  ).toEqual({ available: false, reason: 'capability' })
})

it('refuses only memory with both encoders admitted, without guessing unreported memory', () => {
  expect(
    clipBrowserRenderCapability({ video: true, audio: true, deviceMemoryGB: 1 }, true),
  ).toEqual({ available: false, reason: 'memory' })
  expect(clipBrowserRenderCapability({ video: true, audio: true }, true)).toEqual({
    available: true,
  })
  expect(
    clipBrowserRenderCapability({ video: true, audio: true, deviceMemoryGB: 2 }, true),
  ).toEqual({ available: true })
})

it('needs AAC only for a plan retaining original audio, including a silent volume setting', () => {
  const plan = clipTimelineFixture().plan
  plan.sourceAudio = plan.cuts.map((c) => ({
    sourceId: c.sourceId,
    fingerprint: c.fingerprint,
    retainOriginalAudio: false,
  }))
  expect(clipRenderNeedsAudio(plan)).toBe(false)
  expect(
    clipBrowserRenderCapability({ video: true, audio: false, deviceMemoryGB: 2 }, false),
  ).toEqual({ available: true })
  plan.sourceAudio[0].retainOriginalAudio = true
  plan.cuts[0].volumePermille = 0
  expect(clipRenderNeedsAudio(plan)).toBe(true)
})

it.each([
  ['vertical', 1080, 1920],
  ['horizontal', 1920, 1080],
  ['square', 1080, 1080],
] as const)('asks for the V12 contract at the %s canvas', (ratio, width, height) => {
  const config = clipBrowserEncoderConfig(ratio)
  expect(config.video).toMatchObject({ codec: 'avc1.640028', width, height, framerate: 30 })
  expect(config.audio).toMatchObject({ codec: 'mp4a.40.2', sampleRate: 48000 })
})
