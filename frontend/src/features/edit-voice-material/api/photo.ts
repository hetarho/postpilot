import { decodeImage, resizeToJpeg, type ResizedJpeg } from '@/shared/lib'
import {
  VOICE_MATERIAL_EDIT_PHOTO_JPEG_QUALITY,
  VOICE_MATERIAL_EDIT_PHOTO_MAX_EDGE,
} from '../config'
export async function prepareMaterialPhoto(file: File): Promise<ResizedJpeg> {
  const bitmap = await decodeImage(file)
  try {
    return await resizeToJpeg(
      bitmap,
      VOICE_MATERIAL_EDIT_PHOTO_MAX_EDGE,
      VOICE_MATERIAL_EDIT_PHOTO_JPEG_QUALITY,
    )
  } finally {
    bitmap.close()
  }
}
export async function putMaterialPhoto(
  url: string,
  contentType: string,
  blob: Blob,
  signal: AbortSignal,
): Promise<void> {
  const response = await fetch(url, {
    method: 'PUT',
    body: blob,
    headers: { 'Content-Type': contentType },
    signal,
  })
  if (!response.ok) throw new Error('The material photo upload failed')
}
