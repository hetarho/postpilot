import { describe, expect, it, vi } from 'vitest'
import { BrowserInkCache } from './ink-cache'
import type { InkDocument } from './ink-raster'

const document = (key: string): InkDocument => ({
  key,
  svg: '<svg/>',
  fonts: [],
  bounds: { x: 0, y: 0, width: 10, height: 10 },
})
const bitmap = () => ({ width: 10, height: 10, close: vi.fn() }) as unknown as ImageBitmap
describe('bounded reusable browser ink', () => {
  it('reserves physical doubled-mask bytes and rejects a renderer that returns the wrong size', async () => {
    const scaled = { ...document('scaled'), rasterScale: 2 }
    const close = vi.fn()
    const cache = new BrowserInkCache(
      async () => ({ width: 20, height: 20, close }) as unknown as ImageBitmap,
      { bytes: 1600, entries: 1 },
    )
    const lease = await cache.acquire(scaled)
    expect(cache.measurements().bytes).toBe(1600)
    await expect(cache.acquire(document('another'))).rejects.toThrow('CLIP_INK_RESOURCE_LIMIT')
    lease.close()
    cache.destroy()
    expect(close).toHaveBeenCalledOnce()
    const wrong = bitmap()
    const invalid = new BrowserInkCache(async () => wrong)
    await expect(invalid.acquire(scaled)).rejects.toThrow(
      'CLIP_INK_INVALID_GEOMETRY:raster dimensions',
    )
    expect(wrong.close).toHaveBeenCalledOnce()
    expect(invalid.measurements()).toMatchObject({ entries: 0, bytes: 0, leases: 0, rasters: 0 })
  })
  it.each([NaN, Infinity, -1, 0, 1.5, 3])(
    'rejects unversioned raster scale %s before rasterization',
    async (rasterScale) => {
      const render = vi.fn(async () => bitmap()),
        cache = new BrowserInkCache(render)
      await expect(cache.acquire({ ...document('invalid'), rasterScale })).rejects.toThrow(
        'CLIP_INK_RESOURCE_LIMIT:raster scale',
      )
      expect(render).not.toHaveBeenCalled()
      expect(cache.measurements().bytes).toBe(0)
    },
  )
  it('refuses GPU creation from a closed lease and destroys each owned texture once', async () => {
    const cache = new BrowserInkCache(async () => bitmap()),
      owner = {},
      destroy = vi.fn()
    const lease = await cache.acquire(document('owned'))
    lease.gpu(owner, () => ({ value: {}, destroy }))
    lease.close()
    expect(() => lease.gpu(owner, () => ({ value: {}, destroy }))).toThrow('CLIP_INK_CANCELLED')
    cache.dropGPU(owner)
    cache.dropGPU(owner)
    cache.destroy()
    expect(destroy).toHaveBeenCalledOnce()
  })
  it('reuses glyph ink across fractional movement while compensating the original raster phase', async () => {
    const render = vi.fn(async () => bitmap())
    const cache = new BrowserInkCache(render)
    const first = await cache.acquire({ ...document('same-ink'), phase: { x: 0.25, y: 0.75 } })
    first.close()
    const moved = await cache.acquire({ ...document('same-ink'), phase: { x: 0.75, y: 0.25 } })
    expect(moved.offset).toEqual({ x: 0.5, y: -0.5 })
    expect(render).toHaveBeenCalledOnce()
    moved.close()
    cache.destroy()
  })
  it('protects presentation leases, reuses a GPU texture, and destroys CPU/GPU resources on eviction', async () => {
    const a = bitmap(),
      b = bitmap(),
      render = vi.fn().mockResolvedValueOnce(a).mockResolvedValueOnce(b)
    const cache = new BrowserInkCache(render, { bytes: 400, entries: 1 })
    const first = await cache.acquire(document('a'))
    const owner = {},
      destroy = vi.fn(),
      create = vi.fn(() => ({ value: { texture: 1 }, destroy }))
    expect(first.gpu(owner, create)).toBe(first.gpu(owner, create))
    await expect(cache.acquire(document('b'))).rejects.toThrow('CLIP_INK_RESOURCE_LIMIT')
    first.close()
    const replay = await cache.acquire(document('a'))
    expect(replay.bitmap).toBe(a)
    replay.close()
    const second = await cache.acquire(document('b'))
    expect(a.close).toHaveBeenCalledOnce()
    expect(destroy).toHaveBeenCalledOnce()
    expect(create).toHaveBeenCalledOnce()
    expect(render).toHaveBeenCalledTimes(2)
    expect(cache.measurements().peakBytes).toBe(400)
    second.close()
    cache.destroy()
    expect(b.close).toHaveBeenCalledOnce()
    expect(cache.measurements().bytes).toBe(0)
  })
  it('closes a late bitmap after cancellation instead of retaining or returning it', async () => {
    let complete!: (value: ImageBitmap) => void
    const value = bitmap()
    const render = vi.fn(
      () =>
        new Promise<ImageBitmap>((resolve) => {
          complete = resolve
        }),
    )
    const cache = new BrowserInkCache(render)
    const pending = cache.acquire(document('a'))
    const refusal = expect(pending).rejects.toThrow('CLIP_INK_CANCELLED')
    await Promise.resolve()
    cache.destroy()
    complete(value)
    await refusal
    expect(value.close).toHaveBeenCalledOnce()
    expect(cache.measurements().entries).toBe(0)
  })
  it('serializes raster work and shares one pending request across frame borrowers', async () => {
    let active = 0,
      peak = 0
    const cache = new BrowserInkCache(async () => {
      active++
      peak = Math.max(peak, active)
      await Promise.resolve()
      active--
      return bitmap()
    })
    const leases = await Promise.all([
      cache.acquire(document('a')),
      cache.acquire(document('a')),
      cache.acquire(document('b')),
    ])
    expect(peak).toBe(1)
    expect(leases[0]!.bitmap).toBe(leases[1]!.bitmap)
    expect(cache.measurements().rasters).toBe(2)
    leases.forEach((lease) => lease.close())
    cache.destroy()
  })
})
