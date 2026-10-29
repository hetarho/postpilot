import { afterEach, describe, expect, it, vi } from 'vitest'
import { measureOriginalMedia } from './analysis-copy'

const state = vi.hoisted(() => {
  vi.resetModules()
  return {
    times: [0, 62],
    durations: [1, 1],
    declared: 1 as number | null,
    timeResolution: 1000,
    seen: [] as number[],
    closed: [] as number[],
    returned: 0,
    dispose: vi.fn(),
  }
})
vi.mock('./video/range-video', async (original) => ({
  ...(await original<object>()),
  openOriginalVideo: vi.fn(async () => ({
    metadata: {
      provenance: 'browser_client',
      codedWidth: 16,
      codedHeight: 16,
      displayWidth: 16,
      displayHeight: 16,
      timeResolution: state.timeResolution,
      durationFromMetadata: state.declared,
    },
    async *samples(start = 0, end = Infinity) {
      try {
        for (const [index, timestamp] of state.times.entries()) {
          if (!Number.isFinite(timestamp) || (timestamp >= start && timestamp < end)) {
            state.seen.push(timestamp)
            yield {
              timestamp,
              duration: state.durations[index],
              close: () => state.closed.push(timestamp),
            }
          }
        }
      } finally {
        state.returned++
      }
    },
    dispose: state.dispose,
  })),
}))
vi.mock('mediabunny', async (original) => ({
  ...(await original<object>()),
  canEncodeVideo: vi.fn(async () => true),
  Input: class {
    async getAudioTracks() {
      return []
    }
    dispose() {}
  },
}))
const limits = {
  durationMs: 60000,
  fps: 15,
  longEdge: 720,
  videoBitrates: [900000, 650000],
  audioBitrate: 96000,
  audioRate: 48000,
  decoderReserveFrames: 48,
  decoderReserveBytes: 1073741824,
  maxCopyBytes: 8388608,
  maxFileBytes: 2147483648,
  maxReadBytes: 4194304,
  maxReadTotalBytes: 4294967296,
  maxCacheBytes: 1048576,
  timeoutMs: 30000,
}
const access = { kind: 'blob' as const, blob: new Blob(['source']) }
afterEach(() => {
  state.times = [0, 62]
  state.durations = [1, 1]
  state.declared = 1
  state.timeResolution = 1000
  state.seen = []
  state.closed = []
  state.returned = 0
  state.dispose.mockClear()
  vi.restoreAllMocks()
})
describe('original EOF measurement source-time admission', () => {
  it.each([90000, 1000000])(
    'keeps measured VFR admissible at time resolution %s',
    async (resolution) => {
      state.timeResolution = resolution
      state.declared = null
      state.times = Array.from({ length: 300 }, (_, i) => (i * 3003 - Math.floor(i / 2)) / 90000)
      state.durations = state.times.map(() => 3003 / 90000)
      const measured = await measureOriginalMedia(access, limits, new AbortController().signal)
      const end = state.times.at(-1)! + state.durations.at(-1)!
      const rate = measured.frameRateNumerator / measured.frameRateDenominator
      expect(measured.frameRateNumerator).toBeGreaterThan(0)
      expect(measured.frameRateNumerator).toBeLessThanOrEqual(1000000)
      expect(measured.frameRateDenominator).toBeGreaterThan(0)
      expect(measured.frameRateDenominator).toBeLessThanOrEqual(1000000)
      expect(rate).toBeCloseTo(300 / end, 4)
      expect(measured).toMatchObject({
        decodedFrames: 300,
        durationMs: Math.round(end * 1000),
        cadenceVerified: false,
      })
      expect(state.closed).toEqual(state.times)
      expect(state.dispose).toHaveBeenCalledOnce()
    },
  )
  it('preserves exact NTSC source cadence independently of the analysis frame rate', async () => {
    state.timeResolution = 90000
    state.declared = null
    state.times = [0, 1001 / 30000, 2002 / 30000]
    state.durations = state.times.map(() => 1001 / 30000)
    expect(await measureOriginalMedia(access, limits, new AbortController().signal)).toMatchObject({
      frameRateNumerator: 30000,
      frameRateDenominator: 1001,
      cadenceVerified: true,
      decodedFrames: 3,
    })
  })
  it('does not claim exact cadence for an approximated high-resolution constant rate', async () => {
    state.timeResolution = 1000000000
    state.declared = null
    state.times = [0, 0.033333333, 0.066666666]
    state.durations = state.times.map(() => 0.033333333)
    const measured = await measureOriginalMedia(access, limits, new AbortController().signal)
    expect(measured.cadenceVerified).toBe(false)
    expect(measured.frameRateNumerator).toBeLessThanOrEqual(1000000)
    expect(measured.frameRateDenominator).toBeLessThanOrEqual(1000000)
    expect(measured.frameRateNumerator / measured.frameRateDenominator).toBeCloseTo(30, 4)
  })
  it.each([1, null])(
    'refuses sparse out-of-budget PTS with declared duration %s and releases every observed frame',
    async (declared) => {
      state.declared = declared
      await expect(
        measureOriginalMedia(access, limits, new AbortController().signal),
      ).rejects.toThrow('CLIP_INPUT_TOO_LARGE')
      expect(state.seen).toEqual([0, 62])
      expect(state.closed).toEqual([0, 62])
      expect(state.returned).toBe(1)
      expect(state.dispose).toHaveBeenCalledOnce()
    },
  )
  it('refuses a frame whose start is inside but actual end exceeds the remaining source budget', async () => {
    state.times = [0, 59]
    state.durations = [1, 2]
    await expect(
      measureOriginalMedia(access, limits, new AbortController().signal),
    ).rejects.toThrow('CLIP_INPUT_TOO_LARGE')
    expect(state.closed).toEqual([0, 59])
  })
  it.each([1, null])(
    'reaches actual EOF and measures valid in-budget gapped VFR independently of header %s',
    async (declared) => {
      state.declared = declared
      state.times = [0, 2, 5.5]
      state.durations = [0.5, 0.5, 0.5]
      const measured = await measureOriginalMedia(access, limits, new AbortController().signal)
      expect(measured).toMatchObject({
        provenance: 'browser_client',
        durationMs: 6000,
        decodedFrames: 3,
        cadenceVerified: false,
        frameRateNumerator: 1,
        frameRateDenominator: 2,
      })
      expect(state.seen).toEqual([0, 2, 5.5])
      expect(state.closed).toEqual(state.seen)
      expect(state.returned).toBe(1)
    },
  )
  it('refuses a nonfinite actual timestamp and closes the sample', async () => {
    state.times = [NaN]
    state.durations = [1]
    await expect(
      measureOriginalMedia(access, limits, new AbortController().signal),
    ).rejects.toThrow('CLIP_SOURCE_TIMESTAMP_INVALID')
    expect(state.closed).toHaveLength(1)
  })
  it('refuses an exhausted elapsed-time budget while releasing the current sample', async () => {
    vi.spyOn(performance, 'now').mockReturnValueOnce(0).mockReturnValue(30001)
    await expect(
      measureOriginalMedia(access, limits, new AbortController().signal),
    ).rejects.toThrow('CLIP_SOURCE_TIMEOUT')
    expect(state.closed).toEqual([0])
  })
})
