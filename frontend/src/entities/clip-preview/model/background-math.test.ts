import { describe, expect, it } from 'vitest'
import {
  backgroundLuminance,
  backgroundSampleFrames,
  browserGroundDecisions,
  meanRegionPixels,
  nativeFadeBlackPixel,
  summarizeBrowserGround,
} from './background-math'

describe('native caption background math', () => {
  it('seeks output first/middle/last with ceil and clips the final real frame', () => {
    expect(backgroundSampleFrames(101, 1000, 1800, 30)).toEqual([4, 17, 30])
    expect(backgroundSampleFrames(59000, 60000, 1800, 30)).toEqual([1770, 1785, 1799])
  })
  it('measures luminance of RGB means and temporal sigma, preserving threshold controls', () => {
    const low = summarizeBrowserGround([
      [201 / 255, 203 / 255, 203 / 255],
      [201 / 255, 203 / 255, 203 / 255],
      [201 / 255, 203 / 255, 203 / 255],
    ])
    const high = summarizeBrowserGround([
      [202 / 255, 204 / 255, 204 / 255],
      [202 / 255, 204 / 255, 204 / 255],
      [202 / 255, 204 / 255, 204 / 255],
    ])
    expect(browserGroundDecisions(low)).toEqual({ scrim: false, accentWhite: false })
    expect(browserGroundDecisions(high)).toEqual({ scrim: true, accentWhite: true })
    const noise = summarizeBrowserGround([
      [0, 0, 0],
      [1, 1, 1],
      [0, 0, 0],
    ])
    expect(noise.mean).toBeCloseTo(1 / 3)
    expect(noise.sigma).toBeCloseTo(Math.sqrt(2 / 9))
    expect(browserGroundDecisions(noise)).toEqual({ scrim: true, accentWhite: false })
    expect(backgroundLuminance([1, 1, 1])).toBeCloseTo(1)
  })
  it('uses every second region pixel rather than full-frame averages', () => {
    const data = new Uint8ClampedArray(4 * 4 * 4).fill(255)
    for (const [x, y] of [
      [0, 0],
      [2, 0],
      [0, 2],
      [2, 2],
    ])
      for (let c = 0; c < 3; c++) data[(y! * 4 + x!) * 4 + c] = 0
    expect(meanRegionPixels(data, 4, 4)).toEqual([0, 0, 0])
  })
  it('uses clipped limited-YUV fadeblack, not a plain RGB fade', () => {
    const value = nativeFadeBlackPixel([1, 1, 1], [1, 1, 1], 0.2, 0)
    expect(value[0]).toBeLessThan(0.2)
    expect(value[0]).toBeCloseTo(((0.2 * 235 - 16) * 1.164384) / 255, 5)
    expect(nativeFadeBlackPixel([1, 1, 1], [1, 1, 1], 0, 0)).toEqual([0, 0, 0])
    expect(() => backgroundLuminance([NaN, 0, 0])).toThrow('CLIP_BACKGROUND_INVALID')
    expect(() => summarizeBrowserGround([])).toThrow('CLIP_BACKGROUND_MISSING')
  })
})
