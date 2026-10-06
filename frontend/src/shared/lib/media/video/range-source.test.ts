import { describe, expect, it, vi } from 'vitest'
import { createFiniteMediaSource } from './range-source'
const limits = {
  maxFileBytes: 100,
  maxReadBytes: 16,
  maxReadTotalBytes: 32,
  maxCacheBytes: 16,
  timeoutMs: 1000,
}
const response = (start: number, end: number, size = 80, bytes = new Uint8Array(end - start)) =>
  new Response(bytes, {
    status: 206,
    headers: {
      'Content-Range': `bytes ${start}-${end - 1}/${size}`,
      'Content-Length': String(bytes.length),
    },
  })

describe('finite original access', () => {
  it('bootstraps signed GET with one byte and performs only requested finite ranges', async () => {
    const fetcher = vi.fn(async (_url: unknown, init?: RequestInit) => {
      const range = (init!.headers as Record<string, string>).Range.match(/bytes=(\d+)-(\d+)/u)!
      return response(Number(range[1]), Number(range[2]) + 1)
    }) as unknown as typeof fetch
    const reader = createFiniteMediaSource(
      { kind: 'url', url: 'private-url' },
      limits,
      new AbortController().signal,
      { fetch: fetcher },
    )
    expect(await reader.getSize()).toBe(80)
    expect(await reader.read(20, 28)).toHaveLength(8)
    expect(
      vi
        .mocked(fetcher)
        .mock.calls.map(([, options]) => (options!.headers as Record<string, string>).Range),
    ).toEqual(['bytes=0-0', 'bytes=20-27'])
    expect(
      vi.mocked(fetcher).mock.calls.every(([, options]) => options!.method === undefined),
    ).toBe(true)
    expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 8, largestReadBytes: 8 })
    reader.dispose()
  })
  it.each([200, 403, 404])('rejects HTTP %s and cancels its unopened body', async (status) => {
    const cancelled = vi.fn()
    const body = new ReadableStream<Uint8Array>({ cancel: cancelled })
    const reader = createFiniteMediaSource(
      { kind: 'url', url: 'private-url' },
      limits,
      new AbortController().signal,
      { fetch: vi.fn(async () => new Response(body, { status })) },
    )
    await expect(reader.getSize()).rejects.toThrow(
      status === 403
        ? 'CLIP_SOURCE_EXPIRED'
        : status === 404
          ? 'CLIP_SOURCE_MISSING'
          : 'CLIP_SOURCE_RANGE_UNSUPPORTED',
    )
    expect(cancelled).toHaveBeenCalledOnce()
    reader.dispose()
  })
  it('cancels oversized declared totals before opening/allocating the body', async () => {
    const cancelled = vi.fn()
    const body = new ReadableStream<Uint8Array>({ cancel: cancelled })
    const reader = createFiniteMediaSource(
      { kind: 'url', url: 'private-url' },
      limits,
      new AbortController().signal,
      {
        fetch: vi.fn(
          async () =>
            new Response(body, { status: 206, headers: { 'Content-Range': 'bytes 0-0/101' } }),
        ),
      },
    )
    await expect(reader.getSize()).rejects.toThrow('CLIP_SOURCE_MEMORY_LIMIT')
    expect(cancelled).toHaveBeenCalledOnce()
    reader.dispose()
  })
  it('rejects malformed ranges, changed sizes and overlong streamed bodies', async () => {
    for (const headers of [
      { 'Content-Range': 'bytes 0-1/80' } as Record<string, string>,
      { 'Content-Range': 'bytes 0-0/80', 'Content-Length': '9' },
    ]) {
      const cancelled = vi.fn(),
        body = new ReadableStream<Uint8Array>({ cancel: cancelled })
      const reader = createFiniteMediaSource(
        { kind: 'url', url: 'url' },
        limits,
        new AbortController().signal,
        { fetch: vi.fn(async () => new Response(body, { status: 206, headers })) },
      )
      await expect(reader.getSize()).rejects.toThrow('CLIP_SOURCE_RANGE_INVALID')
      expect(cancelled).toHaveBeenCalledOnce()
      reader.dispose()
    }
    const reader = createFiniteMediaSource(
      { kind: 'url', url: 'url' },
      limits,
      new AbortController().signal,
      {
        fetch: vi.fn(
          async () =>
            new Response(new Uint8Array(10), {
              status: 206,
              headers: { 'Content-Range': 'bytes 0-0/80' },
            }),
        ),
      },
    )
    await expect(reader.getSize()).rejects.toThrow('CLIP_SOURCE_RANGE_INVALID')
    reader.dispose()
  })
  it('slices local originals lazily and bounds concurrent total bytes before awaiting reads', async () => {
    const slices = vi.fn((start: number, end: number) => ({
      arrayBuffer: async () => new ArrayBuffer(end - start),
    }))
    const blob = { size: 80, slice: slices } as unknown as Blob
    const reader = createFiniteMediaSource(
      { kind: 'blob', blob },
      limits,
      new AbortController().signal,
    )
    expect(slices).not.toHaveBeenCalled()
    expect(await reader.getSize()).toBe(80)
    await Promise.all([reader.read(0, 16), reader.read(16, 32)])
    await expect(reader.read(32, 33)).rejects.toThrow('CLIP_SOURCE_MEMORY_LIMIT')
    await expect(reader.read(0, 17)).rejects.toThrow('CLIP_SOURCE_MEMORY_LIMIT')
    expect(slices).toHaveBeenCalledTimes(2)
    reader.dispose()
  })
  it('fences late local reads after cancellation and never retries expired access', async () => {
    let release!: (bytes: ArrayBuffer) => void
    const controller = new AbortController()
    const reader = createFiniteMediaSource(
      {
        kind: 'blob',
        blob: {
          size: 80,
          slice: () => ({
            arrayBuffer: () =>
              new Promise<ArrayBuffer>((resolve) => {
                release = resolve
              }),
          }),
        } as unknown as Blob,
      },
      limits,
      controller.signal,
    )
    const pending = reader.read(0, 8)
    await Promise.resolve()
    await Promise.resolve()
    controller.abort()
    release(new ArrayBuffer(8))
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    reader.dispose()
  })
})
