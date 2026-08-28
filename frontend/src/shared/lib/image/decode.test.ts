import { afterEach, describe, expect, it, vi } from 'vitest'
import { DecodeError } from './decode-error'
import { decodeImage, isHeif } from './decode'

// The WASM decoder module is what must stay out of the main path; here it is a spy so the
// test can prove which files reach it.
const decodeHeif = vi.fn<(file: Blob) => Promise<ImageBitmap>>(
  async () => 'heif-bitmap' as unknown as ImageBitmap,
)
vi.mock('./heif', () => ({ decodeHeif: (file: Blob) => decodeHeif(file) }))

afterEach(() => {
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('isHeif', () => {
  it('recognises HEIC/HEIF by extension, case-insensitively, or by MIME type', () => {
    expect(isHeif({ name: 'IMG_1.HEIC', type: '' })).toBe(true)
    expect(isHeif({ name: 'a.heif', type: '' })).toBe(true)
    expect(isHeif({ name: 'blob', type: 'image/heic' })).toBe(true)
    expect(isHeif({ name: 'a.jpg', type: 'image/jpeg' })).toBe(false)
  })
})

describe('decodeImage', () => {
  it('routes a HEIC to the lazily loaded decoder and nothing else to it', async () => {
    const createImageBitmap = vi.fn(async () => 'native-bitmap')
    vi.stubGlobal('createImageBitmap', createImageBitmap)
    const heic = new File(['x'], 'IMG_1.heic')
    const jpg = new File(['x'], 'IMG_2.jpg', { type: 'image/jpeg' })

    await expect(decodeImage(heic)).resolves.toBe('heif-bitmap')
    expect(decodeHeif).toHaveBeenCalledWith(heic)
    expect(createImageBitmap).not.toHaveBeenCalled()

    await expect(decodeImage(jpg)).resolves.toBe('native-bitmap')
    // Upright: a phone photo's rotation lives in EXIF.
    expect(createImageBitmap).toHaveBeenCalledWith(jpg, { imageOrientation: 'from-image' })
    expect(decodeHeif).toHaveBeenCalledTimes(1)
  })

  it('falls back to the plain call on an engine that rejects the orientation option', async () => {
    const createImageBitmap = vi.fn(async (_file: Blob, options?: object) => {
      if (options) throw new TypeError('unknown imageOrientation')
      return 'native-bitmap'
    })
    vi.stubGlobal('createImageBitmap', createImageBitmap)
    const jpg = new File(['x'], 'IMG_2.jpg')

    await expect(decodeImage(jpg)).resolves.toBe('native-bitmap')
    expect(createImageBitmap).toHaveBeenLastCalledWith(jpg)
  })

  it('reports a file the browser cannot decode as unreadable', async () => {
    vi.stubGlobal(
      'createImageBitmap',
      vi.fn(async () => {
        throw new DOMException('bad image', 'InvalidStateError')
      }),
    )

    const error = await decodeImage(new File(['x'], 'broken.png')).catch((caught) => caught)
    expect(error).toBeInstanceOf(DecodeError)
    expect((error as DecodeError).reason).toBe('unreadable')
  })
})
