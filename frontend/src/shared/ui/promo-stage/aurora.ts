import { PROMO_AURORA_MIN_FRAME_MS, PROMO_AURORA_RESOLUTION_SCALE } from '@/shared/config'
import { prefersReducedMotion } from '@/shared/lib'

/** The aurora behind a promotional stage, drawn by a fragment shader (THEME-37).
 *
 *  Three layers of drifting fractal noise, each tinted with one of the promotional hues, folded
 *  into the stage's recessed ground so the picture starts from the surface the cards stand on
 *  rather than from black. The colours are the SAME roles the CSS fallback wears: they are read
 *  from the stylesheet at start and again whenever the theme flips, so the shader re-skins with
 *  `data-theme` exactly like everything else and no colour is ever written here.
 *
 *  Costs are bounded on purpose: half-resolution (the picture is smooth noise the browser scales
 *  up), at most ~30 frames a second, paused while the stage is off-screen, and a single still frame
 *  under reduced motion. Where WebGL is missing — jsdom, a locked-down browser — `startAurora`
 *  returns `null` and the caller keeps its CSS fallback. */
export interface AuroraHandle {
  stop(): void
}

const VERTEX_SHADER = `
attribute vec2 a_position;
void main() {
  gl_Position = vec4(a_position, 0.0, 1.0);
}`

const FRAGMENT_SHADER = `
precision mediump float;
uniform vec2 u_resolution;
uniform float u_time;
uniform vec3 u_ground;
uniform vec3 u_hue0;
uniform vec3 u_hue1;
uniform vec3 u_hue2;

float hash(vec2 p) {
  p = fract(p * vec2(123.34, 456.21));
  p += dot(p, p + 45.32);
  return fract(p.x * p.y);
}

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  return mix(
    mix(hash(i), hash(i + vec2(1.0, 0.0)), u.x),
    mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), u.x),
    u.y
  );
}

float fbm(vec2 p) {
  float value = 0.0;
  float amplitude = 0.5;
  for (int i = 0; i < 4; i++) {
    value += amplitude * noise(p);
    p = p * 2.03 + vec2(1.7, 9.2);
    amplitude *= 0.5;
  }
  return value;
}

void main() {
  vec2 uv = gl_FragCoord.xy / u_resolution;
  vec2 p = vec2(uv.x * u_resolution.x / u_resolution.y, uv.y);
  float t = u_time * 0.045;
  float n0 = fbm(p * 1.1 + vec2(t, -t * 0.6));
  float n1 = fbm(p * 1.5 + vec2(-t * 0.7, t * 0.4) + 4.2);
  float n2 = fbm(p * 0.8 + vec2(t * 0.5, t * 0.8) + 9.1);
  vec3 colour = u_ground;
  colour = mix(colour, u_hue0, smoothstep(0.42, 0.80, n0) * 0.62);
  colour = mix(colour, u_hue1, smoothstep(0.46, 0.84, n1) * 0.55);
  colour = mix(colour, u_hue2, smoothstep(0.50, 0.88, n2) * 0.42);
  // The rim stays close to the ground so the panel's edge still reads as an edge.
  float rim = smoothstep(0.0, 0.3, uv.x) * smoothstep(0.0, 0.3, 1.0 - uv.x)
    * smoothstep(0.0, 0.4, uv.y) * smoothstep(0.0, 0.4, 1.0 - uv.y);
  colour = mix(u_ground, colour, 0.4 + 0.6 * rim);
  gl_FragColor = vec4(colour, 1.0);
}`

type Rgb = [number, number, number]

/** The roles the shader paints with, in uniform order: the ground and the three hues. These are
 *  the Tier 3 role properties `index.css` declares on `:root`, not the `--color-*` names the
 *  utilities use: `@theme inline` inlines those into the utilities it generates and never emits
 *  them as runtime variables, so only the role itself is there to be read. */
const ROLES = [
  '--surface-recessed',
  '--promo-aurora-1',
  '--promo-aurora-2',
  '--promo-aurora-3',
] as const

/** Reads the four roles off the stylesheet as sRGB. The browser does the colour maths: the
 *  token's OKLCH string is painted onto a one-pixel 2D canvas and read back, which is the only way
 *  to resolve a token into numbers without a second copy of the palette living in TypeScript. */
function readPalette(element: Element): Rgb[] | null {
  const probe = document.createElement('canvas')
  probe.width = 1
  probe.height = 1
  const context = probe.getContext('2d', { willReadFrequently: true })
  if (!context) return null
  const style = getComputedStyle(element)
  const palette: Rgb[] = []
  for (const role of ROLES) {
    const value = style.getPropertyValue(role).trim()
    if (!value) return null
    context.clearRect(0, 0, 1, 1)
    context.fillStyle = value
    context.fillRect(0, 0, 1, 1)
    const [r, g, b] = context.getImageData(0, 0, 1, 1).data
    palette.push([r / 255, g / 255, b / 255])
  }
  return palette
}

function compile(gl: WebGLRenderingContext, type: number, source: string): WebGLShader | null {
  const shader = gl.createShader(type)
  if (!shader) return null
  gl.shaderSource(shader, source)
  gl.compileShader(shader)
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    gl.deleteShader(shader)
    return null
  }
  return shader
}

/** Starts the aurora on `canvas`, or returns `null` where it cannot run. */
export function startAurora(canvas: HTMLCanvasElement): AuroraHandle | null {
  // jsdom has no `WebGLRenderingContext` at all; asking it for a context only logs a complaint.
  if (typeof WebGLRenderingContext === 'undefined') return null
  const gl = canvas.getContext('webgl', {
    alpha: false,
    antialias: false,
    depth: false,
    stencil: false,
    powerPreference: 'low-power',
  })
  if (!gl || gl.isContextLost()) return null
  const palette = readPalette(canvas)
  if (!palette) return null

  const vertex = compile(gl, gl.VERTEX_SHADER, VERTEX_SHADER)
  const fragment = compile(gl, gl.FRAGMENT_SHADER, FRAGMENT_SHADER)
  const program = gl.createProgram()
  if (!vertex || !fragment || !program) return null
  gl.attachShader(program, vertex)
  gl.attachShader(program, fragment)
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return null
  gl.useProgram(program)

  // One triangle that covers the clip space; the fragment shader does the rest.
  const buffer = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW)
  const position = gl.getAttribLocation(program, 'a_position')
  gl.enableVertexAttribArray(position)
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)

  const uniforms = {
    resolution: gl.getUniformLocation(program, 'u_resolution'),
    time: gl.getUniformLocation(program, 'u_time'),
    colours: [
      gl.getUniformLocation(program, 'u_ground'),
      gl.getUniformLocation(program, 'u_hue0'),
      gl.getUniformLocation(program, 'u_hue1'),
      gl.getUniformLocation(program, 'u_hue2'),
    ],
  }
  const applyPalette = (colours: Rgb[]) => {
    colours.forEach((rgb, index) => gl.uniform3fv(uniforms.colours[index], rgb))
  }
  applyPalette(palette)

  const still = prefersReducedMotion()
  let frame = 0
  let lastDrawn = -Infinity
  let visible = true
  let stopped = false
  const started = performance.now()

  const draw = (now: number) => {
    gl.viewport(0, 0, canvas.width, canvas.height)
    gl.uniform2f(uniforms.resolution, canvas.width, canvas.height)
    gl.uniform1f(uniforms.time, still ? 0 : (now - started) / 1000)
    gl.drawArrays(gl.TRIANGLES, 0, 3)
  }
  const loop = (now: number) => {
    frame = 0
    if (stopped || !visible) return
    if (now - lastDrawn >= PROMO_AURORA_MIN_FRAME_MS) {
      lastDrawn = now
      draw(now)
    }
    if (!still) frame = requestAnimationFrame(loop)
  }
  const schedule = () => {
    if (stopped || frame !== 0 || !visible) return
    frame = requestAnimationFrame(loop)
  }

  // The canvas is sized to its box at a fraction of the device's pixels; a resize redraws even
  // under reduced motion, because a stretched still frame is a blurry one.
  const resize = () => {
    const box = canvas.getBoundingClientRect()
    const scale = Math.min(window.devicePixelRatio || 1, 2) * PROMO_AURORA_RESOLUTION_SCALE
    canvas.width = Math.max(1, Math.round(box.width * scale))
    canvas.height = Math.max(1, Math.round(box.height * scale))
    lastDrawn = -Infinity
    schedule()
  }
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(canvas)

  // Off-screen, the loop stops rather than paints into a scrolled-away box.
  const intersection =
    typeof IntersectionObserver === 'function'
      ? new IntersectionObserver((entries) => {
          visible = entries.some((entry) => entry.isIntersecting)
          if (visible) {
            lastDrawn = -Infinity
            schedule()
          }
        })
      : null
  intersection?.observe(canvas)

  // The theme flips by attribute on <html>; the roles are re-read and the next frame wears them.
  const themeObserver = new MutationObserver(() => {
    const next = readPalette(canvas)
    if (next) {
      applyPalette(next)
      lastDrawn = -Infinity
      schedule()
    }
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme'],
  })

  resize()

  return {
    stop() {
      stopped = true
      if (frame) cancelAnimationFrame(frame)
      resizeObserver.disconnect()
      intersection?.disconnect()
      themeObserver.disconnect()
      // The context is handed back only once the canvas has left the document. A canvas that is
      // still mounted is about to be started again — React's development double-effect, a Fast
      // Refresh — and a context lost here would come back lost, leaving the fallback to show.
      if (!canvas.isConnected) gl.getExtension('WEBGL_lose_context')?.loseContext()
    },
  }
}
