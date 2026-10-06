import {
  Container,
  Sprite,
  Texture,
  ImageSource,
  Matrix,
  RenderTexture,
  Geometry,
  Mesh,
  Shader,
  GlProgram,
  UniformGroup,
  Rectangle,
  type BlurFilter,
  type WebGLRenderer,
} from 'pixi.js'
import type { BrowserCaptionPreparedScene, BrowserCaptionSceneNode } from './ink-caption-draw'
import {
  BrowserCaptionNeonPixi,
  captionGaussian,
  captionGaussianSigma,
} from './ink-caption-filter-pixi'
import { ClipInkError, inkRasterDimensions } from './ink-typography'
import { BrowserCaptionEmberPixi } from './ink-caption-ember-pixi'

const colorVector = (hex: string) =>
  new Float32Array([1, 3, 5].map((index) => parseInt(hex.slice(index, index + 2), 16) / 255))
const vertex = `#version 300 es
in vec2 aPosition;
uniform mat3 uProjectionMatrix;
uniform mat3 uWorldTransformMatrix;
uniform mat3 uTransformMatrix;
uniform vec4 uBox;
out vec2 vPoint;
out vec2 vUV;
void main(){vec2 p=uBox.xy+aPosition*uBox.zw;vPoint=p;vUV=aPosition;gl_Position=vec4((uProjectionMatrix*uWorldTransformMatrix*uTransformMatrix*vec3(p,1.)).xy,0.,1.);}`
const fragment = `#version 300 es
in vec2 vPoint;
uniform vec4 uRect;
uniform vec3 uFill;
uniform vec4 uShadow;
uniform vec3 uShadowColor;
uniform float uAlpha;
uniform vec4 uColor;
out vec4 finalColor;
float cdf(float x){float a=abs(x)*0.70710678118;float t=1./(1.+.3275911*a);float e=1.-(((((1.061405429*t-1.453152027)*t)+1.421413741)*t-.284496736)*t+.254829592)*t*exp(-a*a);return .5*(1.+sign(x)*e);}
void main(){
 vec2 lo=uRect.xy,hi=lo+uRect.zw,px=max(fwidth(vPoint),vec2(.0001));
 vec2 edge=clamp((min(hi,vPoint+px*.5)-max(lo,vPoint-px*.5))/px,0.,1.);
 float fa=edge.x*edge.y*uAlpha;
 float sa=0.;
 if(uShadow.z>0. && uRect.z>0.){vec2 q=vPoint-uShadow.xy;vec2 g=vec2(cdf((q.x-lo.x)/uShadow.z)-cdf((q.x-hi.x)/uShadow.z),cdf((q.y-lo.y)/uShadow.z)-cdf((q.y-hi.y)/uShadow.z));sa=max(0.,g.x*g.y)*uShadow.w*uAlpha;}
 finalColor=vec4(uFill*fa+uShadowColor*sa*(1.-fa),fa+sa*(1.-fa))*uColor;
}`
const gradientFragment = `#version 300 es
in vec2 vPoint;
in vec2 vUV;
uniform sampler2D uTexture;
uniform vec4 uGradientBox;
uniform vec4 uStops[6];
uniform float uOffsets[6];
uniform vec4 uColor;
out vec4 finalColor;
void main(){vec2 p=(vPoint-uGradientBox.xy)/uGradientBox.zw;float t=dot(p-vec2(0.,.15),vec2(1.,.7))/1.49;vec4 color=uStops[0];for(int i=1;i<6;i++){if(t>=uOffsets[i])color=uStops[i];else if(t>=uOffsets[i-1]){color=mix(uStops[i-1],uStops[i],clamp((t-uOffsets[i-1])/max(.000001,uOffsets[i]-uOffsets[i-1]),0.,1.));break;}}float a=texture(uTexture,vUV).a;finalColor=vec4(color.rgb*a,a)*uColor;}`
const lightFragment = `#version 300 es
in vec2 vPoint;
uniform vec4 uEllipseA;
uniform vec4 uEllipseB;
uniform vec4 uStopsA[3];
uniform vec4 uStopsB[3];
uniform float uMiddleA;
uniform float uMiddleB;
uniform vec4 uColor;
out vec4 finalColor;
vec4 light(vec4 ellipse,vec4 colors[3],float middle){float r=length((vPoint-ellipse.xy)/ellipse.zw);vec4 c=r<middle?mix(colors[0],colors[1],clamp(r/middle,0.,1.)):mix(colors[1],colors[2],clamp((r-middle)/(1.-middle),0.,1.));return vec4(c.rgb*c.a,c.a);}
void main(){vec4 a=light(uEllipseA,uStopsA,uMiddleA),b=light(uEllipseB,uStopsB,uMiddleB);finalColor=(b+a*(1.-b.a))*uColor;}`
interface Node {
  container: Container
  matrix: Matrix
  sprite?: Sprite
  mask?: Sprite
  mesh?: Mesh<Geometry, Shader>
  uniforms?: UniformGroup
  blur?: BlurFilter
  neon?: BrowserCaptionNeonPixi
  ember?: BrowserCaptionEmberPixi
}
interface ShapeUniforms {
  uBox: Float32Array
  uRect: Float32Array
  uFill: Float32Array
  uShadow: Float32Array
  uShadowColor: Float32Array
  uAlpha: number
}

/** PixiJS8 WebGL scene: stable textures/nodes, one bounded group surface,
 * and uniforms/rectangular masks driven only by frozen output time. */
export class BrowserCaptionScenePixi {
  private owner = {}
  private group = new Container()
  private presentation = new Container()
  private surface?: RenderTexture
  private quad?: Sprite
  private nodes = new Map<string, Node>()
  private geometry = new Geometry({
    attributes: {
      aPosition: { buffer: new Float32Array([0, 0, 1, 0, 1, 1, 0, 1]), format: 'float32x2' },
    },
    indexBuffer: new Uint32Array([0, 1, 2, 0, 2, 3]),
  })
  private program = new GlProgram({
    vertex,
    fragment,
    name: 'clip-caption-rectangle-shadow',
    preferredFragmentPrecision: 'highp',
  })
  private gradientProgram = new GlProgram({
    vertex,
    fragment: gradientFragment,
    name: 'clip-caption-gradient-srgb',
    preferredFragmentPrecision: 'highp',
  })
  private lightProgram = new GlProgram({
    vertex,
    fragment: lightFragment,
    name: 'clip-caption-radial-srgb',
    preferredFragmentPrecision: 'highp',
  })
  private view = new Matrix()
  private destroyed = false
  constructor(
    private renderer: WebGLRenderer,
    private releaseGPU: (owner: object) => void,
  ) {}
  private create(node: BrowserCaptionSceneNode): Node {
    const container = new Container(),
      result: Node = { container, matrix: new Matrix() }
    this.group.addChild(container)
    if (node.flames || node.sparks) {
      result.ember = new BrowserCaptionEmberPixi(node)
      container.addChild(result.ember.group)
    } else if (node.ink && node.pose.effect?.kind === 'neon' && node.document) {
      result.neon = new BrowserCaptionNeonPixi(this.renderer, node.document.bounds)
      container.addChild(result.neon.group)
    } else if (node.ink && node.pose.effect?.kind === 'gradient') {
      const uniforms = new UniformGroup({
        uBox: { value: new Float32Array(4), type: 'vec4<f32>' },
        uGradientBox: { value: new Float32Array(4), type: 'vec4<f32>' },
        uStops: { value: new Float32Array(24), type: 'vec4<f32>', size: 6 },
        uOffsets: { value: new Float32Array(6), type: 'f32', size: 6 },
      })
      result.uniforms = uniforms
      result.mesh = new Mesh({
        geometry: this.geometry,
        shader: new Shader({
          glProgram: this.gradientProgram,
          resources: { shape: uniforms, uTexture: Texture.WHITE.source },
        }),
      })
      container.addChild(result.mesh)
    } else if (node.light) {
      const colors = (stops: readonly { hex: string; alpha: number }[]) =>
        new Float32Array(stops.flatMap((stop) => [...colorVector(stop.hex), stop.alpha]))
      const uniforms = new UniformGroup({
        uBox: { value: new Float32Array(4), type: 'vec4<f32>' },
        uEllipseA: { value: new Float32Array(4), type: 'vec4<f32>' },
        uEllipseB: { value: new Float32Array(4), type: 'vec4<f32>' },
        uStopsA: { value: colors(node.light.palettes[0]!), type: 'vec4<f32>', size: 3 },
        uStopsB: { value: colors(node.light.palettes[1]!), type: 'vec4<f32>', size: 3 },
        uMiddleA: { value: node.light.palettes[0]![1]!.at, type: 'f32' },
        uMiddleB: { value: node.light.palettes[1]![1]!.at, type: 'f32' },
      })
      result.uniforms = uniforms
      result.mesh = new Mesh({
        geometry: this.geometry,
        shader: new Shader({ glProgram: this.lightProgram, resources: { shape: uniforms } }),
      })
      container.addChild(result.mesh)
      result.blur = captionGaussian(node.light.sigma)
      container.filters = [result.blur]
    } else if (node.ink) {
      result.sprite = new Sprite()
      container.addChild(result.sprite)
      if (node.pose.effect?.kind === 'blur') {
        result.blur = captionGaussian(node.pose.effect.sigma)
        container.filters = [result.blur]
      }
      if (node.pose.clip) {
        result.mask = new Sprite(Texture.WHITE)
        container.addChild(result.mask)
        result.sprite.mask = result.mask
      }
    } else if (node.rect) {
      const uniforms = new UniformGroup({
        uBox: { value: new Float32Array(4), type: 'vec4<f32>' },
        uRect: { value: new Float32Array(4), type: 'vec4<f32>' },
        uFill: { value: colorVector(node.rect.fill), type: 'vec3<f32>' },
        uShadow: { value: new Float32Array(4), type: 'vec4<f32>' },
        uShadowColor: {
          value: colorVector(node.rect.shadow?.hex ?? node.rect.fill),
          type: 'vec3<f32>',
        },
        uAlpha: { value: node.pose.opacity, type: 'f32' },
      })
      const shader = new Shader({ glProgram: this.program, resources: { shape: uniforms } })
      result.uniforms = uniforms
      result.mesh = new Mesh({ geometry: this.geometry, shader })
      container.addChild(result.mesh)
    }
    return result
  }
  render(scene: BrowserCaptionPreparedScene, target?: RenderTexture, clear = true) {
    try {
      if (
        this.renderer.context.webGLVersion !== 2 ||
        scene.nodes.some(
          (node) =>
            node.pose.effect && !['blur', 'neon', 'gradient'].includes(node.pose.effect.kind),
        )
      )
        throw new ClipInkError('CLIP_INK_FILTER_UNSUPPORTED', scene.style)
      this.renderScene(scene, target, clear)
      if (this.renderer.gl.getError() !== this.renderer.gl.NO_ERROR)
        throw new ClipInkError('CLIP_INK_FILTER_UNSUPPORTED', scene.style)
    } catch (error) {
      this.destroy()
      if (error instanceof ClipInkError) throw error
      throw new ClipInkError('CLIP_INK_FILTER_UNSUPPORTED', scene.style)
    }
  }
  private renderScene(scene: BrowserCaptionPreparedScene, target?: RenderTexture, clear = true) {
    if (this.destroyed) throw new ClipInkError('CLIP_INK_CANCELLED')
    if (this.renderer.gl.isContextLost()) {
      this.destroy()
      throw new ClipInkError('CLIP_INK_CONTEXT_LOST', scene.style)
    }
    const { bounds } = scene,
      { width, height } = inkRasterDimensions(bounds)
    if (!this.surface) {
      this.surface = RenderTexture.create({
        width,
        height,
        resolution: 1,
        antialias: true,
        dynamic: true,
      })
      this.quad = new Sprite(this.surface)
      this.presentation.addChild(this.quad)
    } else if (this.surface.width !== width || this.surface.height !== height)
      this.surface.resize(width, height)
    const active = new Set<string>(),
      ordered: Container[] = []
    for (const current of scene.nodes) {
      const key = current.id + '/' + (current.document?.key ?? (current.light ? 'light' : 'rect'))
      active.add(key)
      let node = this.nodes.get(key)
      if (!node) {
        node = this.create(current)
        this.nodes.set(key, node)
      }
      ordered.push(node.container)
      const m = current.pose.matrix
      node.matrix.set(...m)
      node.container.setFromMatrix(node.matrix)
      node.container.visible =
        current.pose.opacity > 0 && (!current.pose.clip || current.pose.clip.width > 0)
      if (node.ember) node.ember.update(current.pose)
      else if (current.ink && current.document) {
        const texture = current.ink.gpu(this.owner, () => {
          const value = new Texture({
            source: new ImageSource({
              resource: current.ink!.bitmap,
              alphaMode: 'premultiplied-alpha',
            }),
          })
          return {
            value,
            destroy: () => {
              for (const node of this.nodes.values())
                if (node.mesh?.shader?.resources.uTexture === value.source)
                  node.mesh.shader.resources.uTexture = Texture.WHITE.source
              value.destroy(true)
            },
          }
        })
        const document = current.document
        if (node.neon) {
          node.neon.update(texture, current.ink.offset, current.pose)
          node.neon.group.alpha = current.pose.opacity
        } else if (node.mesh && node.uniforms && current.pose.effect?.kind === 'gradient') {
          const e = current.pose.effect,
            u = node.uniforms.uniforms
          ;(u.uBox as Float32Array).set([
            document.bounds.x + current.ink.offset.x,
            document.bounds.y + current.ink.offset.y,
            document.bounds.width,
            document.bounds.height,
          ])
          ;(u.uGradientBox as Float32Array).set([e.box.x, e.box.y, e.box.width, e.box.height])
          ;(u.uStops as Float32Array).set(e.stops.flatMap((stop) => [...colorVector(stop.hex), 1]))
          ;(u.uOffsets as Float32Array).set(e.stops.map((stop) => stop.at))
          node.mesh.shader!.resources.uTexture = texture.source
          node.mesh.alpha = current.pose.opacity
          node.uniforms.update()
        } else if (node.sprite) {
          node.sprite.texture = texture
          node.sprite.width = document.bounds.width
          node.sprite.height = document.bounds.height
          node.sprite.position.set(
            document.bounds.x + current.ink.offset.x,
            document.bounds.y + current.ink.offset.y,
          )
          node.sprite.alpha = current.pose.opacity
          node.sprite.tint = current.pose.tint ?? 0xffffff
          if (node.blur && current.pose.effect?.kind === 'blur') {
            captionGaussianSigma(node.blur, current.pose.effect.sigma * current.pose.matrix[0])
            node.container.filterArea = new Rectangle(
              document.bounds.x,
              document.bounds.y,
              document.bounds.width,
              document.bounds.height,
            )
          }
          if (node.mask && current.pose.clip) {
            const clip = current.pose.clip
            node.mask.position.set(clip.x, clip.y)
            node.mask.width = clip.width
            node.mask.height = clip.height
          }
        }
      } else if (current.light && current.pose.light && node.uniforms && node.mesh) {
        const u = node.uniforms.uniforms
        ;(u.uBox as Float32Array).set([bounds.x, bounds.y, bounds.width, bounds.height])
        for (const [index, ellipse] of current.pose.light.ellipses.entries())
          (u[index ? 'uEllipseB' : 'uEllipseA'] as Float32Array).set([
            ellipse.cx,
            ellipse.cy,
            ellipse.rx,
            ellipse.ry,
          ])
        node.uniforms.update()
        node.mesh.alpha = current.pose.opacity
        node.container.filterArea = new Rectangle(bounds.x, bounds.y, bounds.width, bounds.height)
      } else if (current.rect && current.pose.rect && node.uniforms) {
        const rect = current.pose.rect,
          s = current.rect.shadow,
          extent = s ? s.blur * 1.5 + Math.max(Math.abs(s.dx), Math.abs(s.dy)) : 0
        const uniforms = node.uniforms.uniforms as unknown as ShapeUniforms
        uniforms.uBox.set([
          rect.x - extent,
          rect.y - extent,
          rect.width + extent * 2,
          rect.height + extent * 2,
        ])
        uniforms.uRect.set([rect.x, rect.y, rect.width, rect.height])
        uniforms.uFill.set(colorVector(current.rect.fill))
        uniforms.uShadow.set(s ? [s.dx, s.dy, s.blur / 2, s.alpha] : [0, 0, 0, 0])
        uniforms.uAlpha = current.pose.opacity
        node.uniforms.update()
      }
    }
    for (const [key, node] of this.nodes)
      if (!active.has(key)) {
        node.ember?.destroy()
        node.neon?.destroy()
        node.blur?.destroy()
        node.mesh?.shader?.destroy()
        node.container.destroy({ children: true })
        this.nodes.delete(key)
      }
    // Retained geometry survives text/phrase replacement; restore declared
    // paint order after removing old ink without rebuilding owned resources.
    ordered.forEach((container, index) => {
      if (this.group.children[index] !== container) this.group.setChildIndex(container, index)
    })
    this.view.set(1, 0, 0, 1, -bounds.x, -bounds.y)
    this.renderer.render({
      container: this.group,
      target: this.surface,
      transform: this.view,
      clear: true,
      clearColor: [0, 0, 0, 0],
    })
    this.quad!.position.set(bounds.x, bounds.y)
    this.quad!.alpha = scene.opacity
    this.renderer.render({ container: this.presentation, target, clear, clearColor: [0, 0, 0, 0] })
  }
  measurements() {
    const pooled = this.renderer.texture.managedTextures.filter((source) =>
      source?.label?.startsWith('texturePool_'),
    )
    return {
      nodes: this.nodes.size,
      dynamicShapes: [...this.nodes.values()].reduce(
        (sum, node) => sum + (node.ember?.measurements().shapes ?? 0),
        0,
      ),
      geometryBytes: [...this.nodes.values()].reduce(
        (sum, node) => sum + (node.ember?.measurements().geometryBytes ?? 0),
        0,
      ),
      shapeUniformBytes: [...this.nodes.values()].reduce(
        (sum, node) => sum + (node.ember?.measurements().uniformBytes ?? 0),
        0,
      ),
      managedTextureBytes: this.renderer.texture.managedTextures.reduce(
        (bytes, source) => bytes + (source?.pixelWidth ?? 0) * (source?.pixelHeight ?? 0) * 4,
        0,
      ),
      filterScratchBytes: pooled.length
        ? pooled.reduce(
            (bytes, source) => bytes + (source?.pixelWidth ?? 0) * (source?.pixelHeight ?? 0) * 4,
            0,
          )
        : undefined,
      effectBytes: [...this.nodes.values()].reduce(
        (bytes, node) => bytes + (node.neon?.measurements() ?? 0),
        0,
      ),
      filters: [...this.nodes.values()].reduce(
        (count, node) =>
          count +
          Number(Boolean(node.blur)) +
          Number(Boolean(node.neon)) +
          (node.ember?.measurements().filters ?? 0),
        0,
      ),
      groupBytes: (this.surface?.width ?? 0) * (this.surface?.height ?? 0) * 4,
    }
  }
  destroy() {
    if (this.destroyed) return
    this.destroyed = true
    for (const node of this.nodes.values()) {
      node.ember?.destroy()
      node.neon?.destroy()
      node.blur?.destroy()
      node.mesh?.shader?.destroy()
      node.container.destroy({ children: true })
    }
    this.nodes.clear()
    this.releaseGPU(this.owner)
    this.group.destroy()
    this.presentation.destroy({ children: true })
    this.surface?.destroy(true)
    this.surface = undefined
    this.quad = undefined
    this.geometry.destroy()
    this.program.destroy()
    this.gradientProgram.destroy()
    this.lightProgram.destroy()
  }
}
