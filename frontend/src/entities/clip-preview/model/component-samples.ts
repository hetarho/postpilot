import {
  CLIP_CAPTION_STYLES,
  CLIP_DESIGN,
  type ClipRatioId,
} from '@/entities/clip-design/@x/clip-preview'
import type {
  ClipEditPlan,
  ClipEditableText,
  ClipCaptionFragment,
  ClipCaptionPreview,
  ClipRegionPresetSamples,
} from '@/entities/clip-plan/@x/clip-preview'
import { BrowserLocalComponents } from './local-components'
import {
  evaluateBrowserFrame,
  freezeBrowserPreviewComposition,
  type BrowserCompositionSnapshot,
} from './browser-composition'
import { inkRegionCapacity } from './ink-static'
import { CLIP_LOCAL_PREVIEW } from '../config/local-preview'

function surface(snapshot: BrowserCompositionSnapshot) {
  if (typeof OffscreenCanvas === 'undefined') throw new Error('CLIP_PREVIEW_CANVAS_UNAVAILABLE')
  const size = CLIP_DESIGN.ratios[snapshot.ratio].canvas,
    canvas = new OffscreenCanvas(size.width, size.height),
    context = canvas.getContext('2d')
  if (!context) throw new Error('CLIP_CANVAS_UNAVAILABLE')
  return { canvas, context }
}
async function fragmentSvg(canvas: OffscreenCanvas, x: number, y: number, signal: AbortSignal) {
  const blob = await canvas.convertToBlob({ type: 'image/png' })
  signal.throwIfAborted()
  if (blob.size > CLIP_LOCAL_PREVIEW.sampleBytes) throw new Error('CLIP_PREVIEW_TOO_LARGE')
  const bytes = new Uint8Array(await blob.arrayBuffer())
  let encoded = ''
  for (const value of bytes) encoded += String.fromCharCode(value)
  return `<image x="${-x}" y="${-y}" width="${canvas.width}" height="${canvas.height}" href="data:image/png;base64,${btoa(encoded)}"/>`
}
/** First rapid phrase and native measured box, held at the approved representative state. */
export async function localCaptionFragments(
  snapshot: BrowserCompositionSnapshot,
  local: BrowserLocalComponents,
  signal: AbortSignal,
  selectedId?: string,
): Promise<ClipCaptionPreview> {
  await local.resolveLayout(signal)
  const size = CLIP_DESIGN.ratios[snapshot.ratio],
    captions: ClipCaptionFragment[] = []
  for (const id of new Set(
    snapshot.components
      .filter((component) => component.element.role === 'caption')
      .map((component) => component.instanceId),
  )) {
    const geometry = await local.captionGeometry(id, signal)
    if (!geometry) continue
    let svg = ''
    if (id === selectedId) {
      const component = snapshot.components.find((component) => component.instanceId === id)!,
        frame = evaluateBrowserFrame(snapshot, component.visibleFirstFrame)
      frame.components = frame.components
        .filter((state) => state.component === component)
        .map((state) => ({ ...state, progress: 0.5, animationProgress: 0.5 }))
      const { canvas, context } = surface(snapshot)
      const resources = await local.prepare(frame, signal, undefined, { representative: true })
      try {
        resources.forEach((resource) => resource.draw(context))
        svg = await fragmentSvg(canvas, geometry.box.x, geometry.box.y, signal)
      } finally {
        resources.forEach((resource) => resource.close())
        canvas.width = 0
        canvas.height = 0
      }
    }
    captions.push({ ...geometry, svg })
  }
  return {
    ratio: snapshot.ratio,
    canvas: { x: 0, y: 0, ...size.canvas },
    safeArea: { ...size.safe },
    captions,
  }
}
function sampleElement(text: string, style: string, pace: string): ClipEditableText {
  return {
    instanceId: 'sample',
    elementId: 'sample',
    cutId: '',
    kind: 'fixed',
    role: 'caption',
    text,
    rows: [],
    style,
    position: 'auto',
    align: 'center',
    basis: 'output-start',
    startMs: 1000,
    endMs: 3000,
    resolvedStartMs: 1000,
    resolvedEndMs: 3000,
    pace,
    accent: '',
    keyword: 'AV',
    groupId: '',
    itemId: '',
    ...(pace === 'rapid' ? { phrases: [{ text, startMs: 1000, endMs: 1800 }] } : {}),
  }
}
export async function freezeLocalSample(
  ratio: ClipRatioId,
  elements: ClipEditableText[],
  design: {
    captionPace?: string
    captionStyles?: string[]
    introPreset?: string
    outroPreset?: string
  } = {},
) {
  const size = CLIP_DESIGN.ratios[ratio].canvas
  const plan: ClipEditPlan = {
    nativeComposition: true,
    durationMs: 15000,
    elements,
    cuts: [
      {
        id: 'cut',
        sourceId: 'sample',
        fingerprint: '0'.repeat(64),
        startMs: 0,
        endMs: 15000,
        transitionMs: 0,
        volumePermille: 0,
        playbackRatePermille: 1000,
        copies: [],
      },
    ],
  }
  return freezeBrowserPreviewComposition({
    ownerId: 'local-component-catalog',
    projectId: 'representative',
    projectRevision: 1,
    planRevision: 1,
    ratio,
    plan,
    design: { ...design, hideDisclosure: true },
    sources: [
      {
        sourceId: 'sample',
        fingerprint: '0'.repeat(64),
        durationMs: 15000,
        width: size.width,
        height: size.height,
        hasAudio: false,
        allowedRatePermille: [1000],
      },
    ],
  })
}
/** One bounded, serialized generic catalog cache. It holds no owner's words or original. */
class LocalComponentSamples {
  private local?: BrowserLocalComponents
  private queue: Promise<unknown> = Promise.resolve()
  private cache = new Map<
    string,
    { value: ClipCaptionPreview | ClipRegionPresetSamples; bytes: number }
  >()
  private bytes = 0
  async get<T extends ClipCaptionPreview | ClipRegionPresetSamples>(
    key: string,
    signal: AbortSignal,
    render: () => Promise<T>,
  ): Promise<T> {
    const cached = this.cache.get(key)
    if (cached) {
      signal.throwIfAborted()
      this.cache.delete(key)
      this.cache.set(key, cached)
      return cached.value as T
    }
    const pending = this.queue.then(async () => {
      signal.throwIfAborted()
      const value = await render()
      signal.throwIfAborted()
      const bytes = JSON.stringify(value).length * 2
      if (bytes > CLIP_LOCAL_PREVIEW.sampleBytes) throw new Error('CLIP_PREVIEW_TOO_LARGE')
      const old = this.cache.get(key)
      if (old) this.bytes -= old.bytes
      this.cache.delete(key)
      this.cache.set(key, { value, bytes })
      this.bytes += bytes
      while (
        this.cache.size > CLIP_LOCAL_PREVIEW.sampleEntries ||
        this.bytes > CLIP_LOCAL_PREVIEW.sampleBytes
      ) {
        const first = this.cache.keys().next().value!
        this.bytes -= this.cache.get(first)!.bytes
        this.cache.delete(first)
      }
      return value
    })
    this.queue = pending.catch(() => {})
    return pending
  }
  private renderer(snapshot: BrowserCompositionSnapshot) {
    if (this.local) this.local.updateSnapshot(snapshot)
    else this.local = new BrowserLocalComponents(snapshot)
    return this.local
  }
  styles(ratio: ClipRatioId, pace: string, signal: AbortSignal) {
    return this.get(JSON.stringify(['styles', ratio, pace]), signal, async () => {
      const captions: ClipCaptionFragment[] = []
      for (const style of CLIP_CAPTION_STYLES) {
        const snapshot = await freezeLocalSample(
          ratio,
          [sampleElement('갂 AV 12,500원', style, pace)],
          { captionPace: pace, captionStyles: [style] },
        )
        const preview = await localCaptionFragments(
          snapshot,
          this.renderer(snapshot),
          signal,
          'sample',
        )
        captions.push({ ...preview.captions[0]!, instanceId: style })
      }
      return {
        ratio,
        canvas: { x: 0, y: 0, ...CLIP_DESIGN.ratios[ratio].canvas },
        safeArea: { ...CLIP_DESIGN.ratios[ratio].safe },
        captions,
      }
    })
  }
  presets(ratio: ClipRatioId, slotLabel: string, signal: AbortSignal) {
    return this.get(JSON.stringify(['presets', ratio, slotLabel]), signal, async () => {
      if (!slotLabel.includes('{n}') || [...slotLabel].length > 16 || /[\r\n]/u.test(slotLabel))
        throw new Error('CLIP_PREVIEW_INVALID_SLOT_LABEL')
      const value: ClipRegionPresetSamples = {
        ratio,
        canvas: { x: 0, y: 0, ...CLIP_DESIGN.ratios[ratio].canvas },
        intro: [],
        outro: [],
      }
      for (const kind of ['intro', 'outro'] as const)
        for (const preset of Object.keys(CLIP_DESIGN.regions[kind])) {
          const element = sampleElement('', '', 'steady')
          element.role = kind === 'intro' ? 'hook' : 'ending'
          element.resolvedStartMs = 0
          element.resolvedEndMs = 2500
          element.startMs = 0
          element.endMs = 2500
          element.rows = Array.from({ length: inkRegionCapacity(kind, preset) }, (_, index) => ({
            role: '',
            text: slotLabel.replace('{n}', String(index + 1)),
          }))
          const snapshot = await freezeLocalSample(ratio, [element], {
              introPreset: kind === 'intro' ? preset : 'a',
              outroPreset: kind === 'outro' ? preset : 'b',
            }),
            renderer = this.renderer(snapshot)
          const frame = evaluateBrowserFrame(snapshot, 30),
            { canvas, context } = surface(snapshot),
            resources = await renderer.prepare(frame, signal, undefined, { representative: true })
          try {
            resources.forEach((resource) => resource.draw(context))
            value[kind].push({
              preset,
              box: { x: 0, y: 0, ...CLIP_DESIGN.ratios[ratio].canvas },
              svg: await fragmentSvg(canvas, 0, 0, signal),
            })
          } finally {
            resources.forEach((resource) => resource.close())
            canvas.width = 0
            canvas.height = 0
          }
        }
      return value
    })
  }
}
export const localComponentSamples = new LocalComponentSamples()
