import { decodeImage, resizeToJpeg, type ResizedJpeg } from '@/shared/lib'
import { VOICE_PHOTO_JPEG_QUALITY, VOICE_PHOTO_MAX_LONG_EDGE_PX } from '../config'

/** Converts the owner's photo in the browser — the original never leaves the device (I6). */
export async function preparePhoto(file: File): Promise<ResizedJpeg> {
  const bitmap = await decodeImage(file)
  try {
    return await resizeToJpeg(bitmap, VOICE_PHOTO_MAX_LONG_EDGE_PX, VOICE_PHOTO_JPEG_QUALITY)
  } finally {
    bitmap.close()
  }
}

/** The direct PUT to private storage. The Content-Type is sent back exactly as the server gave
 *  it: it is part of the signature. */
export async function putPhoto(putUrl: string, contentType: string, blob: Blob): Promise<void> {
  const response = await fetch(putUrl, {
    method: 'PUT',
    body: blob,
    headers: { 'Content-Type': contentType },
  })
  if (!response.ok) throw new Error(`PUT failed: ${response.status}`)
}
