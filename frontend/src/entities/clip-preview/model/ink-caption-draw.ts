import type { BrowserInkLease } from './ink-cache'
import type { InkDocument } from './ink-raster'
import type { InkCaptionScene, InkCaptionSceneNode, InkCaptionPose } from './ink-caption-scene'
import { ClipInkError, inkRasterDimensions } from './ink-typography'

export interface BrowserCaptionSceneNode {
  readonly id: string
  readonly pose: InkCaptionPose
  readonly document?: InkDocument
  readonly ink?: BrowserInkLease
  readonly rect?: InkCaptionSceneNode['rect']
}
export interface BrowserCaptionPreparedScene {
  readonly opacity: number
  readonly bounds: InkCaptionScene['bounds']
  readonly nodes: readonly BrowserCaptionSceneNode[]
}

/** One bounded diagnostic canvas pair; glyph ink is always the cached resource.
 * GPU callers consume the same node matrices, rectangular masks and ink leases. */
export class BrowserCaptionSceneCanvas {
  private group?: OffscreenCanvas
  private tint?: OffscreenCanvas
  private destroyed = false
  draw(
    context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
    scene: BrowserCaptionPreparedScene,
  ) {
    if(this.destroyed) throw new ClipInkError('CLIP_INK_CANCELLED')
    const { bounds, nodes } = scene,
      {width,height} = inkRasterDimensions(bounds)
    this.group ??= new OffscreenCanvas(width, height)
    if (this.group.width !== width || this.group.height !== height) {
      this.group.width = width
      this.group.height = height
    }
    const group = this.group.getContext('2d')!
    group.setTransform(1, 0, 0, 1, 0, 0)
    group.globalCompositeOperation = 'source-over'
    group.clearRect(0, 0, width, height)
    group.translate(-bounds.x, -bounds.y)
    for (const node of nodes) {
      if (node.pose.opacity <= 0) continue
      group.save()
      const m = node.pose.matrix
      group.setTransform(m[0], m[1], m[2], m[3], m[4] - bounds.x, m[5] - bounds.y)
      group.globalAlpha = node.pose.opacity
      const clip = node.pose.clip
      if (clip) {
        group.beginPath()
        group.rect(clip.x, clip.y, clip.width, clip.height)
        group.clip()
      }
      if (node.ink && node.document) {
        const { bitmap, offset } = node.ink,
          document = node.document
        let source: CanvasImageSource = bitmap
        if (node.pose.tint) {
          this.tint ??= new OffscreenCanvas(bitmap.width, bitmap.height)
          if (this.tint.width !== bitmap.width || this.tint.height !== bitmap.height) {
            this.tint.width = bitmap.width
            this.tint.height = bitmap.height
          }
          const tint = this.tint.getContext('2d')!
          tint.globalCompositeOperation = 'source-over'
          tint.clearRect(0, 0, bitmap.width, bitmap.height)
          tint.drawImage(bitmap, 0, 0)
          tint.globalCompositeOperation = 'source-in'
          tint.fillStyle = node.pose.tint
          tint.fillRect(0, 0, bitmap.width, bitmap.height)
          source = this.tint
        }
        group.drawImage(
          source,
          document.bounds.x + offset.x,
          document.bounds.y + offset.y,
          document.bounds.width,
          document.bounds.height,
        )
      } else if (node.rect && node.pose.rect) {
        const r = node.pose.rect,
          s = node.rect.shadow
        if (s) {
          const rgb = [1, 3, 5].map((index) => parseInt(s.hex.slice(index, index + 2), 16))
          group.shadowColor = `rgba(${rgb.join(',')},${s.alpha})`
          group.shadowBlur = s.blur
          group.shadowOffsetX = s.dx
          group.shadowOffsetY = s.dy
        }
        group.fillStyle = node.rect.fill
        group.fillRect(r.x, r.y, r.width, r.height)
      }
      group.restore()
    }
    context.save()
    context.globalAlpha *= scene.opacity
    context.drawImage(this.group, bounds.x, bounds.y)
    context.restore()
  }
  measurements() {
    return {
      groupBytes: (this.group?.width ?? 0) * (this.group?.height ?? 0) * 4,
      tintBytes: (this.tint?.width ?? 0) * (this.tint?.height ?? 0) * 4,
    }
  }
  destroy() {
    this.destroyed = true
    if (this.group) {
      this.group.width = 0
      this.group.height = 0
    }
    if (this.tint) {
      this.tint.width = 0
      this.tint.height = 0
    }
    this.group = undefined
    this.tint = undefined
  }
}
