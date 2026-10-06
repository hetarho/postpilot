import {
  AlphaFilter,
  Container,
  Geometry,
  GlProgram,
  Mesh,
  Rectangle,
  Shader,
  UniformGroup,
} from 'pixi.js'
import { captionGaussian } from './ink-caption-filter-pixi'
import { inkEmberFilterBounds } from './ink-caption-ember'
import type { InkCaptionPose, InkCaptionSceneNode } from './ink-caption-scene'

const color = (hex: string) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
const vertex = `#version 300 es
in vec2 aPosition;
uniform mat3 uProjectionMatrix;
uniform mat3 uWorldTransformMatrix;
uniform mat3 uTransformMatrix;
uniform vec4 uBox;
out vec2 vPoint;
void main(){vec2 p=uBox.xy+aPosition*uBox.zw;vPoint=p;gl_Position=vec4((uProjectionMatrix*uWorldTransformMatrix*uTransformMatrix*vec3(p,1.)).xy,0.,1.);}`
const flameFragment = `#version 300 es
in vec2 vPoint;
uniform vec2 uCurve[7];
uniform vec4 uGradientBox;
uniform vec4 uStops[4];
uniform vec4 uColor;
out vec4 finalColor;
vec2 cubic(vec2 a,vec2 b,vec2 c,vec2 d,float t){float s=1.-t;return a*s*s*s+3.*b*s*s*t+3.*c*s*t*t+d*t*t*t;}
float curveX(float y,int side){vec2 a=side==0?uCurve[0]:uCurve[3],b=side==0?uCurve[1]:uCurve[4],c=side==0?uCurve[2]:uCurve[5],d=side==0?uCurve[3]:uCurve[6];float lo=0.,hi=1.;for(int i=0;i<18;i++){float t=(lo+hi)*.5;float at=cubic(a,b,c,d,t).y;if((side==0&&at>y)||(side==1&&at<y))lo=t;else hi=t;}return cubic(a,b,c,d,(lo+hi)*.5).x;}
void main(){
 float left=curveX(vPoint.y,0),right=curveX(vPoint.y,1),edge=min(vPoint.x-left,right-vPoint.x);
 float horizontal=clamp(.5+edge/max(fwidth(edge),.0001),0.,1.);
 float vertical=clamp(.5+min(vPoint.y-uCurve[3].y,uCurve[0].y-vPoint.y)/max(fwidth(vPoint.y),.0001),0.,1.);
 vec2 p=(vPoint-uGradientBox.xy)/uGradientBox.zw;float radius=length(p-vec2(.5,.88))/.68;
 vec4 c=radius<.28?mix(uStops[0],uStops[1],clamp(radius/.28,0.,1.)):radius<.62?mix(uStops[1],uStops[2],clamp((radius-.28)/.34,0.,1.)):mix(uStops[2],uStops[3],clamp((radius-.62)/.38,0.,1.));
 float a=c.a*horizontal*vertical;finalColor=vec4(c.rgb*a,a)*uColor;
}`
const sparkFragment = `#version 300 es
in vec2 vPoint;
uniform vec4 uCircle;
uniform vec3 uFill;
uniform vec4 uColor;
out vec4 finalColor;
void main(){float edge=uCircle.z-length(vPoint-uCircle.xy);float a=clamp(.5+edge/max(fwidth(edge),.0001),0.,1.)*uCircle.w;finalColor=vec4(uFill*a,a)*uColor;}`

/** Fixed quads/shaders; native monotone cubic paths are evaluated in bounded
 * fragments. No per-frame SVG, triangulation buffer or text texture is built. */
export class BrowserCaptionEmberPixi {
  readonly group = new Container()
  private geometry = new Geometry({
    attributes: {
      aPosition: { buffer: new Float32Array([0, 0, 1, 0, 1, 1, 0, 1]), format: 'float32x2' },
    },
    indexBuffer: new Uint32Array([0, 1, 2, 0, 2, 3]),
  })
  private program: GlProgram
  private meshes: { mesh: Mesh<Geometry, Shader>; uniforms: UniformGroup }[] = []
  private blur?: ReturnType<typeof captionGaussian>
  private alpha?: AlphaFilter
  private destroyed = false
  constructor(private node: Pick<InkCaptionSceneNode, 'flames' | 'sparks'>) {
    this.program = new GlProgram({
      vertex,
      fragment: node.flames ? flameFragment : sparkFragment,
      name: node.flames ? 'clip-ember-cubic-srgb' : 'clip-ember-spark-srgb',
      preferredFragmentPrecision: 'highp',
    })
    if (node.flames) {
      this.blur = captionGaussian(node.flames.sigma)
      this.alpha = new AlphaFilter({ alpha: 1 })
      this.group.filters = [this.blur, this.alpha]
    }
  }
  private create() {
    const uniforms = this.node.flames
      ? new UniformGroup({
          uBox: { value: new Float32Array(4), type: 'vec4<f32>' },
          uCurve: { value: new Float32Array(14), type: 'vec2<f32>', size: 7 },
          uGradientBox: { value: new Float32Array(4), type: 'vec4<f32>' },
          uStops: {
            value: new Float32Array(
              this.node.flames.stops.flatMap((s) => [...color(s.hex), s.alpha]),
            ),
            type: 'vec4<f32>',
            size: 4,
          },
        })
      : new UniformGroup({
          uBox: { value: new Float32Array(4), type: 'vec4<f32>' },
          uCircle: { value: new Float32Array(4), type: 'vec4<f32>' },
          uFill: { value: new Float32Array(color(this.node.sparks!.fill)), type: 'vec3<f32>' },
        })
    const mesh = new Mesh({
      geometry: this.geometry,
      shader: new Shader({ glProgram: this.program, resources: { shape: uniforms } }),
    })
    this.group.addChild(mesh)
    return { mesh, uniforms }
  }
  update(pose: InkCaptionPose) {
    const shapes = pose.flames ?? pose.sparks ?? []
    while (this.meshes.length < shapes.length) this.meshes.push(this.create())
    while (this.meshes.length > shapes.length) {
      const old = this.meshes.pop()!
      old.mesh.shader!.destroy()
      old.mesh.destroy()
    }
    for (const [i, { mesh, uniforms }] of this.meshes.entries()) {
      const u = uniforms.uniforms
      if (pose.flames) {
        const flame = pose.flames[i]!,
          b = flame.bounds
        ;(u.uBox as Float32Array).set([b.x - 1, b.y - 1, b.width + 2, b.height + 2])
        ;(u.uGradientBox as Float32Array).set([b.x, b.y, b.width, b.height])
        ;(u.uCurve as Float32Array).set(flame.points)
        mesh.visible = true
      } else if (pose.sparks) {
        const spark = pose.sparks[i]!,
          r = spark.radius
        ;(u.uBox as Float32Array).set([spark.cx - r - 1, spark.cy - r - 1, 2 * r + 2, 2 * r + 2])
        ;(u.uCircle as Float32Array).set([spark.cx, spark.cy, r, spark.alpha])
        mesh.visible = r > 0
      }
      uniforms.update()
    }
    if (pose.flames && this.alpha) {
      const b = inkEmberFilterBounds(pose.flames)
      this.group.filterArea = new Rectangle(
        b.x + pose.matrix[4],
        b.y + pose.matrix[5],
        b.width,
        b.height,
      )
      // Native opacity belongs to the composed/blurred layer, not each tongue.
      this.alpha.alpha = pose.opacity
    }
  }
  measurements() {
    return {
      shapes: this.meshes.length,
      filters: this.destroyed || !this.node.flames ? 0 : 2,
      geometryBytes: this.destroyed ? 0 : 56,
      uniformBytes: this.meshes.reduce(
        (sum, { uniforms }) =>
          sum +
          Object.values(uniforms.uniforms).reduce<number>(
            (bytes, v) => bytes + (ArrayBuffer.isView(v) ? v.byteLength : 0),
            0,
          ),
        0,
      ),
    }
  }
  destroy() {
    if (this.destroyed) return
    this.destroyed = true
    for (const { mesh } of this.meshes) mesh.shader!.destroy()
    this.group.destroy({ children: true })
    this.meshes = []
    this.blur?.destroy()
    this.alpha?.destroy()
    this.geometry.destroy()
    this.program.destroy()
  }
}
