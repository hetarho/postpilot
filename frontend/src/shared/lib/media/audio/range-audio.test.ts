import { expect, it } from 'vitest'
import { audioGuardWindow } from './range-audio'

const guards = { decoderPrerollMs: 125, decoderTailMs: 125 }
it('keeps negative AAC priming before a file-start selection and bounded positive guards', () => {
  expect(
    audioGuardWindow(
      { startUs: 0, endUs: 2_000_000, targetSampleRate: 48000 },
      48000,
      -1024 / 48000,
      guards,
    ),
  ).toEqual({ startSample: 0, endSample: 102000, decodeStart: -1024 / 48000, decodeEnd: 2.125 })
  const window = audioGuardWindow(
    { startUs: 123_456_000, endUs: 125_456_000, targetSampleRate: 48000 },
    48000,
    -1024 / 48000,
    guards,
  )
  expect(window.startSample / 48000).toBeCloseTo(123.331)
  expect(window.endSample - window.startSample).toBe(108000)
})
it('aligns every 44.1k guard to the common 48k rational clock without restarting phase', () => {
  for (const startUs of [0, 1_123_000, 4_234_000, 123_456_000]) {
    const window = audioGuardWindow(
      { startUs, endUs: startUs + 2_000_000, targetSampleRate: 48000 },
      44100,
      -1024 / 44100,
      guards,
    )
    expect(window.startSample % 147).toBe(0)
    expect(Number.isInteger((window.startSample * 48000) / 44100)).toBe(true)
    expect(window.decodeStart).toBeLessThanOrEqual(startUs / 1_000_000)
    expect((window.endSample - window.startSample) / 44100).toBeLessThan(2.254)
  }
})
it('refuses malformed source clocks before PCM allocation', () => {
  expect(() =>
    audioGuardWindow({ startUs: -1, endUs: 1, targetSampleRate: 48000 }, 48000, 0, guards),
  ).toThrow('CLIP_SOURCE_AUDIO_TIMESTAMP_INVALID')
  expect(() =>
    audioGuardWindow({ startUs: 0, endUs: 1000, targetSampleRate: 0 }, 48000, 0, guards),
  ).toThrow('CLIP_SOURCE_AUDIO_TIMESTAMP_INVALID')
})
