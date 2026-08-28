import { afterEach, describe, expect, it, vi } from 'vitest'
import { fitWithin, resizeToJpeg } from './resize'

describe('fitWithin', () => {
  it('scales a landscape photo down to the long edge', () => {
    expect(fitWithin(4032, 3024, 1024)).toEqual({ width: 1024, height: 768 })
  })

  it('scales a portrait photo down to the long edge', () => {
    expect(fitWithin(3024, 4032, 1024)).toEqual({ width: 768, height: 1024 })
  })

  it('never upscales', () => {
    expect(fitWithin(640, 480, 1024)).toEqual({ width: 640, height: 480 })
    expect(fitWithin(1024, 1024, 1024)).toEqual({ width: 1024, height: 1024 })
  })

  it('keeps an extreme aspect ratio at least one pixel wide', () => {
    expect(fitWithin(100_000, 10, 1024)).toEqual({ width: 1024, height: 1 })
  })
})

describe('resizeToJpeg', () => {
  const drawImage = vi.fn()
  const fillRect = vi.fn()
  const convertToBlob = vi.fn(async () => new Blob(['jpeg'], { type: 'image/jpeg' }))

  class FakeOffscreenCanvas {
    constructor(
      readonly width: number,
      readonly height: number,
    ) {}
    getContext() {
      return { drawImage, fillRect, fillStyle: '' }
    }
    convertToBlob = convertToBlob
  }

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('draws the bitmap at the fitted size and encodes a JPEG at the given quality', async () => {
    vi.stubGlobal('OffscreenCanvas', FakeOffscreenCanvas)
    const bitmap = { width: 4032, height: 3024, close: vi.fn() } as unknown as ImageBitmap

    const result = await resizeToJpeg(bitmap, 1024, 0.85)

    expect(result).toMatchObject({ width: 1024, height: 768 })
    expect(result.blob.type).toBe('image/jpeg')
    expect(drawImage).toHaveBeenCalledWith(bitmap, 0, 0, 1024, 768)
    expect(convertToBlob).toHaveBeenCalledWith({ type: 'image/jpeg', quality: 0.85 })
    // The caller owns the bitmap.
    expect(bitmap.close).not.toHaveBeenCalled()
  })

  it('paints a background first, so transparency does not become black', async () => {
    vi.stubGlobal('OffscreenCanvas', FakeOffscreenCanvas)
    const bitmap = { width: 200, height: 100, close: vi.fn() } as unknown as ImageBitmap

    await resizeToJpeg(bitmap, 1024, 0.85)

    expect(fillRect).toHaveBeenCalledWith(0, 0, 200, 100)
    expect(fillRect.mock.invocationCallOrder[0]).toBeLessThan(drawImage.mock.invocationCallOrder[0])
  })
})
