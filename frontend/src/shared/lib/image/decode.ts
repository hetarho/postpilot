import { DecodeError } from './decode-error'
import { fileExtension } from './filename'

const HEIF_EXTENSIONS = new Set(['heic', 'heif'])
const HEIF_TYPES = new Set(['image/heic', 'image/heif', 'image/heic-sequence', 'image/heif-sequence'])

/** By extension or MIME type — a phone's file picker does not always fill in either. */
export function isHeif(file: { name: string; type: string }): boolean {
  return HEIF_EXTENSIONS.has(fileExtension(file.name)) || HEIF_TYPES.has(file.type)
}

/** Decodes a selected photo to a bitmap, upright.
 *
 *  JPEG/PNG/WebP go through the browser's own decoder. HEIC/HEIF go through the WASM
 *  decoder, loaded on first use only ([I6]; PRD §6.2) — nothing about it is fetched for
 *  a session that never selects one. Throws `DecodeError`. */
export async function decodeImage(file: File): Promise<ImageBitmap> {
  if (isHeif(file)) {
    const { decodeHeif } = await import('./heif')
    return decodeHeif(file)
  }
  try {
    // A phone photo carries its rotation as EXIF; without this the bitmap would come out
    // sideways and the resize would bake that in.
    return await createImageBitmap(file, { imageOrientation: 'from-image' })
  } catch (error) {
    // An engine that predates the `from-image` value rejects the option itself, not the
    // file, with a TypeError. Those engines already apply EXIF by default, so the plain
    // call gives the same upright bitmap.
    if (error instanceof TypeError) {
      try {
        return await createImageBitmap(file)
      } catch (fallbackError) {
        throw toDecodeError(fallbackError)
      }
    }
    throw toDecodeError(error)
  }
}

function toDecodeError(error: unknown): DecodeError {
  return new DecodeError('unreadable', error instanceof Error ? error.message : undefined)
}
