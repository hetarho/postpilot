import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import { ClipInkError, type InkBox } from './ink-typography'

export const CLIP_BACKGROUND_VERSION = 'clip-browser-background-v1'
export type BackgroundRGB = readonly [number, number, number]
export interface BrowserGround {
  frames: number[]
  mean: number
  sigma: number
  rgb: [number, number, number]
}

export function backgroundLuminance(rgb: BackgroundRGB): number {
  const channel = (v: number) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
  if (rgb.some((v) => !Number.isFinite(v) || v < 0 || v > 1))
    throw new ClipInkError('CLIP_BACKGROUND_INVALID')
  return 0.2126 * channel(rgb[0]) + 0.7152 * channel(rgb[1]) + 0.0722 * channel(rgb[2])
}
/** Native three-frame RGB means first, then temporal luminance variance. */
export function summarizeBrowserGround(colors: readonly BackgroundRGB[]): BrowserGround {
  if (colors.length !== 3) throw new ClipInkError('CLIP_BACKGROUND_MISSING')
  const frames = colors.map(backgroundLuminance)
  const mean = frames.reduce((n, v) => n + v, 0) / 3
  const rgb: [number, number, number] = [0, 0, 0]
  for (const color of colors)
    for (let c = 0; c < 3; c++) rgb[c as 0 | 1 | 2] += color[c as 0 | 1 | 2] / 3
  return {
    frames,
    mean,
    sigma: Math.sqrt(frames.reduce((n, v) => n + (v - mean) ** 2, 0) / 3),
    rgb,
  }
}
/** Native output ceil seek; the last requested instant precedes window close. */
export function backgroundSampleFrames(
  startMs: number,
  endMs: number,
  totalFrames: number,
  fps: number,
): [number, number, number] {
  if (
    !Number.isSafeInteger(totalFrames) ||
    totalFrames < 1 ||
    !Number.isSafeInteger(fps) ||
    fps < 1 ||
    !Number.isFinite(startMs) ||
    !Number.isFinite(endMs) ||
    startMs < 0 ||
    endMs <= startMs
  )
    throw new ClipInkError('CLIP_BACKGROUND_INVALID')
  const duration = Math.floor((totalFrames * 1000) / fps)
  const last = Math.max(0, duration - Math.ceil(1000 / fps))
  const end = Math.min(endMs, last + 1)
  const start = Math.min(startMs, end - 1)
  const indexes = [start, Math.floor((start + end) / 2), Math.max(start, end - 1)].map((at) =>
    Math.min(totalFrames - 1, Math.ceil((at * fps) / 1000)),
  )
  return indexes as [number, number, number]
}
export function browserGroundDecisions(ground: BrowserGround) {
  return {
    scrim:
      ground.mean >= CLIP_DESIGN.luma.scrim_threshold ||
      ground.sigma >= CLIP_DESIGN.luma.sigma_threshold,
    accentWhite: ground.mean >= CLIP_DESIGN.luma.scrim_threshold,
  }
}
export function meanRegionPixels(
  data: Uint8ClampedArray,
  width: number,
  height: number,
): [number, number, number] {
  if (
    !Number.isSafeInteger(width) ||
    !Number.isSafeInteger(height) ||
    width < 1 ||
    height < 1 ||
    data.byteLength !== width * height * 4
  )
    throw new ClipInkError('CLIP_BACKGROUND_INVALID')
  const rgb: [number, number, number] = [0, 0, 0]
  let count = 0
  for (let y = 0; y < height; y += 2)
    for (let x = 0; x < width; x += 2) {
      const at = (y * width + x) * 4
      for (let c = 0; c < 3; c++) rgb[c as 0 | 1 | 2] += data[at + c]! / 255
      count++
    }
  return rgb.map((v) => v / count) as [number, number, number]
}
export function nativeFadeBlackPixel(
  outgoing: BackgroundRGB,
  incoming: BackgroundRGB,
  w0: number,
  w1: number,
): [number, number, number] {
  if (!Number.isFinite(w0) || !Number.isFinite(w1) || w0 < 0 || w1 < 0 || w0 + w1 > 1.0000001)
    throw new ClipInkError('CLIP_BACKGROUND_INVALID')
  const yuv = (rgb: BackgroundRGB) => {
    const [r, g, b] = rgb.map((v) => Math.round(v * 255))
    return [
      16 + 0.256788 * r! + 0.504129 * g! + 0.097906 * b!,
      128 - 0.148223 * r! - 0.290993 * g! + 0.439216 * b!,
      128 + 0.439216 * r! - 0.367788 * g! - 0.071427 * b!,
    ]
  }
  const a = yuv(outgoing),
    b = yuv(incoming),
    rest = 1 - w0 - w1
  const y = w0 * a[0]! + w1 * b[0]! - 16
  const cb = w0 * a[1]! + w1 * b[1]! + rest * 128 - 128
  const cr = w0 * a[2]! + w1 * b[2]! + rest * 128 - 128
  const clip = (v: number) => Math.min(255, Math.max(0, v)) / 255
  return [
    clip(1.164384 * y + 1.596027 * cr),
    clip(1.164384 * y - 0.391762 * cb - 0.812968 * cr),
    clip(1.164384 * y + 2.017232 * cb),
  ]
}
export function integerBackgroundRegion(box: InkBox, width: number, height: number): InkBox {
  if (
    [box.x, box.y, box.width, box.height, width, height].some((v) => !Number.isFinite(v)) ||
    box.width <= 0 ||
    box.height <= 0
  )
    throw new ClipInkError('CLIP_BACKGROUND_INVALID')
  const x = Math.max(0, Math.trunc(box.x)),
    y = Math.max(0, Math.trunc(box.y))
  const right = Math.min(width, Math.trunc(box.x + box.width)),
    bottom = Math.min(height, Math.trunc(box.y + box.height))
  if (right <= x || bottom <= y) throw new ClipInkError('CLIP_BACKGROUND_MISSING')
  return { x, y, width: right - x, height: bottom - y }
}

export function backgroundBoxUnion(boxes: readonly InkBox[]): InkBox {
  const valid = boxes.filter((b) => b.width > 0 && b.height > 0)
  if (!valid.length) throw new ClipInkError('CLIP_BACKGROUND_MISSING')
  const x = Math.min(...valid.map((b) => b.x)),
    y = Math.min(...valid.map((b) => b.y))
  return {
    x,
    y,
    width: Math.max(...valid.map((b) => b.x + b.width)) - x,
    height: Math.max(...valid.map((b) => b.y + b.height)) - y,
  }
}
