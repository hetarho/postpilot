import { afterEach, expect, it, vi } from 'vitest'
afterEach(() => vi.unstubAllGlobals())
it.each(['preview', undefined])(
  'refuses %s snapshots before source, canvas or encoder initialization',
  async (purpose) => {
    vi.resetModules()
    const postMessage = vi.fn(),
      canvas = vi.fn(),
      encoder = vi.fn()
    const worker = { postMessage, onmessage: undefined as unknown as (event: MessageEvent) => void }
    vi.stubGlobal('self', worker)
    vi.stubGlobal('OffscreenCanvas', canvas)
    vi.stubGlobal('VideoEncoder', encoder)
    await import('./video.worker')
    worker.onmessage({
      data: { type: 'start', input: { snapshot: { ...(purpose ? { purpose } : {}) }, assets: [] } },
    } as MessageEvent)
    await vi.waitFor(() => expect(postMessage).toHaveBeenCalled())
    expect(postMessage).toHaveBeenCalledWith(
      { type: 'error', error: 'CLIP_SNAPSHOT_EXPORT_PURPOSE_REQUIRED', measurements: undefined },
      { transfer: [] },
    )
    expect(canvas).not.toHaveBeenCalled()
    expect(encoder).not.toHaveBeenCalled()
    expect(postMessage).toHaveBeenCalledTimes(1)
  },
)
