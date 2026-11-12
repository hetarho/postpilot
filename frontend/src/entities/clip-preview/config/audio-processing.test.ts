import { describe, expect, it, vi } from 'vitest'
import { createFiniteMediaSource } from '@/shared/lib/media'
import { CLIP_AUDIO_PROCESSING } from './audio-processing'

const original = Uint8Array.from(
  { length: 197_309 },
  (_, i) => (i * 31 + Math.floor(i / 251)) % 256,
)

function audioSource(options: { wireCap?: number; hold?: () => Promise<void> } = {}) {
  const controller = new AbortController()
  const fetcher = vi.fn(async (_url: unknown, init?: RequestInit) => {
    const match = (init!.headers as Record<string, string>).Range.match(/^bytes=(\d+)-(\d+)$/u)!
    const start = Number(match[1]),
      end = Number(match[2]) + 1
    if (end - start > 1) await options.hold?.()
    return new Response(original.slice(start, end), {
      status: 206,
      headers: {
        'Content-Range': `bytes ${start}-${end - 1}/${original.length}`,
        'Content-Length': String(end - start),
      },
    })
  })
  const reader = createFiniteMediaSource(
    { kind: 'url', url: 'https://example.com/retained-original' },
    {
      ...CLIP_AUDIO_PROCESSING,
      ...(options.wireCap === undefined ? {} : { maxReadTotalBytes: options.wireCap }),
    },
    controller.signal,
    { fetch: fetcher },
  )
  return { reader, fetcher, controller }
}

describe('configured retained-source audio access', () => {
  it('batches clustered AAC packets across video gaps, including a nonzero final window', async () => {
    const { reader, fetcher } = audioSource()
    const packets: [number, number][] = [
      [66_100, 66_310],
      [66_310, 66_750],
      [67_100, 67_360],
      [140_100, 140_370],
      [140_370, 140_800],
      [141_000, 141_240],
      [196_700, 196_980],
      [196_980, 197_180],
      [197_180, original.length],
    ]
    try {
      expect(fetcher).not.toHaveBeenCalled()
      for (const [start, end] of packets) {
        const bytes = await reader.read(start, end)
        expect(bytes).toEqual(original.slice(start, end))
        expect(bytes.buffer.byteLength).toBe(end - start)
        bytes.fill(0)
      }
      expect(await reader.read(197_180, original.length)).toEqual(original.slice(197_180))
      expect(
        fetcher.mock.calls.map(([, init]) => (init!.headers as Record<string, string>).Range),
      ).toEqual(['bytes=0-0', 'bytes=65536-98303', 'bytes=131072-163839', 'bytes=196608-197308'])
      expect(reader.measurements()).toEqual({
        reads: 3,
        bytesRead: 66_237,
        largestReadBytes: 32_768,
        sizeBytes: original.length,
      })
    } finally {
      reader.dispose()
    }
  })

  it('reserves the entire pending window and refuses the next before issuing an over-budget GET', async () => {
    let release!: () => void, started!: () => void
    const pending = new Promise<void>((resolve) => {
      release = resolve
    })
    const requested = new Promise<void>((resolve) => {
      started = resolve
    })
    // Lower only this fixture's wire cap to expose admission with two small packets.
    const { reader, fetcher } = audioSource({
      wireCap: 65_535,
      hold: () => {
        started()
        return pending
      },
    })
    try {
      const first = reader.read(66_100, 66_310)
      await requested
      const reservedBeforeResponse = reader.measurements()
      const refused = expect(reader.read(140_100, 140_370)).rejects.toMatchObject({
        code: 'CLIP_SOURCE_MEMORY_LIMIT',
      })
      expect(fetcher).toHaveBeenCalledTimes(2)
      release()
      expect(await first).toEqual(original.slice(66_100, 66_310))
      await refused
      expect(reservedBeforeResponse).toMatchObject({ reads: 1, bytesRead: 32_768 })
      expect(fetcher).toHaveBeenCalledTimes(2)
      expect(reader.measurements()).toMatchObject({ reads: 1, bytesRead: 32_768 })
    } finally {
      release()
      reader.dispose()
    }
  })

  it('aborts a pending audio window without publishing late packet bytes', async () => {
    let release!: () => void, started!: () => void
    const pending = new Promise<void>((resolve) => {
      release = resolve
    })
    const requested = new Promise<void>((resolve) => {
      started = resolve
    })
    const { reader, fetcher, controller } = audioSource({
      hold: () => {
        started()
        return pending
      },
    })
    try {
      const packet = reader.read(66_100, 66_310)
      await requested
      const cancelled = expect(packet).rejects.toMatchObject({ name: 'AbortError' })
      controller.abort()
      expect((fetcher.mock.calls[1]![1]!.signal as AbortSignal).aborted).toBe(true)
      release()
      await cancelled
      await expect(reader.read(66_310, 66_750)).rejects.toMatchObject({ name: 'AbortError' })
      expect(fetcher).toHaveBeenCalledTimes(2)
    } finally {
      release()
      reader.dispose()
    }
  })
})
