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
  type WebGLRenderer,
} from 'pixi.js'
import type { BrowserCaptionPreparedScene, BrowserCaptionSceneNode } from './ink-caption-draw'
import { ClipInkError, inkRasterDimensions } from './ink-typography'

const rgb = (hex: string) =>
  new Float32Array([1, 3, 5].map((index) => parseInt(hex.slice(index, index + 2), 16) / 255))
const vertex = `#version 300 es
in vec2 aPosition;
uniform mat3 uProjectionMatrix;
uniform mat3 uWorldTransformMatrix;
uniform mat3 uTransformMatrix;
uniform vec4 uBox;
out vec2 vPoint;
void main(){vec2 p=uBox.xy+aPosition*uBox.zw;vPoint=p;gl_Position=vec4((uProjectionMatrix*uWorldTransformMatrix*uTransformMatrix*vec3(p,1.)).xy,0.,1.);}`
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
interface Node {
  container: Container
  matrix: Matrix
  sprite?: Sprite
  mask?: Sprite
  mesh?: Mesh<Geometry, Shader>
  uniforms?: UniformGroup
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
    if (node.ink) {
      result.sprite = new Sprite()
      container.addChild(result.sprite)
      if (node.pose.clip) {
        result.mask = new Sprite(Texture.WHITE)
        container.addChild(result.mask)
        result.sprite.mask = result.mask
      }
    } else if (node.rect) {
      const uniforms = new UniformGroup({
        uBox: { value: new Float32Array(4), type: 'vec4<f32>' },
        uRect: { value: new Float32Array(4), type: 'vec4<f32>' },
        uFill: { value: rgb(node.rect.fill), type: 'vec3<f32>' },
        uShadow: { value: new Float32Array(4), type: 'vec4<f32>' },
        uShadowColor: { value: rgb(node.rect.shadow?.hex ?? node.rect.fill), type: 'vec3<f32>' },
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
    if (this.destroyed) throw new ClipInkError('CLIP_INK_CANCELLED')
    const { bounds } = scene,
      {width,height} = inkRasterDimensions(bounds)
    if (!this.surface) {
      this.surface = RenderTexture.create({ width, height, resolution: 1, antialias: true })
      this.quad = new Sprite(this.surface)
      this.presentation.addChild(this.quad)
    } else if (this.surface.width !== width || this.surface.height !== height)
      this.surface.resize(width, height)
    const active = new Set<string>()
    for (const current of scene.nodes) {
      const key = current.id + '/' + (current.document?.key ?? 'rect')
      active.add(key)
      let node = this.nodes.get(key)
      if (!node) {
        node = this.create(current)
        this.nodes.set(key, node)
      }
      const m = current.pose.matrix
      node.matrix.set(...m)
      node.container.setFromMatrix(node.matrix)
      node.container.visible =
        current.pose.opacity > 0 && (!current.pose.clip || current.pose.clip.width > 0)
      if (current.ink && current.document && node.sprite) {
        const texture = current.ink.gpu(this.owner, () => {
          const value = new Texture({
            source: new ImageSource({
              resource: current.ink!.bitmap,
              alphaMode: 'premultiplied-alpha',
            }),
          })
          return { value, destroy: () => value.destroy(true) }
        })
        node.sprite.texture = texture
        node.sprite.width = current.document.bounds.width
        node.sprite.height = current.document.bounds.height
        node.sprite.position.set(
          current.document.bounds.x + current.ink.offset.x,
          current.document.bounds.y + current.ink.offset.y,
        )
        node.sprite.alpha = current.pose.opacity
        node.sprite.tint = current.pose.tint ?? 0xffffff
        if (node.mask && current.pose.clip) {
          const clip = current.pose.clip
          node.mask.position.set(clip.x, clip.y)
          node.mask.width = clip.width
          node.mask.height = clip.height
        }
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
        uniforms.uFill.set(rgb(current.rect.fill))
        uniforms.uShadow.set(s ? [s.dx, s.dy, s.blur / 2, s.alpha] : [0, 0, 0, 0])
        uniforms.uAlpha = current.pose.opacity
        node.uniforms.update()
      }
    }
    for (const [key, node] of this.nodes)
      if (!active.has(key)) {
        node.mesh?.shader?.destroy()
        node.container.destroy({ children: true })
        this.nodes.delete(key)
      }
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
    return {
      nodes: this.nodes.size,
      groupBytes: (this.surface?.width ?? 0) * (this.surface?.height ?? 0) * 4,
    }
  }
  destroy() {
    if (this.destroyed) return
    this.destroyed = true
    this.releaseGPU(this.owner)
    for (const node of this.nodes.values()) {
      node.mesh?.shader?.destroy()
      node.container.destroy({ children: true })
    }
    this.nodes.clear()
    this.group.destroy()
    this.presentation.destroy({ children: true })
    this.surface?.destroy(true)
    this.surface = undefined
    this.quad = undefined
    this.geometry.destroy()
    this.program.destroy()
  }
}
