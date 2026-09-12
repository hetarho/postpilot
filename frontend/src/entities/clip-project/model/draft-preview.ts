import { CLIP_DRAFT_PREVIEW, CLIP_TRANSITION } from '@/shared/config'
import type { ClipEditCut, ClipEditPlan } from './edit-plan'

export interface PreviewCut {
  cut: ClipEditCut
  index: number
  startMs: number
  endMs: number
}
export function previewTimeline(plan: ClipEditPlan): PreviewCut[] {
  if (
    plan.cuts.some(
      (c, index) =>
        !Number.isSafeInteger(c.startMs) ||
        !Number.isSafeInteger(c.endMs) ||
        c.startMs < 0 ||
        c.endMs - c.startMs <=
          Math.max(
            2 * CLIP_TRANSITION.fade_ms,
            c.transitionMs + (plan.cuts[index + 1]?.transitionMs ?? 0),
          ) ||
        ![0, CLIP_TRANSITION.fade_ms, CLIP_TRANSITION.black_ms].includes(c.transitionMs),
    )
  )
    return []
  let end = 0
  return plan.cuts.map((cut, index) => {
    const startMs = end - (index === 0 ? 0 : cut.transitionMs)
    end = startMs + cut.endMs - cut.startMs
    return { cut, index, startMs, endMs: end }
  })
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
  return active.map((item, index) => {
    const incoming = active.length === 2 ? active[1]! : undefined
    const progress = incoming ? (time - incoming.startMs) / incoming.cut.transitionMs : 1
    const black = incoming?.cut.transitionMs === CLIP_TRANSITION.black_ms
    const opacity = incoming
      ? black
        ? index === 0
          ? Math.max(0, 1 - progress * 2)
          : Math.max(0, progress * 2 - 1)
        : index === 0
          ? 1
          : progress
      : 1
    return {
      ...item,
      sourceMs: item.cut.startMs + time - item.startMs,
      opacity,
      audioGain: incoming ? (index === 0 ? 1 - progress : progress) : 1,
    }
  })
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

export interface PreviewSourceAccess {
  resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>
}
