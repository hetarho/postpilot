import {
  CLIP_DESIGN,
  CLIP_CAPTION_EFFECT_PAINT as PAINT,
  type ClipRatioId,
} from '@/entities/clip-design/@x/clip-preview'
import { document as inkDocument } from './ink-static'
import { inkCaptionFonts, inkStrokeWidth, type InkCaptionLayout } from './ink-layout'
import { inkNumber, inkTextMarkup, type InkBox } from './ink-typography'
import {
  inkEntrance,
  inkRound,
  inkTranslation,
  type InkCaptionScene,
  type InkCaptionFlame,
  type InkCaptionSpark,
} from './ink-caption-scene'

const cubic = (a: number, b: number, c: number, d: number, t: number) =>
  a * (1 - t) ** 3 + 3 * b * (1 - t) ** 2 * t + 3 * c * (1 - t) * t ** 2 + d * t ** 3

/** SVG objectBoundingBox uses actual curve extrema, not its control polygon. */
function cubicExtent(a: number, b: number, c: number, d: number) {
  const aa = -a + 3 * b - 3 * c + d,
    bb = 2 * (a - 2 * b + c),
    cc = b - a,
    discriminant = bb * bb - 4 * aa * cc
  const roots =
    Math.abs(aa) < 1e-12
      ? Math.abs(bb) < 1e-12
        ? []
        : [-cc / bb]
      : discriminant < 0
        ? []
        : [(-bb + Math.sqrt(discriminant)) / (2 * aa), (-bb - Math.sqrt(discriminant)) / (2 * aa)]
  const values = [a, d, ...roots.filter((t) => t > 0 && t < 1).map((t) => cubic(a, b, c, d, t))]
  return [Math.min(...values), Math.max(...values)] as const
}
export function inkEmberCurveBounds(points: readonly number[]): InkBox {
  const extent = (axis: number) => {
    const a = cubicExtent(points[axis]!, points[2 + axis]!, points[4 + axis]!, points[6 + axis]!),
      b = cubicExtent(points[6 + axis]!, points[8 + axis]!, points[10 + axis]!, points[12 + axis]!)
    return [Math.min(a[0], b[0]), Math.max(a[1], b[1])] as const
  }
  const x = extent(0),
    y = extent(1)
  return { x: x[0], y: y[0], width: x[1] - x[0], height: y[1] - y[0] }
}

export function inkEmberTongue(
  px: number,
  base: number,
  height: number,
  width: number,
  phase: number,
  progress: number,
): InkCaptionFlame {
  const h = height * (1 + 0.3 * Math.sin(progress * 2 * Math.PI * 2.4 + phase)),
    w = width * (1 + 0.16 * Math.cos(progress * 2 * Math.PI * 1.5 + phase)),
    lean = Math.sin(progress * 2 * Math.PI * 1.8 + phase) * h * 0.17
  // Match each native SVG number independently, including rounded endpoints.
  const points = [
    px,
    base,
    px - w,
    base - h * 0.32,
    px - w * 0.5 + lean,
    base - h * 0.7,
    px + lean,
    base - h,
    px + w * 0.6 + lean,
    base - h * 0.68,
    px + w,
    base - h * 0.3,
    px,
    base,
  ].map(inkRound)
  return { points, bounds: inkEmberCurveBounds(points) }
}
export function inkEmberGeometry(box: InkBox, progress: number) {
  const mid = box.y + box.height * 0.58,
    n = Math.floor(Math.max(9, box.width / 88)),
    nf = Math.floor((n * 8) / 5)
  const back = Array.from({ length: n }, (_, i) =>
      inkEmberTongue(
        box.x - 16 + ((box.width + 32) * (i + 0.5)) / n,
        mid + box.height * 0.26,
        210 + ((i * 47) % 110),
        44 + ((i * 13) % 24),
        i * 0.9,
        progress,
      ),
    ),
    front = Array.from({ length: nf }, (_, i) =>
      inkEmberTongue(
        box.x - 22 + ((box.width + 44) * (i + 0.5)) / nf,
        mid,
        105 + ((i * 37) % 95),
        19 + ((i * 7) % 15),
        i * 0.6,
        progress,
      ),
    )
  const sparks: InkCaptionSpark[] = Array.from({ length: 16 }, (_, i) => {
    const life = (progress * 1.3 + ((i * 0.37) % 1)) % 1,
      radius = 4.6 * (1 - life) * (0.5 + ((i * 0.21) % 0.6))
    return {
      index: i,
      cx: inkRound(box.x + ((i * 0.6180339887) % 1) * box.width + Math.sin(life * 7 + i) * 20),
      cy: inkRound(box.y - 30 - life * 250),
      radius: radius <= 0.6 ? 0 : inkRound(radius),
      alpha: radius <= 0.6 ? 0 : inkRound((1 - life) * 0.85),
    }
  })
  return { back, front, sparks }
}
export function inkEmberFilterBounds(flames: readonly InkCaptionFlame[]): InkBox {
  const x = Math.min(...flames.map((f) => f.bounds.x)),
    y = Math.min(...flames.map((f) => f.bounds.y)),
    right = Math.max(...flames.map((f) => f.bounds.x + f.bounds.width)),
    bottom = Math.max(...flames.map((f) => f.bounds.y + f.bounds.height)),
    width = right - x,
    height = bottom - y
  // Native cembsoft/cembsharp use SVG's default -10%/120% filter region.
  return { x: x - width * 0.1, y: y - height * 0.1, width: width * 1.2, height: height * 1.2 }
}

export function inkCaptionEmberScene(
  ratio: ClipRatioId,
  layout: InkCaptionLayout,
): InkCaptionScene {
  const { style, region, role, lines } = layout,
    fonts = inkCaptionFonts(layout),
    move = (p: number, d: number) => inkTranslation(0, inkEntrance(layout, p, d).dy),
    stroke = `stroke="${style.paint.stroke}" stroke-width="${inkNumber(inkStrokeWidth(style.rule.stroke))}" stroke-linejoin="round" paint-order="stroke"`
  const doc = inkDocument(
    ratio,
    'caption/ember/text-glow',
    region,
    110,
    fonts,
    (o) =>
      `<defs><filter id="glow" x="-60%" y="-70%" width="220%" height="250%" color-interpolation-filters="sRGB"><feGaussianBlur in="SourceAlpha" stdDeviation="30" result="b0"/><feFlood flood-color="${PAINT.ember.wide}" flood-opacity="0.8"/><feComposite in2="b0" operator="in" result="g0"/><feGaussianBlur in="SourceAlpha" stdDeviation="9" result="b1"/><feFlood flood-color="${PAINT.ember.tight}" flood-opacity="0.8"/><feComposite in2="b1" operator="in" result="g1"/><feMerge><feMergeNode in="g0"/><feMergeNode in="g0"/><feMergeNode in="g1"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs><g filter="url(#glow)">${lines.map((line) => `<text x="${inkNumber(line.x - o.x)}" y="${inkNumber(line.y - o.y)}" xml:space="preserve" font-family="${fonts[0]!.family}" font-size="${inkNumber(role.size)}" font-weight="${role.weight}" letter-spacing="${inkNumber(role.tracking * role.size)}" fill="${style.paint.fill}" ${stroke}>${inkTextMarkup(line.text, role, true)}</text>`).join('')}</g>`,
  )
  const canvas = CLIP_DESIGN.ratios[ratio].canvas,
    x = Math.max(0, Math.floor(region.x - style.bleed)),
    y = Math.max(0, Math.floor(region.y - style.bleed))
  return {
    layout,
    opacity: (p, d) => inkEntrance(layout, p, d).opacity,
    bounds: {
      x,
      y,
      width: Math.min(canvas.width, Math.ceil(region.x + region.width + style.bleed)) - x,
      height: Math.min(canvas.height, Math.ceil(region.y + region.height + style.bleed)) - y,
    },
    nodes: [
      {
        id: 'back',
        flames: { sigma: 12, stops: PAINT.ember.stops },
        pose: (p, d) => ({
          opacity: 0.9,
          matrix: move(p, d),
          flames: inkEmberGeometry(region, p).back,
        }),
      },
      { id: 'text', document: doc, pose: (p, d) => ({ opacity: 1, matrix: move(p, d) }) },
      {
        id: 'front',
        flames: { sigma: 4.5, stops: PAINT.ember.stops },
        pose: (p, d) => ({
          opacity: 0.86,
          matrix: move(p, d),
          flames: inkEmberGeometry(region, p).front,
        }),
      },
      {
        id: 'sparks',
        sparks: { fill: PAINT.ember.spark },
        pose: (p, d) => ({
          opacity: 1,
          matrix: move(p, d),
          sparks: inkEmberGeometry(region, p).sparks,
        }),
      },
    ],
  }
}
