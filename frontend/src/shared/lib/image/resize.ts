export interface Dimensions {
  width: number
  height: number
}

/** The size that fits `maxEdge` while keeping the aspect ratio. Never upscales. */
export function fitWithin(width: number, height: number, maxEdge: number): Dimensions {
  const longEdge = Math.max(width, height)
  if (longEdge <= maxEdge) return { width, height }
  const scale = maxEdge / longEdge
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

export interface ResizedJpeg extends Dimensions {
  blob: Blob
}

/** Downscales a bitmap to `maxEdge` and encodes it as JPEG.
 *
 *  Re-encoding through a canvas is also what strips the metadata (GPS, device, original
 *  timestamp) a phone writes into every photo — the copy that leaves the device carries
 *  pixels only. The caller owns `bitmap` and closes it. */
export async function resizeToJpeg(
  bitmap: ImageBitmap,
  maxEdge: number,
  quality: number,
): Promise<ResizedJpeg> {
  const { width, height } = fitWithin(bitmap.width, bitmap.height, maxEdge)
  const canvas = createCanvas(width, height)
  const context = canvas.getContext('2d') as
    OffscreenCanvasRenderingContext2D | CanvasRenderingContext2D | null
  if (!context) throw new Error('2d canvas unavailable')
  // JPEG has no alpha; a transparent PNG would otherwise come out on black.
  context.fillStyle = '#fff' // style-escape: canvas flatten colour for JPEG export, not a UI colour
  context.fillRect(0, 0, width, height)
  context.drawImage(bitmap, 0, 0, width, height)
  const blob = await toBlob(canvas, 'image/jpeg', quality)
  return { blob, width, height }
}

/** Re-encodes a bitmap as PNG at its own dimensions.
 *
 *  No resize and no flatten, unlike `resizeToJpeg`: the caller's bitmap is a photo already
 *  bounded by `IMAGE_MAX_LONG_EDGE_PX` on the way in ([I6]), and PNG keeps its alpha, so there is
 *  no transparent-onto-black problem to paint around.
 *
 *  It exists because the system clipboard takes PNG and not JPEG — see `shared/lib/clipboard`.
 *  The caller owns `bitmap` and closes it. */
export async function encodePng(bitmap: ImageBitmap): Promise<Blob> {
  const canvas = createCanvas(bitmap.width, bitmap.height)
  const context = canvas.getContext('2d') as
    OffscreenCanvasRenderingContext2D | CanvasRenderingContext2D | null
  if (!context) throw new Error('2d canvas unavailable')
  context.drawImage(bitmap, 0, 0)
  return toBlob(canvas, 'image/png')
}

type AnyCanvas = OffscreenCanvas | HTMLCanvasElement

function createCanvas(width: number, height: number): AnyCanvas {
  if (typeof OffscreenCanvas !== 'undefined') return new OffscreenCanvas(width, height)
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  return canvas
}

function toBlob(
  canvas: AnyCanvas,
  type: 'image/jpeg' | 'image/png',
  quality?: number,
): Promise<Blob> {
  if ('convertToBlob' in canvas) return canvas.convertToBlob({ type, quality })
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error(`${type} encode failed`))),
      type,
      quality,
    )
  })
}
