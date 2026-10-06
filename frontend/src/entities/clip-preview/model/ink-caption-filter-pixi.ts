import {
  BlurFilter,
  Container,
  Matrix,
  Rectangle,
  RenderTexture,
  Sprite,
  type Texture,
  type WebGLRenderer,
} from 'pixi.js'
import type { InkBox } from './ink-typography'
import type { InkCaptionPose } from './ink-caption-scene'

// Exact variance of PixiJS 8.22.0's symmetric 15-tap kernel. Its optimized
// quality passes normalize their squared step sizes; strength is not sigma.
const kernel = [0.000489, 0.002403, 0.009246, 0.02784, 0.065602, 0.120999, 0.174697, 0.197448]
const deviation = Math.sqrt(
  kernel.slice(0, 7).reduce((sum, weight, index) => sum + 2 * weight * (7 - index) ** 2, 0),
)
/** Linear texture sampling adds fractional-pixel variance to every tap. Solve
 * that exact discrete variance as well: a continuous-only sigma conversion
 * noticeably softens rapid blur-in when native sigma is below one pixel. */
export function captionGaussianStrength(sigma: number) {
  const sum = kernel[7]! + 2 * kernel.slice(0, 7).reduce((value, weight) => value + weight, 0)
  const normalizer = Math.sqrt(1 + 0.25 + 0.0625 + 0.015625)
  const variance = (strength: number) =>
    [1, 0.5, 0.25, 0.125].reduce((total, coefficient) => {
      const step = (strength * coefficient) / normalizer
      return (
        total +
        kernel.slice(0, 7).reduce((value, weight, index) => {
          const offset = (7 - index) * step,
            whole = Math.floor(offset),
            fraction = offset - whole
          return (
            value +
            (2 * weight * (whole * whole * (1 - fraction) + (whole + 1) ** 2 * fraction)) / sum
          )
        }, 0)
      )
    }, 0)
  let low = 0,
    high = sigma / deviation
  for (let i = 0; i < 22; i++) {
    const middle = (low + high) / 2
    if (variance(middle) > sigma * sigma) high = middle
    else low = middle
  }
  return (low + high) / 2
}
export function captionGaussian(sigma: number) {
  const filter = new BlurFilter({
    strength: captionGaussianStrength(sigma),
    quality: 4,
    kernelSize: 15,
    resolution: 1,
  })
  filter.padding = 0
  return filter
}
export function captionGaussianSigma(filter: BlurFilter, sigma: number) {
  filter.strength = captionGaussianStrength(sigma)
  filter.padding = 0
}

/** An owned pair of cropped glow targets. Wide glow is rendered once then
 * merged twice, followed by tight glow and core, exactly as native feMerge. */
export class BrowserCaptionNeonPixi {
  readonly group = new Container()
  private input = new Container()
  private source = new Sprite()
  private wide: RenderTexture
  private tight: RenderTexture
  private output: Sprite[]
  private blur = captionGaussian(26)
  private view = new Matrix()
  private destroyed = false
  constructor(
    private renderer: WebGLRenderer,
    private bounds: InkBox,
  ) {
    this.wide = RenderTexture.create({
      width: Math.ceil(bounds.width),
      height: Math.ceil(bounds.height),
    })
    this.tight = RenderTexture.create({
      width: Math.ceil(bounds.width),
      height: Math.ceil(bounds.height),
    })
    this.input.addChild(this.source)
    this.input.filters = [this.blur]
    this.input.filterArea = new Rectangle(bounds.x, bounds.y, bounds.width, bounds.height)
    this.output = [
      new Sprite(this.wide),
      new Sprite(this.wide),
      new Sprite(this.tight),
      new Sprite(),
    ]
    this.group.addChild(...this.output)
    this.view.set(1, 0, 0, 1, -bounds.x, -bounds.y)
    for (const sprite of this.output.slice(0, 3)) sprite.position.set(bounds.x, bounds.y)
  }
  update(texture: Texture, offset: { x: number; y: number }, pose: InkCaptionPose) {
    const e = pose.effect
    if (e?.kind !== 'neon' || this.destroyed) return
    this.source.texture = texture
    this.source.position.set(this.bounds.x + offset.x, this.bounds.y + offset.y)
    this.source.width = this.bounds.width
    this.source.height = this.bounds.height
    for (const [sigma, target] of [
      [e.wideSigma, this.wide],
      [e.tightSigma, this.tight],
    ] as const) {
      captionGaussianSigma(this.blur, sigma)
      this.renderer.render({
        container: this.input,
        target,
        transform: this.view,
        clear: true,
        clearColor: [0, 0, 0, 0],
      })
    }
    for (const sprite of this.output.slice(0, 2)) {
      sprite.tint = e.wide
      sprite.alpha = e.wideAlpha
    }
    this.output[2]!.tint = e.tight
    this.output[2]!.alpha = e.tightAlpha
    const core = this.output[3]!
    core.texture = texture
    core.position.copyFrom(this.source.position)
    core.width = this.bounds.width
    core.height = this.bounds.height
    core.tint = pose.tint ?? 0xffffff
  }
  measurements() {
    return this.destroyed
      ? 0
      : (this.wide.width * this.wide.height + this.tight.width * this.tight.height) * 4
  }
  destroy() {
    if (this.destroyed) return
    this.destroyed = true
    this.group.destroy({ children: true })
    this.input.destroy({ children: true })
    this.wide.destroy(true)
    this.tight.destroy(true)
    this.blur.destroy()
  }
}
