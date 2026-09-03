import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyImage } from './copyImage'

// jsdom has no `ClipboardItem`, no `createImageBitmap` and no canvas, so all three are stubbed.
// The assertion that matters is the PAYLOAD SHAPE — one item, one `image/png` flavor — because
// that is the only shape SmartEditor ONE accepts and the thing that must never regress.
function stub({ write }: { write?: () => Promise<void> } = {}) {
  const items: Array<Record<string, Promise<Blob>>> = []
  vi.stubGlobal(
    'ClipboardItem',
    class {
      constructor(readonly types: Record<string, Promise<Blob>>) {
        items.push(types)
      }
    },
  )
  const close = vi.fn()
  vi.stubGlobal('createImageBitmap', vi.fn().mockResolvedValue({ width: 4, height: 3, close }))
  vi.stubGlobal(
    'OffscreenCanvas',
    class {
      constructor(
        readonly width: number,
        readonly height: number,
      ) {}
      getContext() {
        return { drawImage: vi.fn() }
      }
      convertToBlob({ type }: { type: string }) {
        return Promise.resolve(new Blob([new Uint8Array([9])], { type }))
      }
    },
  )
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response(new Blob([new Uint8Array([1])], { type: 'image/jpeg' })),
  )
  // The stub AWAITS the item's promises, the way the real `clipboard.write` does: the payload is
  // handed over before the bytes exist, so a stub that resolved regardless would report a failed
  // decode as a successful copy.
  const spy = vi.fn(async (given: unknown[]) => {
    for (const item of given as Array<{ types: Record<string, Promise<Blob>> }>) {
      await Promise.all(Object.values(item.types))
    }
    return (write ?? (() => Promise.resolve()))()
  })
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { write: spy, writeText: vi.fn() },
  })
  return { items, write: spy, close }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

/** A view URL the way the API mints it: SigV4 states the lifetime in the query, which is what
 *  tells an expired read apart from a bucket that allows this origin no `GET`. */
function presigned(signedAt: Date, lifetimeSeconds: number) {
  const stamp = signedAt
    .toISOString()
    .replace(/[-:]/g, '')
    .replace(/\.\d{3}/, '')
  return `https://bucket.test/IMG_1.jpg?X-Amz-Date=${stamp}&X-Amz-Expires=${lifetimeSeconds}`
}

describe('copyImage', () => {
  it('writes one ClipboardItem whose only flavor is image/png', async () => {
    const clipboard = stub()

    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'copied' })

    expect(clipboard.write).toHaveBeenCalledOnce()
    expect(clipboard.write.mock.calls[0][0]).toHaveLength(1)
    expect(clipboard.items).toHaveLength(1)
    expect(Object.keys(clipboard.items[0])).toEqual(['image/png'])
    // The value is a PROMISE, and `write` is called before it settles: awaiting the fetch first
    // spends the user activation WebKit requires, which made every iOS copy fail as `refused`.
    expect(await clipboard.items[0]['image/png']).toBeInstanceOf(Blob)
    expect((await clipboard.items[0]['image/png']).type).toBe('image/png')
  })

  it('closes the bitmap it decoded', async () => {
    const clipboard = stub()
    await copyImage('https://bucket.test/IMG_1.jpg')
    expect(clipboard.close).toHaveBeenCalledOnce()
  })

  // The payload is handed over BEFORE the bytes exist. A failure inside that promise therefore
  // has to come back out through the write, and be told apart from a policy refusal.
  it('reports a load failure as unreadable even though the write is what rejects', async () => {
    stub()
    vi.stubGlobal('createImageBitmap', vi.fn().mockRejectedValue(new Error('bad image')))
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'unreadable' })
  })

  it('reports `unsupported` when the browser has no image clipboard', async () => {
    stub()
    vi.stubGlobal('ClipboardItem', undefined)
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'unsupported' })

    vi.stubGlobal(
      'ClipboardItem',
      class {
        constructor(readonly types: Record<string, Blob>) {}
      },
    )
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn() },
    })
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'unsupported' })
  })

  it('reports `refused` when the write itself is rejected', async () => {
    stub({ write: () => Promise.reject(new DOMException('blocked', 'NotAllowedError')) })
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'refused' })
  })

  // The two read failures lead to OPPOSITE advice — reload the post vs. do not bother, the bytes
  // are unreachable from this origin — so a test that only covered "the load failed" is what let
  // them share one message. Both arrive as a REJECTED fetch, so the URL's own lifetime is what
  // separates them, and these two cases pin that down from either side.
  it('reports `blocked` when the fetch rejects on a URL that had not expired', async () => {
    stub()
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))
    expect(await copyImage(presigned(new Date(), 600))).toEqual({ kind: 'blocked' })
    // A URL carrying no SigV4 lifetime cannot be claimed to have expired.
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'blocked' })
  })

  // R2 answers an expired read with a 403 carrying NO CORS headers, so the browser withholds it
  // and the fetch rejects — the same way it does for a missing `GET` allow. The two are therefore
  // separated by the URL's own lifetime, and this is the case that would otherwise be told to
  // "paste the text and add the photos yourself" when a reload is all it needed.
  it('reports `unreadable` when the fetch rejects on a URL that had already expired', async () => {
    stub()
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))
    expect(await copyImage(presigned(new Date(Date.now() - 3_600_000), 600))).toEqual({
      kind: 'unreadable',
    })
  })

  // Bytes that never became an image must not reach the clipboard as an empty one. A non-2xx the
  // browser DID expose lands here too — R2 hides its expired-URL 403, but a store that does not,
  // MinIO in local dev included, reaches this path instead of the rejection above.
  it('reports `unreadable` for a readable non-2xx, a failed decode, and a failed encode', async () => {
    stub()
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 403 }))
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'unreadable' })

    stub()
    vi.stubGlobal('createImageBitmap', vi.fn().mockRejectedValue(new Error('bad image')))
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'unreadable' })

    stub()
    vi.stubGlobal(
      'OffscreenCanvas',
      class {
        getContext() {
          return null
        }
      },
    )
    expect(await copyImage('https://bucket.test/IMG_1.jpg')).toEqual({ kind: 'unreadable' })
  })

  // `presigned()` above builds the shape this parser reads, so on its own it could agree with a
  // parser that is wrong about the real thing. This URL is the shape aws-sdk-go-v2's
  // `PresignGetObject` actually emits (`backend/internal/storage/r2.go`), pinned as a literal.
  it('reads the lifetime out of a real presigned view URL', async () => {
    const real =
      'https://acct.r2.cloudflarestorage.com/postpilot-prod/u/1/IMG_1.jpg' +
      '?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=key%2F20260904%2Fauto%2Fs3%2Faws4_request' +
      '&X-Amz-Date=20260904T010203Z&X-Amz-Expires=600&X-Amz-SignedHeaders=host&X-Amz-Signature=abc'
    stub()
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date('2026-09-04T02:00:00Z'))
      expect(await copyImage(real)).toEqual({ kind: 'unreadable' })
      vi.setSystemTime(new Date('2026-09-04T01:05:00Z'))
      expect(await copyImage(real)).toEqual({ kind: 'blocked' })
    } finally {
      vi.useRealTimers()
    }
  })

  it('never writes a text flavor beside the image', async () => {
    const clipboard = stub()
    await copyImage('https://bucket.test/IMG_1.jpg')
    for (const item of clipboard.items) {
      expect(Object.keys(item)).not.toContain('text/html')
      expect(Object.keys(item)).not.toContain('text/plain')
    }
  })
})
