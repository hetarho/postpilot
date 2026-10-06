import {
  CLIP_DESIGN,
  CLIP_CAPTION_EFFECT_PAINT as PAINT,
  type ClipRatioId,
} from '@/entities/clip-design/@x/clip-preview'
import { document as inkDocument } from './ink-static'
import { inkCaptionFonts, inkStrokeWidth, type InkCaptionLayout } from './ink-layout'
import { inkNumber, inkTextMarkup, type InkBox } from './ink-typography'
import {
  inkClamp,
  inkCubic,
  inkEntrance,
  inkPivot,
  inkRound,
  inkTranslation,
  type InkCaptionScene,
  type InkCaptionSceneNode,
  type InkCaptionPose,
} from './ink-caption-scene'

const identity = inkTranslation(0, 0)
const pose = (opacity = 1, matrix = identity): InkCaptionPose => ({
  opacity: inkRound(opacity),
  matrix,
})

/** Only fixed local ink is rasterized. Every filter, band and light state is
 * reconstructed from the same native output progress and actual cue duration. */
export function inkCaptionEffectsScene(
  ratio: ClipRatioId,
  layout: InkCaptionLayout,
): InkCaptionScene {
  const { style, role, region, lines } = layout,
    fonts = inkCaptionFonts(layout)
  const doc = (id: string, box: InkBox, markup: (origin: { x: number; y: number }) => string) =>
    inkDocument(ratio, `caption/${style.id}/${id}`, box, style.bleed, fonts, markup)
  const text = (
    line: { text: string; x: number; y: number },
    origin: { x: number; y: number },
    attributes: string,
  ) =>
    `<text x="${inkNumber(line.x - origin.x)}" y="${inkNumber(line.y - origin.y)}" xml:space="preserve" font-family="${fonts[0]!.family}" font-size="${inkNumber(role.size)}" font-weight="${role.weight}" letter-spacing="${inkNumber(role.tracking * role.size)}" ${attributes}>${inkTextMarkup(line.text, role, true)}</text>`
  const opacity = (p: number, d: number) => inkEntrance(layout, p, d).opacity
  const move = (p: number, d: number) => inkTranslation(0, inkEntrance(layout, p, d).dy)
  const nodes: InkCaptionSceneNode[] = []
  if (style.id === 'ambient') {
    nodes.push({
      id: 'light',
      light: { palettes: [PAINT.ambient.a, PAINT.ambient.b], sigma: 26 },
      pose: (p, d) => {
        const breath = 0.86 + 0.14 * Math.sin(p * 2 * Math.PI * 0.9),
          cx = region.x + region.width / 2,
          cy = region.y + region.height / 2 - role.size * 0.25,
          rx = region.width * 0.62,
          ry = role.size * 1.15
        return {
          ...pose(1, move(p, d)),
          light: {
            ellipses: [
              {
                cx: inkRound(cx - region.width * 0.18),
                cy: inkRound(cy),
                rx: inkRound(rx * breath),
                ry: inkRound(ry * breath),
              },
              {
                cx: inkRound(cx + region.width * 0.22),
                cy: inkRound(cy + role.size * 0.18),
                rx: inkRound(rx * 0.72 * breath),
                ry: inkRound(ry * 0.86 * breath),
              },
            ],
          },
        }
      },
    })
    const s = style.paint.shadow
    nodes.push({
      id: 'text',
      document: doc(
        'text',
        region,
        (o) =>
          `<defs><filter id="shadow" x="-30%" y="-30%" width="160%" height="180%" color-interpolation-filters="sRGB"><feOffset in="SourceAlpha" dx="${inkNumber(s.dx)}" dy="${inkNumber(s.dy)}" result="o"/><feGaussianBlur in="o" stdDeviation="${inkNumber(s.blur / 2)}" result="b"/><feFlood flood-color="${s.hex}" flood-opacity="${inkNumber(s.alpha)}"/><feComposite in2="b" operator="in" result="s"/><feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs><g filter="url(#shadow)">${lines.map((line) => text(line, o, `fill="${style.paint.fill}"`)).join('')}</g>`,
      ),
      pose: (p, d) => pose(1, move(p, d)),
    })
  } else
    for (const [index, line] of lines.entries()) {
      const box = { x: line.x, y: line.top, width: line.width, height: line.height }
      if (style.id === 'blur-in') {
        nodes.push({
          id: `${index}`,
          document: doc(`${index}`, box, (o) => text(line, o, `fill="${style.paint.fill}"`)),
          pose: (p, d) => {
            const settled =
                d <= 0 || style.motion.inMs <= 0 ? 1 : inkClamp((p * d) / style.motion.inMs),
              e = inkCubic(settled),
              scale = 1.05 - 0.05 * e
            return {
              ...pose(
                1,
                inkPivot(region.x + region.width / 2, region.y + region.height / 2, scale),
              ),
              effect: { kind: 'blur', sigma: inkRound(Math.max(26 * (1 - e), 0.01)) },
            }
          },
        })
      } else if (style.id === 'neon') {
        nodes.push({
          id: `${index}`,
          document: doc(`${index}`, box, (o) => text(line, o, 'fill="white"')),
          pose: (p, d) => {
            let pulse = 0.8 + 0.2 * Math.sin(p * 2 * Math.PI * 3)
            if (p > 0.12 && Math.floor(p * 97) % 19 === 0) pulse *= 0.42
            return {
              ...pose(1, move(p, d)),
              tint: style.paint.fill,
              effect: {
                kind: 'neon',
                wideSigma: inkRound(26 * pulse),
                tightSigma: inkRound(8 * pulse),
                wideAlpha: inkRound(0.9 * pulse),
                tightAlpha: inkRound(0.95 * pulse),
                wide: PAINT.neon.wide,
                tight: PAINT.neon.tight,
              },
            }
          },
        })
      } else if (style.id === 'glitch') {
        const document = doc(`${index}`, box, (o) => text(line, o, 'fill="white"'))
        const state = (p: number) => {
          const step = Math.floor(p * 900),
            burst = p < 0.14 || step % 5 < 2 ? 1 : 0.5
          return {
            step,
            burst,
            dx: (7 + ((step * 7) % 12)) * burst * (step % 2 === 1 ? -1 : 1),
            dy: (((step * 3) % 6) - 3) * burst,
          }
        }
        for (const [color, sign] of [
          [PAINT.glitch.red, -1],
          [PAINT.glitch.cyan, 1],
        ] as const)
          nodes.push({
            id: `${index}/${sign}`,
            document,
            pose: (p) => {
              const s = state(p)
              return { ...pose(0.9, inkTranslation(sign * s.dx, -sign * s.dy)), tint: color }
            },
          })
        for (let band = 0; band < 6; band++)
          nodes.push({
            id: `${index}/band/${band}`,
            document,
            pose: (p) => {
              const s = state(p),
                shift =
                  (s.step + band * 13 + index * 7) % 3 === 0
                    ? 0
                    : (((s.step * 11 + band * 29) % 72) - 36) * s.burst
              return {
                ...pose(1, inkTranslation(shift, 0)),
                tint: style.paint.fill,
                clip: {
                  x: -inkRound(shift),
                  y: inkRound(line.top + (band * line.height) / 6),
                  width: CLIP_DESIGN.ratios[ratio].canvas.width,
                  height: inkRound(line.height / 6 + 1),
                },
              }
            },
          })
      } else if (style.id === 'iridescent') {
        const stroke = `stroke="${style.paint.stroke}" stroke-width="${inkNumber(inkStrokeWidth(style.rule.stroke))}" stroke-linejoin="round" stroke-opacity="0.55" paint-order="stroke"`
        // One fixed group SourceAlpha halo for all lines, exactly as native.
        if (index === 0)
          nodes.push({
            id: 'glow',
            document: doc(
              'glow',
              region,
              (o) =>
                `<defs><filter id="glow" x="-30%" y="-30%" width="160%" height="170%" color-interpolation-filters="sRGB"><feGaussianBlur in="SourceAlpha" stdDeviation="16" result="b"/><feFlood flood-color="${PAINT.iridescent.glow}" flood-opacity="0.55"/><feComposite in2="b" operator="in"/></filter></defs><g filter="url(#glow)">${lines.map((row) => text(row, o, `fill="white" ${stroke}`)).join('')}</g>`,
            ),
            pose: (p, d) => pose(1, move(p, d)),
          })
        nodes.push({
          id: `${index}/stroke`,
          document: doc(`${index}/stroke`, box, (o) => text(line, o, `fill="none" ${stroke}`)),
          pose: (p, d) => pose(1, move(p, d)),
        })
        nodes.push({
          id: `${index}/fill`,
          document: doc(`${index}/fill`, box, (o) => text(line, o, 'fill="white"')),
          pose: (p, d) => ({
            ...pose(1, move(p, d)),
            effect: {
              kind: 'gradient',
              box,
              stops: PAINT.iridescent.stops.map((stop) => ({
                at: inkRound(inkClamp(stop.at + ((p * 0.9) % 1))),
                hex: stop.hex,
              })),
            },
          }),
        })
      } else throw new Error(`CLIP_INK_STYLE_NOT_IMPLEMENTED:${style.id}`)
    }
  if (style.id === 'glitch')
    nodes.sort((a, b) => Number(a.id.includes('/band/')) - Number(b.id.includes('/band/')))
  const canvas = CLIP_DESIGN.ratios[ratio].canvas
  let left = region.x - style.bleed,
    top = region.y - style.bleed,
    right = region.x + region.width + style.bleed,
    bottom = region.y + region.height + style.bleed
  if (style.id === 'ambient') {
    // The full ellipse union is composed before Gaussian26. Reserve its maximum
    // breathing extent plus three sigma, even when it extends beyond text bleed.
    const cx = region.x + region.width / 2,
      cy = region.y + region.height / 2 - role.size * 0.25,
      rx = region.width * 0.62,
      ry = role.size * 1.15
    const ellipses = [
      { cx: cx - region.width * 0.18, cy, rx, ry },
      { cx: cx + region.width * 0.22, cy: cy + role.size * 0.18, rx: rx * 0.72, ry: ry * 0.86 },
    ]
    for (const e of ellipses) {
      left = Math.min(left, e.cx - e.rx - 78)
      top = Math.min(top, e.cy - e.ry - 78)
      right = Math.max(right, e.cx + e.rx + 78)
      bottom = Math.max(bottom, e.cy + e.ry + 78 + style.motion.dy)
    }
  }
  if (style.id === 'blur-in') {
    const extra = Math.max(region.width, region.height) * 0.025 + 3
    left -= extra
    top -= extra
    right += extra
    bottom += extra
  }
  const x = Math.max(0, Math.floor(left)),
    y = Math.max(0, Math.floor(top))
  return {
    layout,
    nodes,
    opacity,
    bounds: {
      x,
      y,
      width: Math.min(canvas.width, Math.ceil(right)) - x,
      height: Math.min(canvas.height, Math.ceil(bottom)) - y,
    },
  }
}
