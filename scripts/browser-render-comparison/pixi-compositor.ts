import { Container, Graphics, FillGradient, ImageSource, Matrix, RenderTexture, Sprite, Texture, type WebGLRenderer, type GradientOptions } from 'pixi.js'

type Gradient = { options: GradientOptions; stops: { offset: number; color: string }[] }
type Paint = string | Gradient
type State = { matrix: Matrix; alpha: number; paint: Paint; operation: string }

/** Diagnostic paint port for the shared frame evaluator and measured component
 * builders. No Canvas2D surface participates in whole-frame composition.
 * Uploads/render submissions finish before the shared decoder closes its lease.
 */
export class PixiFrameCompositor {
  readonly target: RenderTexture
  private presentation: Sprite
  private state: State = { matrix: new Matrix(), alpha: 1, paint: '#000000', operation: 'source-over' }
  private stack: State[] = []
  private liveTextureBytes = 0
  private peakTextureBytes = 0
  private peakManagedBytes = 0
  private draws = 0
  constructor(private renderer: WebGLRenderer, private width: number, private height: number) {
    if (renderer.context.webGLVersion !== 2) throw Error('BENCHMARK_WEBGL2_UNAVAILABLE')
    this.target = RenderTexture.create({ width, height, resolution: 1, antialias: false })
    this.presentation = new Sprite(this.target)
  }
  get globalAlpha() { return this.state.alpha }
  set globalAlpha(value: number) { this.state.alpha = value }
  get fillStyle() { return this.state.paint }
  set fillStyle(value: Paint) { this.state.paint = value }
  get globalCompositeOperation() { return this.state.operation }
  set globalCompositeOperation(value: string) {
    if (!['source-over', 'copy'].includes(value)) throw Error('BENCHMARK_GPU_BLEND_UNSUPPORTED')
    this.state.operation = value
  }
  save() { this.stack.push({ ...this.state, matrix: this.state.matrix.clone() }) }
  restore() {
    const state = this.stack.pop()
    if (!state) throw Error('BENCHMARK_GPU_PAINT_STACK')
    this.state = state
  }
  translate(x: number, y: number) { this.state.matrix.append(new Matrix(1, 0, 0, 1, x, y)) }
  scale(x: number, y: number) { this.state.matrix.append(new Matrix(x, 0, 0, y, 0, 0)) }
  rotate(angle: number) {
    this.state.matrix.append(new Matrix(Math.cos(angle), Math.sin(angle), -Math.sin(angle), Math.cos(angle), 0, 0))
  }
  private gradient(options: Gradient['options']) {
    const value: Gradient = { options, stops: [] }
    return Object.assign(value, { addColorStop(offset: number, color: string) { value.stops.push({ offset, color }) } })
  }
  createLinearGradient(x0: number, y0: number, x1: number, y1: number) {
    return this.gradient({ type: 'linear', start: { x: x0, y: y0 }, end: { x: x1, y: y1 }, textureSpace: 'global' })
  }
  createRadialGradient(x0: number, y0: number, r0: number, x1: number, y1: number, r1: number) {
    return this.gradient({ type: 'radial', center: { x: x0, y: y0 }, innerRadius: r0, outerCenter: { x: x1, y: y1 }, outerRadius: r1, textureSpace: 'global' })
  }
  private submit(child: Container, clear = this.state.operation === 'copy') {
    const stage = new Container()
    stage.setFromMatrix(this.state.matrix)
    stage.alpha = this.state.alpha
    stage.addChild(child)
    try {
      this.renderer.render({ container: stage, target: this.target, clear, clearColor: [0, 0, 0, 1] })
      this.draws++
      this.peakManagedBytes = Math.max(this.peakManagedBytes, this.renderer.texture.managedTextures.reduce((n, source) => n + (source?.pixelWidth ?? 0) * (source?.pixelHeight ?? 0) * 4, 0))
    } finally { stage.destroy({ children: true }) }
  }
  fillRect(x: number, y: number, width: number, height: number) {
    const paint = this.state.paint
    const gradient = typeof paint === 'string' ? undefined : new FillGradient({ ...paint.options, colorStops: paint.stops })
    const shape = new Graphics().rect(x, y, width, height).fill(gradient ?? paint as string)
    try { this.submit(shape) } finally { gradient?.destroy() }
  }
  drawImage(image: VideoFrame | ImageBitmap | OffscreenCanvas, x: number, y: number, width?: number, height?: number) {
    const sourceWidth = image instanceof VideoFrame ? image.displayWidth : image.width
    const sourceHeight = image instanceof VideoFrame ? image.displayHeight : image.height
    const texture = new Texture({ source: new ImageSource({ resource: image, width: sourceWidth, height: sourceHeight }) })
    const sprite = new Sprite(texture)
    sprite.position.set(x, y)
    sprite.width = width ?? sourceWidth
    sprite.height = height ?? sourceHeight
    const bytes = sourceWidth * sourceHeight * 4
    this.liveTextureBytes += bytes
    this.peakTextureBytes = Math.max(this.peakTextureBytes, this.liveTextureBytes)
    try { this.submit(sprite) }
    finally { texture.destroy(true); this.liveTextureBytes -= bytes }
  }
  context() { return this as unknown as OffscreenCanvasRenderingContext2D }
  present() { this.renderer.render({ container: this.presentation, clear: true, clearColor: [0, 0, 0, 1] }) }
  readback() {
    this.present()
    const gl = this.renderer.gl, stride = this.width * 4
    const bottomUp = new Uint8Array(stride * this.height), topDown = new Uint8ClampedArray(bottomUp.length)
    gl.readPixels(0, 0, this.width, this.height, gl.RGBA, gl.UNSIGNED_BYTE, bottomUp)
    if (gl.getError() !== gl.NO_ERROR) throw Error('BENCHMARK_GPU_READBACK_FAILED')
    for (let row = 0; row < this.height; row++) topDown.set(bottomUp.subarray((this.height - 1 - row) * stride, (this.height - row) * stride), row * stride)
    if (this.liveTextureBytes || this.stack.length) throw Error('BENCHMARK_GPU_FRAME_LEAK')
    return new ImageData(topDown, this.width, this.height)
  }
  measurements() {
    return { frameTargetBytes: this.width * this.height * 4, liveTextureBytes: this.liveTextureBytes,
      peakTransientTextureBytes: this.peakTextureBytes, peakManagedTextureBytes: this.peakManagedBytes, draws: this.draws }
  }
  destroy() {
    this.presentation.destroy()
    this.target.destroy(true)
    if (this.liveTextureBytes || this.stack.length) throw Error('BENCHMARK_GPU_CLEANUP_LEAK')
  }
}

export function pixiRendererIdentity(renderer: WebGLRenderer) {
  const gl = renderer.gl, debug = gl.getExtension('WEBGL_debug_renderer_info')
  const raw = debug ? gl.getParameter(debug.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER)
  return { renderer: String(raw), unmasked: !!debug && typeof raw === 'string', webGLVersion: renderer.context.webGLVersion }
}
