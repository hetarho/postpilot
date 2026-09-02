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

  // An expired presigned URL is the ordinary case here, and it must not reach the clipboard as an
  // empty image: reloading the post remints the URL, which is the recovery the panel names.
  it('reports `unreadable` for a failed fetch, a failed decode, and a failed encode', async () => {
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

  it('never writes a text flavor beside the image', async () => {
    const clipboard = stub()
    await copyImage('https://bucket.test/IMG_1.jpg')
    for (const item of clipboard.items) {
      expect(Object.keys(item)).not.toContain('text/html')
      expect(Object.keys(item)).not.toContain('text/plain')
    }
  })
})
