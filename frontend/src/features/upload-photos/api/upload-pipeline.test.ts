import { afterEach, describe, expect, it, vi } from 'vitest'
import { decodeImage, resizeToJpeg } from '@/shared/lib'
import { readJpegCaptureTime } from '@/shared/lib/media'
import { createUploadPipeline } from './upload-pipeline'
import type { createUploadHandshake } from '@/entities/image'

vi.mock('@/shared/lib', () => ({ decodeImage: vi.fn(), resizeToJpeg: vi.fn() }))
vi.mock('@/shared/lib/media', () => ({ readJpegCaptureTime: vi.fn() }))
afterEach(() => vi.resetAllMocks())
describe('original capture time before JPEG conversion', () => {
  it.each(['2026-10-15T03:34:56.000Z', undefined])(
    'reads the original before decoding (%s)',
    async (capturedAt) => {
      const sequence: string[] = []
      const close = vi.fn()
      const resized = { blob: new Blob(['resized']), width: 10, height: 20 }
      vi.mocked(readJpegCaptureTime).mockImplementation(async () => {
        sequence.push('capture')
        return capturedAt
      })
      vi.mocked(decodeImage).mockImplementation(async () => {
        sequence.push('decode')
        return { close } as unknown as ImageBitmap
      })
      vi.mocked(resizeToJpeg).mockImplementation(async () => {
        sequence.push('resize')
        return resized
      })
      const pipeline = createUploadPipeline({} as ReturnType<typeof createUploadHandshake>)
      const original = new File(['original'], 'camera.jpg')
      expect(await pipeline.convert(original)).toEqual({
        ...resized,
        ...(capturedAt ? { capturedAt } : {}),
      })
      expect(readJpegCaptureTime).toHaveBeenCalledWith(original)
      expect(sequence).toEqual(['capture', 'decode', 'resize'])
      expect(close).toHaveBeenCalledOnce()
    },
  )
})
