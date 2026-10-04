import type { AppFailure } from '@/shared/api'
import type { PreparedAsset } from './preview-assets'
import { CLIP_DRAFT_PREVIEW, CLIP_TRANSITION } from '@/entities/clip-design/@x/clip-preview'
import {
  cutOutputMs,
  cutRate,
  outputToSourceMs,
  timelineCuts,
  type ClipEditPlan,
  type ClipTimelineCut,
} from '@/entities/clip-plan/@x/clip-preview'

type ClipEditCut = ClipEditPlan['cuts'][number]

export type PreviewCut = ClipTimelineCut
export function previewTimeline(plan: ClipEditPlan): PreviewCut[] {
  if (
    plan.cuts.some(
      (c, index) =>
        !Number.isSafeInteger(c.startMs) ||
        !Number.isSafeInteger(c.endMs) ||
        c.startMs < 0 ||
        cutOutputMs(c) <=
          Math.max(
            2 * CLIP_TRANSITION.fade_ms,
            c.transitionMs + (plan.cuts[index + 1]?.transitionMs ?? 0),
          ) ||
        ![0, CLIP_TRANSITION.fade_ms, CLIP_TRANSITION.black_ms].includes(c.transitionMs),
    )
  )
    return []
  return timelineCuts(plan)
}

/** The xfade a transition is drawn with (CDS-36): through black for the 300 ms one, a plain
 *  dissolve otherwise, and none for a hard cut. */
export type TransitionKind = '' | 'fade' | 'fadeblack'
function transitionKind(transitionMs: number): TransitionKind {
  if (!transitionMs) return ''
  return transitionMs === CLIP_TRANSITION.black_ms ? 'fadeblack' : 'fade'
}

function smoothstep(edge0: number, edge1: number, x: number) {
  const t = Math.min(1, Math.max(0, (x - edge0) / (edge1 - edge0)))
  return t * t * (3 - 2 * t)
}

/** How much of the outgoing and the incoming frame the server's xfade mixes at its progress,
 *  which runs from 1 on a transition's first frame down to 0: a dissolve is the plain mix, and
 *  a fade through black mixes each side with black on its own smoothstep over xfade's phase,
 *  the rest of the frame being that black. The browser follows the server's curve, not the
 *  other way round, so a scrim the server samples on a ground is decided on the ground the
 *  browser draws (CLIP-192). */
export function transitionWeights(kind: TransitionKind, progress: number): [number, number] {
  if (!kind) return [0, 1]
  if (kind === 'fadeblack') {
    const phase = 0.2
    return [
      smoothstep(1 - phase, 1, progress) * progress,
      (1 - smoothstep(phase, 1, progress)) * (1 - progress),
    ]
  }
  return [progress, 1 - progress]
}

/** The alpha each side is drawn with, outgoing first, so that drawing them in turn over the
 *  black matte leaves each at its weight: the incoming side at its own, the outgoing one at
 *  what still shows through it. A dissolve keeps the outgoing frame whole beneath it. */
export function transitionAlphas(kind: TransitionKind, progress: number): [number, number] {
  const [outgoing, incoming] = transitionWeights(kind, progress)
  if (kind !== 'fadeblack') return [1, incoming]
  return [incoming < 1 ? outgoing / (1 - incoming) : 0, incoming]
}

/** Half-open output intervals; an exact cut boundary belongs to the new cut.
 * At the final endpoint, show the last source frame without seeking past EOF. */
export function previewFrame(timeline: readonly PreviewCut[], requestedMs: number) {
  const duration = timeline.at(-1)?.endMs ?? 0
  const time = Math.max(
    0,
    Math.min(requestedMs, Math.max(0, duration - CLIP_DRAFT_PREVIEW.frameToleranceMs)),
  )
  const active = timeline.filter((c) => time >= c.startMs && time < c.endMs)
  const incoming = active.length === 2 ? active[1]! : undefined
  const into = incoming ? (time - incoming.startMs) / incoming.cut.transitionMs : 1
  const alphas = incoming
    ? transitionAlphas(transitionKind(incoming.cut.transitionMs), 1 - into)
    : [1]
  return active.map((item, index) => ({
    ...item,
    sourceMs: outputToSourceMs(item, time),
    opacity: alphas[index] ?? 1,
    audioGain: incoming ? (index === 0 ? 1 - into : into) : 1,
  }))
}

/** One cut on the server's frame timeline: the frames it contributes at the output rate, the
 *  output frame it starts on, and the frames and xfade its leading transition overlaps the cut
 *  before it with. */
export interface FrameCut {
  cut: ClipEditCut
  index: number
  frames: number
  startFrame: number
  overlap: number
  kind: TransitionKind
}
export interface FrameTimeline {
  fps: number
  cuts: FrameCut[]
  total: number
}

/** The server render's frame arithmetic (Go's cutTimeline): each cut's frames are its
 *  cumulative transformed output time rounded at the output rate (CDS-62), a transition
 *  overlaps a whole number of the previous cut's frames (CDS-36), and the clip is the frames
 *  left after the overlaps. A browser export walks these frames, so it shows the frames a
 *  server export of the same plan shows (CLIP-157). */
export function frameTimeline(plan: Pick<ClipEditPlan, 'cuts'>, fps: number): FrameTimeline {
  let elapsedMs = 0
  let previous = 0
  let total = 0
  const cuts = plan.cuts.map((cut, index): FrameCut => {
    elapsedMs += cutOutputMs(cut)
    const next = Math.round((elapsedMs * fps) / 1000)
    const frames = next - previous
    previous = next
    const overlap = index ? Math.trunc((cut.transitionMs * fps) / 1000) : 0
    const startFrame = index ? total - overlap : 0
    total = index ? total + frames - overlap : frames
    return {
      cut,
      index,
      frames,
      startFrame,
      overlap,
      kind: overlap ? transitionKind(cut.transitionMs) : '',
    }
  })
  return { fps, cuts, total }
}

/** One cut's frame in an output frame: the instant of its source it shows, its weight in the
 *  frame and the alpha that draws it at that weight. */
export interface FrameLayer {
  cut: ClipEditCut
  index: number
  sourceMs: number
  weight: number
  alpha: number
}

/** The cut or cuts one output frame is made of, outgoing first. A cut's own frame k shows k
 *  output frames of source time at its rate after its start, the frame the server's footage
 *  chain keeps there. */
export function frameLayers(timeline: FrameTimeline, frame: number): FrameLayer[] {
  if (frame < 0 || frame >= timeline.total) return []
  const layer = (item: FrameCut, weight: number, alpha: number): FrameLayer => ({
    cut: item.cut,
    index: item.index,
    sourceMs:
      item.cut.startMs + Math.round(((frame - item.startFrame) * cutRate(item.cut)) / timeline.fps),
    weight,
    alpha,
  })
  for (let i = timeline.cuts.length - 1; i >= 0; i--) {
    const item = timeline.cuts[i]!
    if (frame < item.startFrame) continue
    if (frame >= item.startFrame + item.frames) return []
    if (i > 0 && frame < item.startFrame + item.overlap) {
      const progress = 1 - (frame - item.startFrame) / item.overlap
      const [outgoing, incoming] = transitionWeights(item.kind, progress)
      const [outgoingAlpha, incomingAlpha] = transitionAlphas(item.kind, progress)
      return [
        layer(timeline.cuts[i - 1]!, outgoing, outgoingAlpha),
        layer(item, incoming, incomingAlpha),
      ]
    }
    return [layer(item, 1, 1)]
  }
  return []
}

/** The flow simulation's instant (CLIP-175): the playhead, with the scrubber's end drawn as the
 *  clip's last millisecond rather than the empty instant after it. */
export function flowInstant(timeline: readonly PreviewCut[], timeMs: number) {
  const duration = timeline.at(-1)?.endMs ?? 0
  return Math.max(0, Math.min(timeMs, duration - 1))
}
/** The cut the flow simulation stands at the playhead (CLIP-173): the one whose interval holds
 *  it, the incoming one where two overlap in a transition — a still has no fade. */
export function flowCut(timeline: readonly PreviewCut[], timeMs: number) {
  const time = flowInstant(timeline, timeMs)
  return timeline.filter((c) => time >= c.startMs && time < c.endMs).at(-1)
}
/** The overlay the flow simulation shows at the playhead (CLIP-176): every asset whose interval
 *  holds it, drawn whole and at rest — no fade or settle; a sequence-drawn style is its one
 *  representative frame. */
export function flowAssets<T extends Pick<PreviewAsset, 'startMs' | 'endMs'>>(
  assets: readonly T[],
  timeline: readonly PreviewCut[],
  timeMs: number,
): T[] {
  const time = flowInstant(timeline, timeMs)
  return assets.filter((asset) => asset.startMs <= time && time < asset.endMs)
}

export function previewElementIDs(
  plan: ClipEditPlan,
  timeline: readonly PreviewCut[],
  timeMs: number,
) {
  const current = previewFrame(timeline, timeMs)
  const cutIds = new Set(current.map((v) => v.cut.id))
  const next = timeline[(current.at(-1)?.index ?? 0) + 1]
  if (next) cutIds.add(next.cut.id)
  return (plan.elements ?? [])
    .filter((t) => !t.cutId || cutIds.has(t.cutId))
    .map((t) => t.instanceId)
}

/** The export crop places the focal point at canvas centre then clamps its
 * rectangle to the source. CSS object-position percentages are not equivalent. */
export function previewCrop(
  sourceWidth: number,
  sourceHeight: number,
  width: number,
  height: number,
  focal = { x: 0.5, y: 0.5 },
) {
  const scale = Math.max(width / sourceWidth, height / sourceHeight)
  const w = sourceWidth * scale,
    h = sourceHeight * scale
  return {
    width: (w / width) * 100,
    height: (h / height) * 100,
    left: (-Math.max(0, Math.min(w - width, focal.x * w - width / 2)) / width) * 100,
    top: (-Math.max(0, Math.min(h - height, focal.y * h - height / 2)) / height) * 100,
  }
}

export interface PreviewAsset {
  key: string
  instanceId: string
  png: Uint8Array
  x: number
  y: number
  width: number
  height: number
  startMs: number
  endMs: number
  inMs: number
  outMs: number
  dy: number
  layer: number
  representativeFrame?: boolean
}
export interface PreviewPage {
  draftHash: string
  canvasWidth: number
  canvasHeight: number
  assets: PreviewAsset[]
  nextOffset: number
  parity: readonly string[]
}
export function previewMotion(
  asset: Pick<PreviewAsset, 'startMs' | 'endMs' | 'inMs' | 'outMs' | 'dy'>,
  timeMs: number,
) {
  if (timeMs < asset.startMs || timeMs >= asset.endMs) return { opacity: 0, dy: 0 }
  const into = asset.inMs ? Math.min(1, Math.max(0, (timeMs - asset.startMs) / asset.inMs)) : 1
  const out = asset.outMs ? Math.min(1, Math.max(0, (asset.endMs - timeMs) / asset.outMs)) : 1
  return { opacity: Math.min(into, out), dy: asset.dy * (1 - into) ** 3 }
}

/** What a rendered draft preview needs about its prepared overlay: the assets positioned in
 *  canvas pixels, whether they belong to the plan being shown, and the way back from a failure.
 *  `features/preview-clip-draft` produces it; this entity's ui only draws it. */
export interface ClipPreviewOverlay {
  assets: readonly PreparedAsset[]
  canvasWidth: number
  canvasHeight: number
  /** The overlay matches the plan on screen — until it does, the caller shows the old frame. */
  ready: boolean
  updating: boolean
  /** Set only while the preparation for the shown plan failed. */
  failure?: AppFailure
  onRetry: () => void
}

export interface PreviewSourceAccess {
  resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>
}
