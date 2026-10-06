import { Resvg, initWasm } from '@resvg/resvg-wasm'
import wasmURL from '@resvg/resvg-wasm/index_bg.wasm?url'
import {
  CLIP_INK,
  clipInkFont,
  clipInkMetrics,
  type ClipInkFont,
} from '@/entities/clip-design/@x/clip-preview'
import {
  ClipInkError,
  inkNumber,
  inkRasterDimensions,
  inkRuns,
  inkTextMarkup,
  type InkBox,
  type InkRole,
} from './ink-typography'

export interface InkDocument {
  key: string
  svg: string
  fonts: readonly ClipInkFont[]
  bounds: InkBox
  phase?: { x: number; y: number }
  placement?: InkBox
  rasterScale?: number
}
export interface BrowserInkRasterizer {
  measure(text: string, role: InkRole, caption: boolean, signal?: AbortSignal): Promise<InkBox>
  render(document: InkDocument, signal?: AbortSignal): Promise<ImageBitmap>
  destroy(): void
}
let wasm: Promise<void> | undefined
const initialize = () =>
  (wasm ??= (async () => {
    const response = await fetch(wasmURL)
    if (!response.ok) throw new ClipInkError('CLIP_INK_WASM_UNAVAILABLE')
    const bytes = await response.arrayBuffer()
    const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
    const hash = [...digest].map((b) => b.toString(16).padStart(2, '0')).join('')
    if (hash !== CLIP_INK.wasmSHA256) throw new ClipInkError('CLIP_INK_INCOMPATIBLE_WASM')
    await initWasm(bytes)
  })().catch((error) => {
    wasm = undefined
    throw error
  }))

/** Explicit font buffers make system discovery impossible in the WASM adapter. */
export class ResvgBrowserInk implements BrowserInkRasterizer {
  private buffers = new Map<string, Promise<Uint8Array>>()
  private measurements = new Map<string, InkBox>()
  private generation = 0
  private destroyed = false
  private fontController = new AbortController()
  constructor(
    private load: (url: string, signal?: AbortSignal) => Promise<Uint8Array> = async (
      url,
      signal,
    ) => {
      const response = await fetch(url, { signal })
      if (!response.ok) throw new ClipInkError('CLIP_INK_FONT_UNAVAILABLE', url)
      return new Uint8Array(await response.arrayBuffer())
    },
  ) {}
  private check(generation: number, signal?: AbortSignal) {
    if (this.destroyed || signal?.aborted || generation !== this.generation)
      throw new ClipInkError('CLIP_INK_CANCELLED')
  }
  private async font(font: ClipInkFont) {
    const approved = clipInkFont(font.face, font.weight)
    if (JSON.stringify(font) !== JSON.stringify(approved))
      throw new ClipInkError('CLIP_INK_UNSUPPORTED_FONT', font.file)
    let value = this.buffers.get(font.sha256)
    if (!value) {
      value = this.load(CLIP_INK.fontBaseURL + font.file, this.fontController.signal)
        .then(async (bytes) => {
          if (bytes.byteLength !== font.bytes || bytes.byteLength > CLIP_INK.maxFontBytes)
            throw new ClipInkError('CLIP_INK_FONT_CHANGED', font.file)
          const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes.slice().buffer))
          const hash = [...digest].map((b) => b.toString(16).padStart(2, '0')).join('')
          if (hash !== font.sha256) throw new ClipInkError('CLIP_INK_FONT_CHANGED', font.file)
          return bytes
        })
        .catch((error) => {
          this.buffers.delete(font.sha256)
          throw error
        })
      this.buffers.set(font.sha256, value)
    }
    return value
  }
  async measure(
    text: string,
    role: InkRole,
    caption: boolean,
    signal?: AbortSignal,
  ): Promise<InkBox> {
    const runs = inkRuns(text, role, caption)
    const fonts = [
      ...new Map(
        [role.face, ...runs.map((run) => run.face)].map((face) => {
          const font = clipInkFont(face, role.weight)
          return [font.sha256, font] as const
        }),
      ).values(),
    ]
    const key = JSON.stringify([
      CLIP_INK.version,
      text,
      role.face,
      role.weight,
      role.tracking,
      fonts.map((f) => f.sha256),
    ])
    const known = this.measurements.get(key)
    const generation = this.generation
    this.check(generation, signal)
    if (known) {
      this.measurements.delete(key)
      this.measurements.set(key, known)
      return { ...known }
    }
    await initialize()
    const fontBuffers = await Promise.all(fonts.map((f) => this.font(f)))
    this.check(generation, signal)
    const family = clipInkFont(role.face, role.weight).family
    const markup = inkTextMarkup(text, role, caption)
    const position = (anchor: string) => {
      const r = new Resvg(
        `<svg xmlns="http://www.w3.org/2000/svg" width="10000" height="500"><text x="0" y="200" xml:space="preserve" text-anchor="${anchor}" font-family="${family}" font-size="100" font-weight="${role.weight}" letter-spacing="${inkNumber(role.tracking * 100)}">${markup}</text></svg>`,
        { font: { fontBuffers, defaultFontFamily: family } },
      )
      try {
        const box = r.getBBox()
        if (!box) throw new ClipInkError('CLIP_INK_INVALID_TEXT', text)
        try {
          return box.x
        } finally {
          box.free()
        }
      } finally {
        r.free()
      }
    }
    // Native 0.48 query-all reports logical text bounds. Old WASM exposes tight
    // ink; anchor displacement gives the exact shaped advance without pixels.
    const width = position('start') - position('end')
    let ascent = 0
    let descent = 0
    for (const run of runs) {
      const m = clipInkMetrics(run.face, role.weight)
      ascent = Math.max(ascent, (m.ascent / m.units) * 100)
      descent = Math.max(descent, (-m.descent / m.units) * 100)
    }
    const nativeRound = (value: number) => Math.round(value * 1000) / 1000
    const result = {
      x: 0,
      y: nativeRound(-ascent),
      width: nativeRound(width),
      height: nativeRound(ascent + descent),
    }
    this.check(generation, signal)
    this.measurements.set(key, result)
    while (this.measurements.size > CLIP_INK.measurements)
      this.measurements.delete(this.measurements.keys().next().value!)
    return { ...result }
  }
  async render(document: InkDocument, signal?: AbortSignal): Promise<ImageBitmap> {
    const generation = this.generation
    this.check(generation, signal)
    const dimensions = inkRasterDimensions(document.bounds, document.rasterScale)
    await initialize()
    const fontBuffers = await Promise.all(document.fonts.map((f) => this.font(f)))
    this.check(generation, signal)
    const r = new Resvg(document.svg, {
      font: { fontBuffers, defaultFontFamily: document.fonts[0]?.family },
      fitTo: { mode: 'zoom', value: document.rasterScale ?? 1 },
    })
    try {
      if (
        r.width !== Math.ceil(document.bounds.width) ||
        r.height !== Math.ceil(document.bounds.height)
      )
        throw new ClipInkError('CLIP_INK_INVALID_GEOMETRY')
      if (r.imagesToResolve().length) throw new ClipInkError('CLIP_INK_EXTERNAL_RESOURCE')
      const pixels = r.render()
      let bitmap: ImageBitmap
      try {
        bitmap = await createImageBitmap(
          new Blob([pixels.asPng().slice().buffer], { type: 'image/png' }),
        )
      } finally {
        pixels.free()
      }
      try {
        this.check(generation, signal)
        if (bitmap.width !== dimensions.width || bitmap.height !== dimensions.height)
          throw new ClipInkError('CLIP_INK_INVALID_GEOMETRY', 'raster dimensions')
      } catch (error) {
        bitmap.close()
        throw error
      }
      return bitmap
    } finally {
      r.free()
    }
  }
  destroy() {
    this.destroyed = true
    this.fontController.abort()
    this.generation++
    this.measurements.clear()
    this.buffers.clear()
  }
}
