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

function windowed(total = 32, size = 80) {
  const fetcher = vi.fn(async (_url: unknown, init?: RequestInit) => {
    const range = (init!.headers as Record<string, string>).Range.match(/bytes=(\d+)-(\d+)/u)!
    const start = Number(range[1]),
      end = Number(range[2]) + 1
    return response(
      start,
      end,
      size,
      Uint8Array.from({ length: end - start }, (_, n) => start + n),
    )
  })
  const controller = new AbortController()
  const reader = createFiniteMediaSource(
    { kind: 'url', url: 'private-url' },
    { ...limits, maxReadTotalBytes: total, readAheadBytes: 8 },
    controller.signal,
    { fetch: fetcher },
  )
  return { reader, fetcher, controller }
}

describe('finite original access', () => {
  it('coalesces adjacent URL reads on demand, returns detached exact slices and accounts actual wire bytes', async () => {
    const { reader, fetcher } = windowed()
    expect(fetcher).not.toHaveBeenCalled()
    const first = await reader.read(9, 11)
    first[0] = 99
    expect([...(await reader.read(9, 11))]).toEqual([9, 10])
    expect([...(await reader.read(13, 16))]).toEqual([13, 14, 15])
    expect(fetcher).toHaveBeenCalledTimes(2) // one-byte bootstrap and one8-byte window
    expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 8, largestReadBytes: 8 })
    reader.dispose()
  })
  it('shares an in-flight window and serializes a different concurrent window', async () => {
    const { reader, fetcher } = windowed()
    const values = await Promise.all([
      reader.read(17, 19),
      reader.read(22, 24),
      reader.read(33, 35),
    ])
    expect(values.map((v) => [...v])).toEqual([
      [17, 18],
      [22, 23],
      [33, 34],
    ])
    expect(fetcher).toHaveBeenCalledTimes(3)
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 16 })
    reader.dispose()
  })
  it('clamps a final window to EOF and preserves direct reads spanning windows', async () => {
    const { reader } = windowed(32, 19)
    expect([...(await reader.read(18, 19))]).toEqual([18])
    expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 3 })
    expect([...(await reader.read(6, 10))]).toEqual([6, 7, 8, 9])
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 7 })
    reader.dispose()
  })
  it('refuses another window before fetching when actual wire bytes exhaust the total limit', async () => {
    const { reader, fetcher } = windowed(8)
    await reader.read(9, 10)
    await expect(reader.read(17, 18)).rejects.toThrow('CLIP_SOURCE_MEMORY_LIMIT')
    expect(fetcher).toHaveBeenCalledTimes(2)
    expect(reader.measurements().bytesRead).toBe(8)
    reader.dispose()
  })
  it('invalidates cached windows on cancellation and never fetches again', async () => {
    const { reader, fetcher, controller } = windowed()
    await reader.read(9, 10)
    controller.abort()
    await expect(reader.read(10, 11)).rejects.toThrow()
    expect(fetcher).toHaveBeenCalledTimes(2)
    reader.dispose()
  })
  it('retains exact206 validation for read-ahead and refuses a window outside the cache allowance', async () => {
    const fetcher = vi.fn(async (_url: unknown, init?: RequestInit) => {
      return (init!.headers as Record<string, string>).Range === 'bytes=0-0'
        ? response(0, 1)
        : response(8, 15)
    })
    const reader = createFiniteMediaSource(
      { kind: 'url', url: 'private-url' },
      { ...limits, readAheadBytes: 8 },
      new AbortController().signal,
      { fetch: fetcher },
    )
    await expect(reader.read(9, 10)).rejects.toThrow('CLIP_SOURCE_RANGE_INVALID')
    reader.dispose()
    expect(() =>
      createFiniteMediaSource(
        { kind: 'url', url: 'private-url' },
        { ...limits, readAheadBytes: 16 },
        new AbortController().signal,
      ),
    ).toThrow('CLIP_SOURCE_MEMORY_LIMIT')
  })
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
