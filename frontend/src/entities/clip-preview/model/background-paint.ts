import { CLIP_DESIGN, type ClipRatioId } from '@/entities/clip-design/@x/clip-preview'
import type { BrowserBackgroundMeasurement } from './background-sampling'
import type { InkBox } from './ink-typography'
import type { BrowserLocalComponent } from './local-components'
import type { BrowserCompositionSnapshot, BrowserEvaluatedFrame } from './browser-composition'
import type { BrowserBackgroundEvidence } from './background-sampling'
import { previewMotion } from './draft-preview'
import { inkEntrance } from './ink-caption-scene'
import { ClipInkError } from './ink-typography'

export interface BrowserScrim {
  kind: 'linear' | 'radial'
  box: InkBox
  hex: string
  from: number
  to: number
  mid?: number
  midAt?: number
}
export function backgroundScrim(
  ratio: ClipRatioId,
  measurement: BrowserBackgroundMeasurement,
): BrowserScrim | undefined {
  if (!measurement.scrim || measurement.geometry.plate) return undefined
  const layout = CLIP_DESIGN.ratios[ratio],
    region = measurement.geometry.region
  const block = measurement.geometry.regionBlock
  if (block) {
    const bounds = block.layout.bounds,
      preset = block.layout.scrim
    if (preset === 'top')
      return {
        kind: 'linear',
        box: { x: 0, y: 0, width: layout.canvas.width, height: bounds.y + bounds.height + 150 },
        ...CLIP_DESIGN.scrim.top,
      }
    if (preset === 'bottom') {
      const y = bounds.y - 150
      return {
        kind: 'linear',
        box: { x: 0, y, width: layout.canvas.width, height: layout.canvas.height - y },
        ...CLIP_DESIGN.scrim.bottom,
      }
    }
    const p = CLIP_DESIGN.scrim.radial,
      height = bounds.height + 560
    return {
      kind: 'radial',
      box: {
        x: bounds.x + bounds.width / 2 - 680,
        y: bounds.y + bounds.height / 2 - height / 2,
        width: 1360,
        height,
      },
      hex: p.hex,
      from: p.from,
      to: p.to,
      mid: p.mid,
      midAt: p.mid_at,
    }
  }
  let anchor = measurement.geometry.anchor
  if (anchor === 'header' || anchor === 'auto')
    anchor = region.y + region.height / 2 > layout.canvas.height / 2 ? 'bottom' : 'top'
  if (!['top', 'upper_mid', 'bottom', 'lower_mid'].includes(anchor)) return undefined
  const edge = anchor === 'bottom' || anchor === 'lower_mid' ? 'bottom' : 'top'
  const band = edge === 'top' ? layout.scrim_top : layout.scrim_bottom
  return { kind: 'linear', box: { ...band }, ...CLIP_DESIGN.scrim[edge] }
}

/** Each component owns its scrim and ink in the same native overlay order. */
export function drawMeasuredBrowserComponents(
  context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
  snapshot: BrowserCompositionSnapshot,
  frame: BrowserEvaluatedFrame,
  resources: readonly BrowserLocalComponent[],
  evidence: BrowserBackgroundEvidence,
): void {
  if (
    evidence.localSnapshotFingerprint !== snapshot.snapshotFingerprint ||
    evidence.snapshotFingerprint !==
      (snapshot.authoritativeFingerprint ?? snapshot.snapshotFingerprint)
  )
    throw new ClipInkError('CLIP_SNAPSHOT_SUPERSEDED')
  for (const resource of resources) {
    const state = resource.component
    if (!snapshot.components.includes(state.component))
      throw new ClipInkError('CLIP_INK_SUPERSEDED')
    const measurement = evidence.measurements.find(
      (m) =>
        m.instanceId === state.component.instanceId && m.phraseIndex === (state.phraseIndex ?? 0),
    )
    if (
      measurement &&
      (!measurement.geometry.regionBlock || measurement.geometry.regionBlock.owner)
    ) {
      const scrim = backgroundScrim(snapshot.ratio, measurement)
      if (scrim) {
        let motion = previewMotion(
          {
            startMs: state.component.startMs,
            endMs: state.component.endMs,
            ...(measurement.geometry.motion ?? { inMs: 0, outMs: 0, dy: 0 }),
          },
          frame.timeMs,
        )
        if (resource.kind === 'scene' && measurement.geometry.caption) {
          const progress = state.component.element.pace === 'rapid' ? 0.5 : state.progress
          const entrance = inkEntrance(measurement.geometry.caption, progress, state.durationMs)
          motion = {
            opacity: resource.scene.opacity,
            dy: ['word-pop', 'blur-in', 'glitch', 'stack', 'pop', 'sticker'].includes(
              measurement.geometry.caption.style.id,
            )
              ? 0
              : entrance.dy,
          }
        }
        context.save()
        context.globalAlpha = motion.opacity
        context.translate(0, motion.dy)
        drawBackgroundScrim(context, scrim)
        context.restore()
      }
    }
    context.globalAlpha = 1
    resource.draw(context)
  }
}
export function scrimOpacityAt(scrim: BrowserScrim, x: number, y: number): number {
  const b = scrim.box
  if (scrim.kind === 'linear') {
    if (y < b.y || y > b.y + b.height) return 0
    return scrim.from + ((scrim.to - scrim.from) * (y - b.y)) / b.height
  }
  const distance = Math.hypot(
    (x - b.x - b.width / 2) / (b.width / 2),
    (y - b.y - b.height / 2) / (b.height / 2),
  )
  if (distance >= 1) return scrim.to
  const at = scrim.midAt!,
    mid = scrim.mid!
  return distance <= at
    ? scrim.from + ((mid - scrim.from) * distance) / at
    : mid + ((scrim.to - mid) * (distance - at)) / (1 - at)
}
function channels(hex: string): [number, number, number] {
  return [1, 3, 5].map((at) => parseInt(hex.slice(at, at + 2), 16) / 255) as [
    number,
    number,
    number,
  ]
}
function over(rgb: readonly number[], foreground: string, alpha: number): [number, number, number] {
  return channels(foreground).map(
    (v, c) => Math.round((v * alpha + rgb[c]! * (1 - alpha)) * 255) / 255,
  ) as [number, number, number]
}
function luma(rgb: readonly number[]) {
  const c = rgb.map((v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
  return 0.2126 * c[0]! + 0.7152 * c[1]! + 0.0722 * c[2]!
}
export function backgroundContrastNotices(
  ratio: ClipRatioId,
  measurement: BrowserBackgroundMeasurement,
): { code: string; action: string; elementId: string; cutId: string }[] {
  const scrim = backgroundScrim(ratio, measurement)
  const white = CLIP_DESIGN.color.text_white
  const stroke = CLIP_DESIGN.color.stroke_dark
  const parts = measurement.geometry.contrastParts ?? [
    { box: measurement.geometry.region, fill: white.hex, alpha: 1, stroke: true },
  ]
  for (const part of parts) {
    const box = part.box
    // Native Luminance.Hex rounds the temporal RGB mean before layering paint.
    let background = measurement.ground.rgb.map((v) => Math.round(v * 255) / 255) as [
      number,
      number,
      number,
    ]
    if (scrim)
      background = over(
        background,
        scrim.hex,
        scrimOpacityAt(scrim, box.x + box.width / 2, box.y + box.height / 2),
      )
    for (const shape of measurement.geometry.regionBlock?.layout.shapes ?? []) {
      const b = shape.box,
        x = box.x + box.width / 2,
        y = box.y + box.height / 2
      const under = shape.circle
        ? Math.hypot(x - b.x - b.width / 2, y - b.y - b.height / 2) <= b.width / 2
        : x >= b.x && x <= b.x + b.width && y >= b.y && y <= b.y + b.height
      if (under && shape.fill && shape.fillAlpha > 0)
        background = over(background, shape.fill, shape.fillAlpha)
    }
    if (part.stroke) background = over(background, stroke.hex, stroke.alpha)
    const foreground = over(background, part.fill, part.alpha)
    const a = luma(foreground),
      b = luma(background)
    if ((Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05) < 4.5)
      return [
        {
          code: 'composition_contrast',
          action: 'shortfall',
          elementId: measurement.elementId,
          cutId: measurement.cutId,
        },
      ]
  }
  return []
}
export function drawBackgroundScrim(
  context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
  scrim: BrowserScrim,
): void {
  const b = scrim.box
  context.save()
  const atAlpha = (a: number) => {
    const [r, g, blue] = channels(scrim.hex).map((v) => Math.round(v * 255))
    return `rgba(${r},${g},${blue},${a})` // style-escape: native CDS scrim token with its measured gradient alpha
  }
  if (scrim.kind === 'radial') {
    context.translate(b.x + b.width / 2, b.y + b.height / 2)
    context.scale(b.width / 2, b.height / 2)
    const gradient = context.createRadialGradient(0, 0, 0, 0, 0, 1)
    gradient.addColorStop(0, atAlpha(scrim.from))
    gradient.addColorStop(scrim.midAt!, atAlpha(scrim.mid!))
    gradient.addColorStop(1, atAlpha(scrim.to))
    context.fillStyle = gradient
    context.fillRect(-1, -1, 2, 2)
  } else {
    const gradient = context.createLinearGradient(0, b.y, 0, b.y + b.height)
    gradient.addColorStop(0, atAlpha(scrim.from))
    gradient.addColorStop(1, atAlpha(scrim.to))
    context.fillStyle = gradient
    context.fillRect(b.x, b.y, b.width, b.height)
  }
  context.restore()
}
