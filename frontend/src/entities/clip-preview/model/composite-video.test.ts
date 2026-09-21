import { describe, expect, it, vi } from 'vitest'
import type { PreparedAsset } from '@/entities/clip-preview'
import { clipTimelineFixture } from '@/test/clip-editing'
import { compositeBrowserVideo } from './composite-video'
import { RenderRasterCache } from './render-raster-cache'

function bitmap(name: string) {
  return { name, width: 1920, height: 1080, close: vi.fn() } as unknown as ImageBitmap
}
function raster(representativeFrame: boolean, rapid = false): PreparedAsset {
  return {
    key: rapid ? 'rapid' : representativeFrame ? 'sequence' : 'static',
    instanceId: 'caption',
    url: 'server.png',
    x: 40,
    y: 80,
    width: 200,
    height: 50,
    startMs: 1000,
    endMs: 2000,
    // A rapid phrase declares no motion at all (CDS-4); every other style
    // declares its own fade and settle, which the server chain applies too.
    inMs: rapid ? 0 : 200,
    outMs: rapid ? 0 : 200,
    dy: rapid ? 0 : 20,
    layer: 2,
    representativeFrame,
  }
}
async function run(transitionMs = 200) {
  const plan = clipTimelineFixture().plan
  plan.cuts[0] = {
    ...plan.cuts[0],
    startMs: 1000,
    endMs: 5000,
    playbackRatePermille: 2000,
    focal: { x: 1, y: 0.5 },
  }
  plan.cuts[1] = {
    ...plan.cuts[1],
    startMs: 500,
    endMs: 3500,
    playbackRatePermille: 1000,
    transitionMs,
  }
  const sources: { fingerprint: string; timeMs: number; image: ImageBitmap }[] = []
  const draws: { image: ImageBitmap; alpha: number; args: number[]; frame: number }[] = []
  const load = vi.fn(async (asset: PreparedAsset) => bitmap(asset.key))
  const cache = new RenderRasterCache(load)
  const frames: { timestamp: number; duration: number; keyFrame: boolean }[] = []
  const context = {
    globalAlpha: 1,
    fillStyle: '',
    fillRect: vi.fn(),
    drawImage(image: ImageBitmap, ...args: number[]) {
      draws.push({ image, args, alpha: this.globalAlpha, frame: frames.length })
    },
  }
  const progress = vi.fn()
  const result = await compositeBrowserVideo(
    { plan, ratio: 'vertical', assets: [raster(false), raster(true), raster(false, true)] },
    {
      context: context as unknown as OffscreenCanvasRenderingContext2D,
      source: async (fingerprint, timeMs) => {
        const image = bitmap('source')
        sources.push({ fingerprint, timeMs, image })
        return image
      },
      asset: (asset) => cache.get(asset),
      releaseAssets: (keys) => cache.retain(keys),
      encode: async (timestamp, duration, keyFrame) => {
        frames.push({ timestamp, duration, keyFrame })
      },
      progress,
    },
  )
  return { result, sources, draws, load, frames, progress, context }
}
describe('browser video composition', () => {
  it('uses rate-adjusted output intervals, focal cover crop, a 200 ms fade and 30 fps', async () => {
    const value = await run()
    expect(value.result).toEqual({ frameCount: 144, durationUs: 4_800_000 })
    expect(value.context.fillRect).toHaveBeenCalledWith(0, 0, 1080, 1920)
    expect(value.sources[0].timeMs).toBe(1000)
    expect(value.sources[30].timeMs).toBe(3000)
    expect(value.sources.find((source) => source.fingerprint === 'b'.repeat(64))?.timeMs).toBe(500)
    const fade = value.draws.filter((draw) => draw.frame === 57).slice(0, 2)
    expect(fade.map((draw) => draw.alpha)).toEqual([1, 0.5])
    expect(value.draws[0].args[0]).toBeCloseTo(1080 - 1920 * (1920 / 1080))
    expect(value.frames[143].timestamp + value.frames[143].duration).toBe(4_800_000)
    expect(value.frames.filter((frame) => frame.keyFrame)).toHaveLength(3)
    expect(
      value.sources.every((source) => vi.mocked(source.image.close).mock.calls.length === 1),
    ).toBe(true)
    expect(value.progress).toHaveBeenLastCalledWith({ completedFrames: 144, totalFrames: 144 })
  })
  it('moves every caption the way its style declares, inside the half-open interval alone', async () => {
    const value = await run()
    expect(value.load).toHaveBeenCalledTimes(3)
    // A static style and a sequence style both animate: the server fades and
    // settles a static plate from the same manifest numbers (CDS-4, CLIP-157).
    for (const i of [0, 1]) {
      const image = await value.load.mock.results[i].value
      const draws = value.draws.filter((draw) => draw.image === image)
      expect(draws.map((draw) => draw.frame)).toEqual(Array.from({ length: 30 }, (_, i) => i + 30))
      expect(draws[0].args).toEqual([40, 100, 200, 50])
      expect(draws[0].alpha).toBe(0)
      expect(draws[6].alpha).toBe(1)
      expect(image.close).toHaveBeenCalledOnce()
    }
    // A rapid phrase appears whole and does not move.
    const rapid = await value.load.mock.results[2].value
    const phrase = value.draws.filter((draw) => draw.image === rapid)
    expect(phrase.map((draw) => draw.frame)).toEqual(Array.from({ length: 30 }, (_, i) => i + 30))
    expect(phrase.every((draw) => draw.alpha === 1)).toBe(true)
    expect(phrase.every((draw) => draw.args[1] === 80)).toBe(true)
  })
  it('does not blend a hard cut', async () => {
    const value = await run(0)
    expect(value.result.frameCount).toBe(150)
    expect(value.sources).toHaveLength(150)
    expect(value.draws.filter((draw) => draw.frame === 60)).toHaveLength(1)
    expect(value.sources[60].timeMs).toBe(500)
  })
})
