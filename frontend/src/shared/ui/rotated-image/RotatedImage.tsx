import type { ImgHTMLAttributes, Ref } from 'react'
import { twMerge } from 'tailwind-merge'
import { quarterTurn } from '@/shared/lib'

export interface RotatedImageProps extends Omit<ImgHTMLAttributes<HTMLImageElement>, 'style'> {
  /** The photo's clockwise turn in degrees; anything but 90, 180 or 270 is none (POST-107). */
  rotation?: number
  /** `fill`: the image fills a box its parent shapes (a square tile or cell), so turning the
   *  element turns the picture in place. `natural`: the image keeps its own shape at the width
   *  of its column, so a quarter turn also swaps the box's sides. */
  fit: 'fill' | 'natural'
  width?: number
  height?: number
  imgRef?: Ref<HTMLImageElement>
  /** The box a `natural` quarter-turned image is drawn inside. */
  frameClassName?: string
  /** Caps a `natural` quarter-turned frame's height (a CSS length): the frame narrows to keep
   *  the turned shape within it, as `max-height` + `object-contain` does for an unturned one. */
  maxFrameHeight?: string
}

/** A photo shown turned by its rotation (POST-107). The stored bytes are never turned; every
 *  surface turns them here, and the Naver copy turns them the same way (`copyImage`). */
export function RotatedImage({
  rotation,
  fit,
  width,
  height,
  imgRef,
  className,
  frameClassName,
  maxFrameHeight,
  alt,
  ...img
}: RotatedImageProps) {
  const turn = quarterTurn(rotation)
  if (turn === 0) {
    return (
      <img ref={imgRef} alt={alt} width={width} height={height} className={className} {...img} />
    )
  }
  if (fit === 'fill' || turn === 180) {
    return (
      <img
        ref={imgRef}
        alt={alt}
        width={width}
        height={height}
        className={className}
        style={{ transform: `rotate(${turn}deg)` }}
        {...img}
      />
    )
  }
  // A quarter turn of a photo that keeps its own shape: the frame takes the turned shape, and the
  // image — as wide as the frame is tall — is turned about the frame's centre.
  const w = width && width > 0 ? width : 1
  const h = height && height > 0 ? height : 1
  return (
    <span
      className={twMerge('relative block w-full overflow-hidden', frameClassName)}
      style={{
        aspectRatio: `${h} / ${w}`,
        ...(maxFrameHeight ? { width: `min(100%, calc(${maxFrameHeight} * ${h / w}))` } : {}),
      }}
    >
      <img
        ref={imgRef}
        alt={alt}
        width={width}
        height={height}
        // The frame sets the size now, so the caller's own caps would only distort the turned picture.
        className={twMerge(className, 'absolute top-1/2 left-1/2 h-auto max-h-none max-w-none')}
        style={{
          width: `${(w / h) * 100}%`,
          transform: `translate(-50%, -50%) rotate(${turn}deg)`,
        }}
        {...img}
      />
    </span>
  )
}
