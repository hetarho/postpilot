import { expect, it } from 'vitest'
import {
  integratedLoudness48k,
  normalizeLoudness48k,
  normalizeMix48k,
  truePeak48k,
  loudnessRange48k,
} from './loudness'

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
  expect(result).toMatchObject({ silent: true, loudnessLUFS: undefined, gain: 1, limited: false })
  expect(silent.every((value) => value === 0)).toBe(true)
})
it('limits a transient-heavy mix to the target where one gain stops at the ceiling', () => {
  // Speech-level programme with near-full-scale clicks, as on dishes or doors.
  const clicked = () => {
    const input = Float32Array.from(
      { length: 6 * 48000 },
      (_, i) => 0.02 * Math.sin((2 * Math.PI * 1000 * i) / 48000),
    )
    for (let at = 6000; at < input.length; at += 12000) input[at] = 0.99
    return input
  }
  const channels = [clicked(), clicked()]
  const before = integratedLoudness48k(channels)
  expect(-16 - before).toBeGreaterThan(-1.5 - truePeak48k(channels))
  const result = normalizeLoudness48k(channels, -16, -1.5)
  expect(result.limited).toBe(true)
  expect(Math.abs(result.loudnessLUFS! + 16)).toBeLessThanOrEqual(0.5)
  expect(result.truePeakDBTP).toBeLessThanOrEqual(-2)
  expect(integratedLoudness48k(channels)).toBeCloseTo(result.loudnessLUFS!, 6)
  expect(truePeak48k(channels)).toBeCloseTo(result.truePeakDBTP, 6)
})
it('keeps the one-gain path untouched when the ceiling does not bind', () => {
  const channels = [tone(), tone()]
  const expected = channels.map((channel) => channel.slice())
  const result = normalizeLoudness48k(channels, -16, -1.5)
  expect(result.limited).toBe(false)
  for (const channel of expected) for (let i = 0; i < channel.length; i++) channel[i] *= result.gain
  expect(channels).toEqual(expected)
})

it.each([
  [[-20, -30], 10],
  [[-20, -15], 5],
  [[-40, -20], 20],
  [[-50, -35, -20, -35, -50], 15],
] as const)(
  'matches EBU Tech3342 standard tone sequence %s within its1LU tolerance',
  (levels, expected) => {
    const input = Float32Array.from(
      { length: levels.length * 20 * 48000 },
      (_, index) =>
        10 ** (levels[Math.floor(index / (20 * 48000))] / 20) *
        Math.sin((2 * Math.PI * 1000 * index) / 48000),
    )
    expect(Math.abs(loudnessRange48k([input, input]) - expected)).toBeLessThan(1)
  },
)
it('does not mistake a global gain for dynamic-range normalization', () => {
  const input = Float32Array.from(
    { length: 40 * 48000 },
    (_, index) =>
      (index < 20 * 48000 ? 0.01 : 0.1) * Math.sin((2 * Math.PI * 1000 * index) / 48000),
  )
  const before = loudnessRange48k([input, input])
  normalizeLoudness48k([input], -16, -1.5)
  expect(before).toBeGreaterThan(19)
  expect(loudnessRange48k([input, input])).toBeCloseTo(before, 2)
  expect(loudnessRange48k([new Float32Array(48000)])).toBe(0)
})
it('refuses a mix wider than the loudness range ceiling before applying any gain', () => {
  const input = Float32Array.from(
    { length: 40 * 48000 },
    (_, index) =>
      (index < 20 * 48000 ? 0.01 : 0.1) * Math.sin((2 * Math.PI * 1000 * index) / 48000),
  )
  const sample = input[30 * 48000 + 12]
  expect(() => normalizeMix48k([input, input.slice()], -16, -1.5, 11)).toThrow(
    'AUDIO_LOUDNESS_RANGE_UNSUPPORTED',
  )
  expect(input[30 * 48000 + 12]).toBe(sample)
})
