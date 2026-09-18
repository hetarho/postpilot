import { describe, expect, it, vi } from 'vitest'
import { type BrowserVideoTrack, type ClipProject } from '@/entities/clip-project'
import {
  BrowserRenderVerdictError,
  storeBrowserResult,
  type BrowserResultStore,
} from './store-result'

function fixture() {
  const events: string[] = []
  const project = {
    id: 'project',
    result: { id: 'stored', viewUrl: 'https://private.example/clip.mp4' },
  } as ClipProject
  const video: BrowserVideoTrack = {
    config: { codec: 'avc1.640028', width: 1080, height: 1080, framerate: 30 },
    decoderConfig: { codec: 'avc1.640028', codedWidth: 1080, codedHeight: 1080 },
    chunks: Array.from({ length: 30 }, (_, i) => ({
      type: i === 0 ? 'key' : 'delta',
      timestamp: Math.round((i * 1e6) / 30),
      duration: Math.round(1e6 / 30),
      data: new Uint8Array([i]),
    })),
    frameCount: 30,
    durationUs: 1e6,
  }
  const file = new Blob(['produced-mp4'], { type: 'video/mp4' })
  const mux = vi.fn(async () => file)
  const store: BrowserResultStore = {
    prepare: vi.fn(async () => {
      events.push('prepare')
      return { putUrl: 'https://storage.example/put', headers: { 'If-None-Match': '*' } }
    }),
    put: vi.fn(async () => {
      events.push('put')
    }),
    report: vi.fn(async (_id, v) => {
      events.push('report')
      return {
        passed: v.passed,
        notices: v.passed ? [] : [{ code: 'render_output_verdict', action: 'shortfall' }],
      }
    }),
    complete: vi.fn(async () => {
      events.push('complete')
      return project
    }),
  }
  const signal = new AbortController().signal
  const run = () =>
    storeBrowserResult('admission', video, undefined, 'square', 1000, store, signal, undefined, mux)
  return { video, file, mux, store, run, events, project }
}

describe('durable browser output', () => {
  it('accepts reordered encoded packets when their presentation covers every frame', async () => {
    const f = fixture()
    const one = f.video.chunks[1]!
    f.video.chunks[1] = f.video.chunks[2]!
    f.video.chunks[2] = one
    expect(await f.run()).toBe(f.project)
    expect(f.events).toEqual(['prepare', 'put', 'report', 'complete'])
  })
  it('stores before reporting, returns only the stored project and consumes local packets', async () => {
    const f = fixture()
    expect(await f.run()).toBe(f.project)
    expect(f.events).toEqual(['prepare', 'put', 'report', 'complete'])
    expect(f.store.prepare).toHaveBeenCalledWith('admission', f.file.size, expect.any(AbortSignal))
    expect(f.store.put).toHaveBeenCalledWith(
      'https://storage.example/put',
      { 'If-None-Match': '*' },
      f.file,
      expect.any(Function),
      expect.any(AbortSignal),
    )
    expect(f.store.report).toHaveBeenCalledWith(
      'admission',
      expect.objectContaining({
        passed: true,
        measurements: expect.objectContaining({
          width: 1080,
          height: 1080,
          videoFrames: 30,
          frameRateNumerator: 30,
          videoCodec: 'h264',
          videoProfile: 'High',
          hasAudio: false,
        }),
      }),
      expect.any(AbortSignal),
    )
    expect(f.video.chunks).toEqual([])
  })
  it('leaves the previous result standing and rejects when storage fails', async () => {
    const f = fixture()
    vi.mocked(f.store.put).mockRejectedValue(new Error('storage unavailable'))
    await expect(f.run()).rejects.toThrow('storage unavailable')
    expect(f.store.report).not.toHaveBeenCalled()
    expect(f.store.complete).not.toHaveBeenCalled()
    expect(f.video.chunks).toEqual([])
  })
  it('reports actual encoder output and a failed verdict without accepting a local file', async () => {
    const f = fixture()
    f.video.config.width = 720
    f.video.decoderConfig.codec = 'avc1.42001f'
    f.video.chunks.pop()
    await expect(f.run()).rejects.toBeInstanceOf(BrowserRenderVerdictError)
    expect(f.store.prepare).not.toHaveBeenCalled()
    expect(f.store.complete).not.toHaveBeenCalled()
    expect(f.store.report).toHaveBeenCalledWith(
      'admission',
      expect.objectContaining({
        passed: false,
        measurements: expect.objectContaining({
          width: 720,
          videoFrames: 29,
          videoProfile: 'avc1.42001f',
        }),
      }),
      expect.any(AbortSignal),
    )
  })
  it('does not complete a cancelled upload even if its transport resolves late', async () => {
    const f = fixture(),
      controller = new AbortController()
    vi.mocked(f.store.put).mockImplementation(async () => {
      controller.abort()
    })
    await expect(
      storeBrowserResult(
        'admission',
        f.video,
        undefined,
        'square',
        1000,
        f.store,
        controller.signal,
        undefined,
        f.mux,
      ),
    ).rejects.toMatchObject({ name: 'AbortError' })
    expect(f.store.report).not.toHaveBeenCalled()
    expect(f.store.complete).not.toHaveBeenCalled()
  })
  it('keeps server notices when its verdict refuses promotion after PUT', async () => {
    const f = fixture()
    vi.mocked(f.store.report).mockResolvedValue({
      passed: false,
      notices: [{ code: 'render_output_audio', action: 'shortfall' }],
    })
    await expect(f.run()).rejects.toMatchObject({
      notices: [{ code: 'render_output_audio', action: 'shortfall' }],
    })
    expect(f.store.complete).not.toHaveBeenCalled()
  })
})
