import type { Transport } from '@connectrpc/connect'
import { createUploadHandshake } from '@/entities/image'
import { IMAGE_JPEG_QUALITY, IMAGE_MAX_LONG_EDGE_PX } from '@/shared/config'
import { decodeImage, resizeToJpeg } from '@/shared/lib'
import { putBlobWithProgress } from '@/shared/lib/upload'
import type { UploadPipeline } from '../model/upload-batch'

/** The real pipeline: browser decode + resize, then the handshake around a direct PUT to
 *  object storage. The API is never in the path of the bytes ([I6]). */
export function createUploadPipeline(transport: Transport): UploadPipeline {
  const handshake = createUploadHandshake(transport)

  return {
    async convert(file) {
      const bitmap = await decodeImage(file)
      try {
        return await resizeToJpeg(bitmap, IMAGE_MAX_LONG_EDGE_PX, IMAGE_JPEG_QUALITY)
      } finally {
        bitmap.close()
      }
    },

    createUpload: handshake.createUpload,

    async put(putUrl, contentType, blob) {
      // The Content-Type is sent back exactly as given — it is part of the signature. No
      // credentials: this is the storage host, not the API, and the URL itself is the
      // authorization.
      const response = await fetch(putUrl, {
        method: 'PUT',
        body: blob,
        headers: { 'Content-Type': contentType },
      })
      if (!response.ok) throw new Error(`PUT failed: ${response.status}`)
    },

    putWithProgress,

    confirm: handshake.confirmUpload,
  }
}

/** The same PUT, reported as it goes.
 *
 *  `XMLHttpRequest` rather than `fetch` for one reason: fetch exposes no upload progress at
 *  all, and a 200 MB clip over mobile takes a minute — a card that only says 올리는 중 for that
 *  long reads as stuck (VIDEO-7). The photo PUT above is left on fetch: it moves ~200 KB, and
 *  a second transport for it would be two code paths for one thing.
 */
export function putWithProgress(
  putUrl: string,
  contentType: string,
  blob: Blob,
  onProgress: (percent: number) => void,
): Promise<void> {
  return putBlobWithProgress(putUrl, { 'Content-Type': contentType }, blob, onProgress)
}
