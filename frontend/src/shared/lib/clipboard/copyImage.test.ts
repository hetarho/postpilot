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
  const createBitmap = vi.fn().mockResolvedValue({ width: 4, height: 3, close })
  vi.stubGlobal('createImageBitmap', createBitmap)
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
  // The stub AWAITS the item's promises, the way the real `clipboard.write` does: the payload is
  // handed over before the bytes exist, so a stub that resolved regardless would report a failed
  // read as a successful copy.
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
  return { items, write: spy, close, createBitmap }
}

/** The photo as it stands on the panel: an element that has already painted. What is copied is
 *  read out of THIS, never re-requested — that is the whole contract of the function. */
function painted(): HTMLImageElement {
  const element = document.createElement('img')
  element.src = 'https://bucket.test/IMG_1.jpg'
  return element
}

/** The browser's refusal to hand back pixels it considers unreadable by this origin. `DOMException`
 *  is used because that is what a real `createImageBitmap` rejects with; only the NAME is matched,
 *  so the encode path's own throw is recognised the same way. */
function securityError() {
  return new DOMException('tainted', 'SecurityError')
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('copyImage', () => {
  it('writes one ClipboardItem whose only flavor is image/png', async () => {
    const clipboard = stub()

    expect(await copyImage(painted())).toEqual({ kind: 'copied' })

    expect(clipboard.write).toHaveBeenCalledOnce()
    expect(clipboard.write.mock.calls[0][0]).toHaveLength(1)
    expect(clipboard.items).toHaveLength(1)
    expect(Object.keys(clipboard.items[0])).toEqual(['image/png'])
    // The value is a PROMISE, and `write` is called before it settles: awaiting the encode first
    // spends the user activation WebKit requires, which made every iOS copy fail as `refused`.
    expect(await clipboard.items[0]['image/png']).toBeInstanceOf(Blob)
    expect((await clipboard.items[0]['image/png']).type).toBe('image/png')
  })

  // The bug this function was rewritten for: a presigned view URL outlives nothing, the panel
  // outlives it, and a copy that re-downloaded the photo failed on a photo still on screen —
  // while the browser's own 이미지 복사 on the same photo worked. It reads the ELEMENT now, so
  // there is no request to fail, no URL to expire and no CORS answer to wait for.
  it('reads the pixels off the element and makes no request at all', async () => {
    const clipboard = stub()
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    const element = painted()

    expect(await copyImage(element)).toEqual({ kind: 'copied' })

    expect(fetchSpy).not.toHaveBeenCalled()
    expect(clipboard.createBitmap).toHaveBeenCalledWith(element)
  })

  it('closes the bitmap it decoded', async () => {
    const clipboard = stub()
    await copyImage(painted())
    expect(clipboard.close).toHaveBeenCalledOnce()
  })

  it('reports `unsupported` when the browser has no image clipboard', async () => {
    stub()
    vi.stubGlobal('ClipboardItem', undefined)
    expect(await copyImage(painted())).toEqual({ kind: 'unsupported' })

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
    expect(await copyImage(painted())).toEqual({ kind: 'unsupported' })
  })

  it('reports `refused` when the write itself is rejected', async () => {
    stub({ write: () => Promise.reject(new DOMException('blocked', 'NotAllowedError')) })
    expect(await copyImage(painted())).toEqual({ kind: 'refused' })
  })

  // The payload is handed over BEFORE the bytes exist. A failure inside that promise therefore
  // has to come back out through the write, and be told apart from a policy refusal.
  it('reports a read failure as such even though the write is what rejects', async () => {
    stub()
    vi.stubGlobal('createImageBitmap', vi.fn().mockRejectedValue(new Error('not loaded')))
    expect(await copyImage(painted())).toEqual({ kind: 'unreadable' })
  })

  // The two read failures lead to OPPOSITE advice — reload the post vs. do not bother, this origin
  // may not read these pixels — so a test that only covered "the read failed" is what let them
  // share one message. `SecurityError` is the browser's own word for the second, and both ends of
  // the read raise it, so both ends are pinned.
  it('reports `blocked` when the pixels are refused to this origin', async () => {
    stub()
    vi.stubGlobal('createImageBitmap', vi.fn().mockRejectedValue(securityError()))
    expect(await copyImage(painted())).toEqual({ kind: 'blocked' })

    // The same fact raised by the encode instead: a canvas tainted by an origin-unclean draw.
    stub()
    vi.stubGlobal(
      'OffscreenCanvas',
      class {
        getContext() {
          return {
            drawImage: () => {
              throw securityError()
            },
          }
        }
      },
    )
    expect(await copyImage(painted())).toEqual({ kind: 'blocked' })
  })

  it('reports `unreadable` for a failed decode and a failed encode', async () => {
    stub()
    vi.stubGlobal('createImageBitmap', vi.fn().mockRejectedValue(new Error('bad image')))
    expect(await copyImage(painted())).toEqual({ kind: 'unreadable' })

    stub()
    vi.stubGlobal(
      'OffscreenCanvas',
      class {
        getContext() {
          return null
        }
      },
    )
    expect(await copyImage(painted())).toEqual({ kind: 'unreadable' })
  })

  it('never writes a text flavor beside the image', async () => {
    const clipboard = stub()
    await copyImage(painted())
    for (const item of clipboard.items) {
      expect(Object.keys(item)).not.toContain('text/html')
      expect(Object.keys(item)).not.toContain('text/plain')
    }
  })
})
