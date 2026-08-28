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
    | OffscreenCanvasRenderingContext2D
    | CanvasRenderingContext2D
    | null
  if (!context) throw new Error('2d canvas unavailable')
  // JPEG has no alpha; a transparent PNG would otherwise come out on black.
  context.fillStyle = '#fff' // style-escape: canvas flatten colour for JPEG export, not a UI colour
  context.fillRect(0, 0, width, height)
  context.drawImage(bitmap, 0, 0, width, height)
  const blob = await toBlob(canvas, quality)
  return { blob, width, height }
}

type AnyCanvas = OffscreenCanvas | HTMLCanvasElement

function createCanvas(width: number, height: number): AnyCanvas {
  if (typeof OffscreenCanvas !== 'undefined') return new OffscreenCanvas(width, height)
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  return canvas
}

function toBlob(canvas: AnyCanvas, quality: number): Promise<Blob> {
  if ('convertToBlob' in canvas) return canvas.convertToBlob({ type: 'image/jpeg', quality })
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('JPEG encode failed'))),
      'image/jpeg',
      quality,
    )
  })
}
