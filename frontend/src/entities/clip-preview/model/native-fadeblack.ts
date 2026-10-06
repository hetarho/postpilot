import { ClipInkError } from './ink-typography'

/** xfade's Y=0 black is below limited-range video black. Apply its clipped
 * offset to the same decoded RGB mix used by final footage, on the GPU. */
export class NativeFadeBlackSurface {
  private canvas: OffscreenCanvas
  private gl: WebGL2RenderingContext
  private program: WebGLProgram
  private texture: WebGLTexture
  private buffer: WebGLBuffer
  private rest: WebGLUniformLocation
  private closed = false
  constructor(width: number, height: number) {
    if (
      !Number.isSafeInteger(width) ||
      !Number.isSafeInteger(height) ||
      width * height > 1920 * 1080 ||
      width < 1 ||
      height < 1
    )
      throw new ClipInkError('CLIP_BACKGROUND_MEMORY_LIMIT')
    this.canvas = new OffscreenCanvas(width, height)
    const gl = this.canvas.getContext('webgl2', { alpha: false, preserveDrawingBuffer: true })
    if (!gl) {
      this.canvas.width = 0
      this.canvas.height = 0
      throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
    }
    this.gl = gl
    let vertex: WebGLShader | undefined, fragment: WebGLShader | undefined
    let program: WebGLProgram | null = null,
      texture: WebGLTexture | null = null,
      buffer: WebGLBuffer | null = null
    try {
      const compile = (type: number, source: string) => {
        const shader = gl.createShader(type)
        if (!shader) throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
        gl.shaderSource(shader, source)
        gl.compileShader(shader)
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
          gl.deleteShader(shader)
          throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
        }
        return shader
      }
      vertex = compile(
        gl.VERTEX_SHADER,
        `#version 300 es
      in vec2 aPosition; out vec2 vUV;
      void main(){gl_Position=vec4(aPosition,0.0,1.0);vUV=vec2((aPosition.x+1.0)*0.5,(1.0-aPosition.y)*0.5);}`,
      )
      fragment = compile(
        gl.FRAGMENT_SHADER,
        `#version 300 es
      precision highp float; in vec2 vUV; uniform sampler2D uImage; uniform float uRest; out vec4 color;
      void main(){vec3 rgb=texture(uImage,vUV).rgb;float belowBlack=16.0*1.164384/255.0*uRest;color=vec4(clamp(rgb-vec3(belowBlack),0.0,1.0),1.0);}`,
      )
      program = gl.createProgram()
      texture = gl.createTexture()
      buffer = gl.createBuffer()
      if (!program || !texture || !buffer)
        throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
      this.program = program
      this.texture = texture
      this.buffer = buffer
      gl.attachShader(program, vertex)
      gl.attachShader(program, fragment)
      gl.linkProgram(program)
      gl.deleteShader(vertex)
      gl.deleteShader(fragment)
      if (!gl.getProgramParameter(program, gl.LINK_STATUS))
        throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
      gl.useProgram(program)
      gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
      gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW)
      const position = gl.getAttribLocation(program, 'aPosition')
      gl.enableVertexAttribArray(position)
      gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
      gl.activeTexture(gl.TEXTURE0)
      gl.bindTexture(gl.TEXTURE_2D, texture)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
      gl.uniform1i(gl.getUniformLocation(program, 'uImage'), 0)
      const rest = gl.getUniformLocation(program, 'uRest')
      if (!rest) throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
      this.rest = rest
    } catch (error) {
      if (vertex) gl.deleteShader(vertex)
      if (fragment) gl.deleteShader(fragment)
      if (buffer) gl.deleteBuffer(buffer)
      if (texture) gl.deleteTexture(texture)
      if (program) gl.deleteProgram(program)
      gl.getExtension('WEBGL_lose_context')?.loseContext()
      this.canvas.width = 0
      this.canvas.height = 0
      throw error
    }
  }
  apply(source: OffscreenCanvas, context: OffscreenCanvasRenderingContext2D, rest: number) {
    if (this.closed || !Number.isFinite(rest) || rest < 0 || rest > 1 || this.gl.isContextLost())
      throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
    const gl = this.gl
    gl.viewport(0, 0, this.canvas.width, this.canvas.height)
    gl.useProgram(this.program)
    gl.bindTexture(gl.TEXTURE_2D, this.texture)
    gl.pixelStorei(gl.UNPACK_COLORSPACE_CONVERSION_WEBGL, gl.NONE)
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, source)
    gl.uniform1f(this.rest, rest)
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4)
    if (gl.getError() !== gl.NO_ERROR)
      throw new ClipInkError('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
    context.save()
    context.globalAlpha = 1
    context.globalCompositeOperation = 'copy'
    context.drawImage(this.canvas, 0, 0)
    context.restore()
  }
  close() {
    if (this.closed) return
    this.closed = true
    this.gl.deleteBuffer(this.buffer)
    this.gl.deleteTexture(this.texture)
    this.gl.deleteProgram(this.program)
    this.gl.getExtension('WEBGL_lose_context')?.loseContext()
    this.canvas.width = 0
    this.canvas.height = 0
  }
}
