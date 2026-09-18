import { expect, it } from 'vitest'
import { stretchStereo } from './stretch'

it.each([0.5, 0.75, 1, 1.25, 1.5, 2])(
  'preserves a tone pitch at %sx, exact length, stereo separation and cut volume',
  (rate) => {
    const input = [1000, 500].map((hz) =>
      Float32Array.from(
        { length: 96000 },
        (_, i) => 0.1 * Math.sin((2 * Math.PI * hz * i) / 48000),
      ),
    )
    const output = stretchStereo(input, 48000, rate, Math.round(96000 / rate), 0.25)
    for (const [channel, expected] of [1000, 500].entries()) {
      expect(output[channel]).toHaveLength(Math.round(96000 / rate))
      const begin = 10000,
        end = output[channel].length - 10000
      let crossings = 0,
        peak = 0
      for (let i = begin; i < end; i++) {
        if (output[channel][i - 1] <= 0 && output[channel][i] > 0) crossings++
        peak = Math.max(peak, Math.abs(output[channel][i]))
      }
      expect((crossings * 48000) / (end - begin)).toBeCloseTo(expected, -1)
      expect(peak).toBeCloseTo(0.025, 4)
    }
    if (rate === 1) expect(output[0][12]).toBeCloseTo(input[0][12] * 0.25, 8)
  },
)
