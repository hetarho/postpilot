import { expect, it } from 'vitest'
import { integratedLoudness48k, normalizeLoudness48k, truePeak48k } from './loudness'

const tone = (seconds = 3) =>
  Float32Array.from(
    { length: seconds * 48000 },
    (_, i) => 0.1 * Math.sin((2 * Math.PI * 1000 * i) / 48000),
  )
it('measures a known 1 kHz stereo tone and reaches the target with one gain', () => {
  const channels = [tone(), tone()]
  expect(integratedLoudness48k(channels)).toBeCloseTo(-20.04, 1)
  const sample = channels[0][12]
  const result = normalizeLoudness48k(channels, -16, -1.5)
  expect(result.silent).toBe(false)
  expect(result.loudnessLUFS).toBeCloseTo(-16, 4)
  expect(channels[0][12]).toBeCloseTo(sample * result.gain, 7)
  expect(truePeak48k(channels)).toBeCloseTo(result.truePeakDBTP, 4)
})
it('gates silence around programme material and leaves digital silence unchanged', () => {
  const padded = new Float32Array(9 * 48000)
  padded.set(tone(), 3 * 48000)
  expect(integratedLoudness48k([padded])).toBeGreaterThan(-24)
  const silent = new Float32Array(48000)
  const result = normalizeLoudness48k([silent, silent.slice()], -16, -1.5)
  expect(result).toMatchObject({ silent: true, loudnessLUFS: undefined, gain: 1 })
  expect(silent.every((value) => value === 0)).toBe(true)
})
it('preserves the peak ceiling instead of compressing a high-crest signal', () => {
  const input = tone()
  input[48000] = 0.99
  const result = normalizeLoudness48k([input], -16, -1.5)
  expect(result.truePeakDBTP).toBeCloseTo(-1.5, 6)
  expect(result.loudnessLUFS!).toBeLessThan(-17)
  expect(truePeak48k([input])).toBeLessThanOrEqual(-1.499)
})
