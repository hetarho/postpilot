import { describe, expect, it, vi } from 'vitest'
import {
  nativeOutputFrame,
  OriginalVideoCursor,
  selectNativeVideoTrack,
  type OriginalVideoInput,
  type VideoRangeSample,
} from './range-video'
import { VideoFrameBudget } from './frame-budget'

function original(
  times = Array.from({ length: 48 }, (_, n) => n / 60),
  rotation = 0,
  flip = false,
) {
  const samples: { index: number; close: ReturnType<typeof vi.fn> }[] = [],
    frames: { index: number; close: ReturnType<typeof vi.fn> }[] = []
  let returned = 0
  const input: OriginalVideoInput = {
    metadata: {
      provenance: 'browser_client',
      codec: 'avc',
      codedWidth: 2,
      codedHeight: 2,
      displayWidth: 2,
      displayHeight: 2,
      timeResolution: 60000,
      firstTimestamp: 0,
      durationFromMetadata: 0.8,
      rotation,
      flip,
    },
    samples: () =>
      (async function* () {
        try {
          for (const [index, timestamp] of times.entries()) {
            const entry = { index, close: vi.fn() }
            samples.push(entry)
            yield {
              timestamp,
              duration: 1 / 60,
              displayWidth: 2,
              displayHeight: 2,
              rotation,
              flip,
              close: entry.close,
              toVideoFrame: () => {
                const frame = {
                  index,
                  timestamp: Math.round(timestamp * 1_000_000),
                  close: vi.fn(),
                  allocationSize: () => 6,
                }
                frames.push(frame)
                return frame as unknown as VideoFrame
              },
            } satisfies VideoRangeSample
          }
        } finally {
          returned++
        }
      })(),
    dispose: vi.fn(),
  }
  return { input, samples, frames, returned: () => returned }
}
describe('native original timestamp selection', () => {
  it('keeps first stream order regardless of player defaults and permits ordinary all-intra movies', async () => {
    const first = {
      hasOnlyKeyPackets: vi.fn(async () => false),
      computePacketStats: vi.fn(async () => ({ packetCount: 2 })),
      default: false,
    }
    const second = {
      hasOnlyKeyPackets: vi.fn(async () => false),
      computePacketStats: vi.fn(async () => ({ packetCount: 2 })),
      default: true,
    }
    expect(await selectNativeVideoTrack([first, second])).toBe(first)
    first.hasOnlyKeyPackets.mockResolvedValue(true)
    first.computePacketStats.mockResolvedValue({ packetCount: 2 })
    expect(await selectNativeVideoTrack([first, second])).toBe(first)
    expect(first.computePacketStats).toHaveBeenCalledWith(2)
  })
  it('explicitly refuses an ambiguous single key-picture track instead of guessing attached-picture semantics', async () => {
    const track = {
      hasOnlyKeyPackets: vi.fn(async () => true),
      computePacketStats: vi.fn(async () => ({ packetCount: 1 })),
    }
    await expect(selectNativeVideoTrack([track])).rejects.toThrow('CLIP_SOURCE_STREAM_AMBIGUOUS')
    expect(await selectNativeVideoTrack([])).toBeNull()
  })
  it.each([
    [500, [0, 1, 2, 3]],
    [750, [0, 2, 3, 5]],
    [1000, [0, 2, 4, 6]],
    [1250, [1, 3, 6, 8]],
    [1500, [1, 4, 7, 10]],
    [2000, [1, 5, 9, 13]],
  ] as const)(
    'matches real native fps round=near mappings at %spermille',
    async (rate, expected) => {
      const fake = original(),
        budget = new VideoFrameBudget(2, 128)
      const cursor = new OriginalVideoCursor(
        fake.input,
        { startUs: 0, endUs: 800000, ratePermille: rate, fps: 30 },
        budget,
        new AbortController().signal,
      )
      for (const [frame, index] of expected.entries()) {
        const resource = await cursor.frame(frame)
        expect((resource.frame as unknown as { index: number }).index).toBe(index)
        resource.close()
      }
      await cursor.close()
      expect(fake.samples.every((sample) => sample.close.mock.calls.length === 1)).toBe(true)
      expect(fake.frames.every((frame) => frame.close.mock.calls.length === 1)).toBe(true)
      expect(fake.returned()).toBe(1)
      expect(budget.snapshot()).toMatchObject({ liveFrames: 0, liveBytes: 0 })
    },
  )
  it('uses exact integer ties and large source times without milliseconds or floating epsilon', () => {
    expect(nativeOutputFrame(1, 0, 60, 1000, 30)).toBe(1)
    expect(nativeOutputFrame(2, 0, 60, 2000, 30)).toBe(1)
    expect(nativeOutputFrame(2147483647000, 2147483646900, 2147483647, 750, 30)).toBe(0)
  })
  it('discards preceding seek guards and anchors cadence to first sample actually inside the cut', async () => {
    const fake = original(),
      cursor = new OriginalVideoCursor(
        fake.input,
        { startUs: 10000, endUs: 400000, ratePermille: 1250, fps: 30 },
        new VideoFrameBudget(2, 128),
        new AbortController().signal,
      )
    const resource = await cursor.frame(0)
    expect((resource.frame as unknown as { index: number }).index).toBe(2)
    expect(fake.samples[0]!.close).toHaveBeenCalledOnce()
    resource.close()
    await cursor.close()
  })
  it('keeps VFR timestamps and independent reordered range origins', async () => {
    const fake = original([0, 0.01, 0.09, 0.16, 0.25, 0.6])
    const first = new OriginalVideoCursor(
      fake.input,
      { startUs: 160000, endUs: 700000, ratePermille: 1000, fps: 30 },
      new VideoFrameBudget(2, 128),
      new AbortController().signal,
    )
    const early = new OriginalVideoCursor(
      fake.input,
      { startUs: 0, endUs: 100000, ratePermille: 1000, fps: 30 },
      new VideoFrameBudget(2, 128),
      new AbortController().signal,
    )
    const a = await first.frame(0),
      b = await early.frame(0)
    expect(a.timestampUs).toBe(160000)
    expect(b.timestampUs).toBe(10000)
    a.close()
    b.close()
    await first.close()
    await early.close()
  })
  it('draws the same display rectangle with rotation and flip and no bitmap/readback', async () => {
    const fake = original(undefined, 90, true),
      cursor = new OriginalVideoCursor(
        fake.input,
        { startUs: 0, endUs: 800000, ratePermille: 1000, fps: 30 },
        new VideoFrameBudget(2, 128),
        new AbortController().signal,
      )
    const resource = await cursor.frame(0)
    const context = {
      save: vi.fn(),
      restore: vi.fn(),
      translate: vi.fn(),
      scale: vi.fn(),
      rotate: vi.fn(),
      drawImage: vi.fn(),
    }
    resource.draw(context as unknown as OffscreenCanvasRenderingContext2D, {
      x: -10,
      y: 3,
      width: 40,
      height: 20,
    })
    expect(context.translate).toHaveBeenCalledWith(10, 13)
    expect(context.scale).toHaveBeenCalledWith(-1, 1)
    expect(context.rotate).toHaveBeenCalledWith(Math.PI / 2)
    expect(context.drawImage).toHaveBeenCalledWith(resource.frame, -10, -20, 20, 40)
    resource.close()
    await cursor.close()
  })
  it('keeps borrowed frame leases valid while source samples advance, then refuses backward seeks', async () => {
    const fake = original(),
      cursor = new OriginalVideoCursor(
        fake.input,
        { startUs: 0, endUs: 800000, ratePermille: 1000, fps: 30 },
        new VideoFrameBudget(2, 128),
        new AbortController().signal,
      )
    const first = await cursor.frame(0),
      next = await cursor.frame(1)
    expect(fake.samples[0]!.close).toHaveBeenCalledOnce()
    expect(fake.frames[0]!.close).not.toHaveBeenCalled()
    await expect(cursor.frame(0)).rejects.toThrow('CLIP_SOURCE_SUPERSEDED')
    await cursor.close()
    expect(fake.frames.every((frame) => frame.close.mock.calls.length === 1)).toBe(true)
    expect(() =>
      first.draw({} as OffscreenCanvasRenderingContext2D, { x: 0, y: 0, width: 2, height: 2 }),
    ).toThrow('CLIP_SOURCE_SUPERSEDED')
    next.close()
  })
})
