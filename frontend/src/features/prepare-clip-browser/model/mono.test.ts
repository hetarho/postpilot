import { describe, expect, it } from 'vitest'
import { analysisMono } from './mono'

describe('native analysis mono rematrix', () => {
  it('preserves canonical original mono level and sample positions', () => {
    const source = new Float32Array([0, 0.1, -0.2, 0.3, 0])
    const plane = source.map((value) => value * Math.SQRT1_2)
    const mono = analysisMono([plane, plane.slice()])
    source.forEach((value, index) => expect(mono[index]).toBeCloseTo(value, 7))
  })
  it('uses the measured native coefficient for stereo and preserves anti-phase silence', () => {
    const mono = analysisMono([new Float32Array([0.1, 0.1, 0]), new Float32Array([0.1, -0.1, 0])])
    expect(mono[0]).toBeCloseTo(0.2 * Math.SQRT1_2, 7)
    expect(Array.from(mono.slice(1))).toEqual([0, 0])
  })
  it('refuses incomplete or nonfinite PCM without adding inferred samples', () => {
    expect(() => analysisMono([new Float32Array(2), new Float32Array(1)])).toThrow()
    expect(() => analysisMono([new Float32Array([NaN]), new Float32Array([0])])).toThrow()
  })
})
