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

function windowed(
  total = 32,
  size = 80,
  onFetch?: (start: number, end: number) => void | Promise<void>,
) {
  const fetcher = vi.fn(async (_url: unknown, init?: RequestInit) => {
    const range = (init!.headers as Record<string, string>).Range.match(/bytes=(\d+)-(\d+)/u)!
    const start = Number(range[1]),
      end = Number(range[2]) + 1
    if (onFetch) await onFetch(start, end)
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
  it('reuses a boundary-spanning packet window for adjacent bytes without retaining returned slices', async () => {
    const { reader, fetcher } = windowed()
    const packet = await reader.read(6, 10)
    expect([...packet]).toEqual([6, 7, 8, 9])
    packet[0] = 99
    expect([...(await reader.read(10, 14))]).toEqual([10, 11, 12, 13])
    expect([...(await reader.read(6, 10))]).toEqual([6, 7, 8, 9])
    expect(fetcher).toHaveBeenCalledTimes(2)
    expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 8, largestReadBytes: 8 })
    reader.dispose()
  })
  it('reuses a cached packet prefix and the following aligned window within the original wire budget', async () => {
    const { reader, fetcher } = windowed(16)
    expect([...(await reader.read(2, 4))]).toEqual([2, 3])
    const packet = await reader.read(6, 10)
    expect([...packet]).toEqual([6, 7, 8, 9])
    expect(packet.buffer.byteLength).toBe(4)
    packet[2] = 99
    expect([...(await reader.read(8, 16))]).toEqual([8, 9, 10, 11, 12, 13, 14, 15])
    expect(fetcher).toHaveBeenCalledTimes(3)
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 16, largestReadBytes: 8 })
    reader.dispose()
  })
  it('shares the next aligned window between concurrent prefix-spanning and following packets', async () => {
    const { reader, fetcher } = windowed(16)
    await reader.read(2, 4)
    const values = await Promise.all([reader.read(6, 10), reader.read(10, 12), reader.read(12, 16)])
    expect(values.map((value) => [...value])).toEqual([
      [6, 7, 8, 9],
      [10, 11],
      [12, 13, 14, 15],
    ])
    expect(values.map((value) => value.buffer.byteLength)).toEqual([4, 2, 4])
    expect(fetcher).toHaveBeenCalledTimes(3)
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 16 })
    reader.dispose()
  })
  it('reuses a prefix at EOF without reserving those cached bytes again', async () => {
    const { reader, fetcher } = windowed(11, 19)
    await reader.read(9, 10)
    expect([...(await reader.read(14, 18))]).toEqual([14, 15, 16, 17])
    expect([...(await reader.read(18, 19))]).toEqual([18])
    expect(fetcher).toHaveBeenCalledTimes(3)
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 11, largestReadBytes: 8 })
    reader.dispose()
  })
  it('never returns a copied packet prefix when its pending remainder is aborted', async () => {
    let release!: () => void,
      started!: () => void,
      hold = false
    const pendingStarted = new Promise<void>((resolve) => {
      started = resolve
    })
    const pendingResponse = new Promise<void>((resolve) => {
      release = resolve
    })
    const { reader, fetcher, controller } = windowed(16, 80, async () => {
      if (!hold) return
      started()
      await pendingResponse
    })
    await reader.read(2, 4)
    hold = true
    const packet = reader.read(6, 10)
    await pendingStarted
    controller.abort()
    release()
    await expect(packet).rejects.toMatchObject({ name: 'AbortError' })
    await expect(reader.read(8, 9)).rejects.toMatchObject({ name: 'AbortError' })
    expect(fetcher).toHaveBeenCalledTimes(3)
    reader.dispose()
  })
  it('shares concurrent crossing packets and waits before fetching the next bounded window', async () => {
    let release!: () => void, started!: () => void
    const firstStarted = new Promise<void>((resolve) => {
      started = resolve
    })
    const firstResponse = new Promise<void>((resolve) => {
      release = resolve
    })
    let held = false
    const { reader, fetcher } = windowed(32, 80, async (start, end) => {
      if ((start === 0 && end === 1) || held) return
      held = true
      started()
      await firstResponse
    })
    const pending = Promise.all([
      reader.read(6, 10),
      reader.read(10, 14),
      reader.read(14, 18),
      reader.read(18, 22),
    ])
    await firstStarted
    expect.soft(fetcher).toHaveBeenCalledTimes(2)
    release()
    expect((await pending).map((value) => [...value])).toEqual([
      [6, 7, 8, 9],
      [10, 11, 12, 13],
      [14, 15, 16, 17],
      [18, 19, 20, 21],
    ])
    expect(fetcher).toHaveBeenCalledTimes(3)
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 16, largestReadBytes: 8 })
    reader.dispose()
  })
  it('clamps a crossing packet window at EOF and reuses its final adjacent byte', async () => {
    const { reader, fetcher } = windowed(32, 19)
    expect([...(await reader.read(14, 18))]).toEqual([14, 15, 16, 17])
    expect([...(await reader.read(18, 19))]).toEqual([18])
    expect(fetcher).toHaveBeenCalledTimes(2)
    expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 5, largestReadBytes: 5 })
    reader.dispose()
  })
  it('refuses a crossing read-ahead reservation before fetch when the total allowance is too small', async () => {
    const { reader, fetcher } = windowed(7)
    await expect(reader.read(6, 10)).rejects.toThrow('CLIP_SOURCE_MEMORY_LIMIT')
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(reader.measurements()).toMatchObject({ reads: 0, bytesRead: 0 })
    reader.dispose()
  })
  it('fences a late crossing window after abort and never publishes or fetches it again', async () => {
    let release!: () => void, started!: () => void
    const firstStarted = new Promise<void>((resolve) => {
      started = resolve
    })
    const firstResponse = new Promise<void>((resolve) => {
      release = resolve
    })
    const { reader, fetcher, controller } = windowed(32, 80, async (start, end) => {
      if (start === 0 && end === 1) return
      started()
      await firstResponse
    })
    const pending = reader.read(6, 10)
    await firstStarted
    controller.abort()
    release()
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    await expect(reader.read(10, 11)).rejects.toMatchObject({ name: 'AbortError' })
    expect(fetcher).toHaveBeenCalledTimes(2)
    expect(fetcher.mock.calls[1][1]?.signal?.aborted).toBe(true)
    reader.dispose()
  })
  it('clamps a final window to EOF and keeps larger-than-window reads direct', async () => {
    const { reader, fetcher } = windowed(32, 19)
    expect([...(await reader.read(18, 19))]).toEqual([18])
    expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 3 })
    expect([...(await reader.read(6, 16))]).toEqual([6, 7, 8, 9, 10, 11, 12, 13, 14, 15])
    expect(reader.measurements()).toMatchObject({ reads: 2, bytesRead: 13, largestReadBytes: 10 })
    expect((fetcher.mock.calls[2][1]!.headers as Record<string, string>).Range).toBe('bytes=6-15')
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
