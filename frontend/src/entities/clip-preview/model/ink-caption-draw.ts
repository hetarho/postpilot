import { Color } from 'pixi.js'
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
  readonly light?: InkCaptionSceneNode['light']
}
export interface BrowserCaptionPreparedScene {
  readonly style?: string
  readonly opacity: number
  readonly bounds: InkCaptionScene['bounds']
  readonly nodes: readonly BrowserCaptionSceneNode[]
}

/** One bounded diagnostic canvas pair; glyph ink is always the cached resource.
 * GPU callers consume the same node matrices, rectangular masks and ink leases. */
export class BrowserCaptionSceneCanvas {
  private group?: OffscreenCanvas
  private tint?: OffscreenCanvas
  private source?: OffscreenCanvas
  private effect?: OffscreenCanvas
  private blur?: OffscreenCanvas
  private destroyed = false
  private color = new Color()
  draw(
    context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
    scene: BrowserCaptionPreparedScene,
  ) {
    if (this.destroyed) throw new ClipInkError('CLIP_INK_CANCELLED')
    const { bounds, nodes } = scene,
      { width, height } = inkRasterDimensions(bounds)
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
      if (node.light || node.pose.effect) {
        if (!('filter' in group)) throw new ClipInkError('CLIP_INK_FILTER_UNSUPPORTED', scene.style)
        this.source ??= new OffscreenCanvas(width, height)
        this.effect ??= new OffscreenCanvas(width, height)
        this.blur ??= new OffscreenCanvas(width, height)
        const surfaces = [this.source, this.effect, this.blur]
        const contexts = surfaces.map((surface) => {
          if (surface.width !== width || surface.height !== height) {
            surface.width = width
            surface.height = height
          }
          const value = surface.getContext('2d')!
          value.setTransform(1, 0, 0, 1, 0, 0)
          value.globalAlpha = 1
          value.globalCompositeOperation = 'source-over'
          value.filter = 'none'
          value.clearRect(0, 0, width, height)
          value.translate(-bounds.x, -bounds.y)
          return value
        })
        const [source, effect, blur] = contexts as [
          OffscreenCanvasRenderingContext2D,
          OffscreenCanvasRenderingContext2D,
          OffscreenCanvasRenderingContext2D,
        ]
        if (node.ink && node.document)
          source.drawImage(
            node.ink.bitmap,
            node.document.bounds.x + node.ink.offset.x,
            node.document.bounds.y + node.ink.offset.y,
            node.document.bounds.width,
            node.document.bounds.height,
          )
        if (node.light && node.pose.light) {
          for (const [index, ellipse] of node.pose.light.ellipses.entries()) {
            source.save()
            source.translate(ellipse.cx, ellipse.cy)
            source.scale(ellipse.rx, ellipse.ry)
            const gradient = source.createRadialGradient(0, 0, 0, 0, 0, 1)
            for (const stop of node.light.palettes[index]!)
              gradient.addColorStop(
                stop.at,
                this.color.setValue(stop.hex).setAlpha(stop.alpha).toRgbaString(),
              )
            source.fillStyle = gradient
            source.beginPath()
            source.arc(0, 0, 1, 0, Math.PI * 2)
            source.fill()
            source.restore()
          }
          effect.filter = `blur(${node.light.sigma}px)`
          effect.drawImage(this.source, bounds.x, bounds.y)
        } else if (node.pose.effect?.kind === 'blur') {
          effect.filter = `blur(${node.pose.effect.sigma}px)`
          effect.drawImage(this.source, bounds.x, bounds.y)
        } else if (node.pose.effect?.kind === 'neon') {
          const e = node.pose.effect
          for (const [sigma, alpha, hex, repeats] of [
            [e.wideSigma, e.wideAlpha, e.wide, 2],
            [e.tightSigma, e.tightAlpha, e.tight, 1],
          ] as const) {
            blur.filter = 'none'
            blur.clearRect(bounds.x, bounds.y, width, height)
            blur.globalCompositeOperation = 'source-over'
            blur.drawImage(this.source, bounds.x, bounds.y)
            blur.globalCompositeOperation = 'source-in'
            blur.fillStyle = hex
            blur.fillRect(bounds.x, bounds.y, width, height)
            effect.filter = `blur(${sigma}px)`
            effect.globalAlpha = alpha
            for (let repeat = 0; repeat < repeats; repeat++)
              effect.drawImage(this.blur, bounds.x, bounds.y)
          }
          effect.filter = 'none'
          effect.globalAlpha = 1
          source.globalCompositeOperation = 'source-in'
          source.fillStyle = node.pose.tint!
          source.fillRect(bounds.x, bounds.y, width, height)
          effect.drawImage(this.source, bounds.x, bounds.y)
        } else if (node.pose.effect?.kind === 'gradient') {
          const e = node.pose.effect
          source.globalCompositeOperation = 'source-in'
          source.save()
          source.translate(e.box.x, e.box.y)
          source.scale(e.box.width, e.box.height)
          const gradient = source.createLinearGradient(0, 0.15, 1, 0.85)
          for (const stop of e.stops) gradient.addColorStop(stop.at, stop.hex)
          source.fillStyle = gradient
          source.fillRect(-1, -1, 3, 3)
          source.restore()
          effect.drawImage(this.source, bounds.x, bounds.y)
        }
        group.drawImage(this.effect, bounds.x, bounds.y)
      } else if (node.ink && node.document) {
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
          group.shadowColor = this.color.setValue(s.hex).setAlpha(s.alpha).toRgbaString()
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
      effectBytes: [this.source, this.effect, this.blur].reduce(
        (bytes, value) => bytes + (value?.width ?? 0) * (value?.height ?? 0) * 4,
        0,
      ),
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
    for (const surface of [this.source, this.effect, this.blur])
      if (surface) {
        surface.width = 0
        surface.height = 0
      }
    this.source = this.effect = this.blur = undefined
    this.group = undefined
    this.tint = undefined
  }
}
