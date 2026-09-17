import { useId, useLayoutEffect, useRef, useState } from 'react'
import {
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_DESIGN,
  CLIP_REGIONS,
  CLIP_RULES,
  type ClipRatioId,
  type ClipRegionPresets,
} from '@/shared/config'
import type { ClipComposition, ResolvedCompositionElement } from '../model/composition'

type TypeName = keyof typeof CLIP_DESIGN.type
type Slot = {
  type: string
  y: number
  fill: string
  alpha?: number
  tracking?: number
  stroke: string
  shadow: string
}
type Metric = { x: number; y: number; width: number; height: number }
type Line = {
  key: string
  text: string
  type: TypeName
  size: number
  tracking: number
  alpha: number
  stroke: number
  shadow: boolean
  slot?: number
}
type Visual = {
  entry: ResolvedCompositionElement
  lines: Line[]
  kind?: 'intro' | 'outro'
  slots?: readonly Slot[]
  rules?: readonly { kind: string; y: number }[]
}

/** Text is measured in the browser with the same bundled faces as export. The
 * first paint reserves nominal geometry; font readiness replaces it with ink bounds. */
function useInkBounds(signature: string) {
  const ref = useRef<SVGSVGElement>(null)
  const [bounds, setBounds] = useState<Record<string, Metric>>({})
  useLayoutEffect(() => {
    let cancelled = false
    const measure = () => {
      if (cancelled) return
      const next: Record<string, Metric> = {}
      for (const text of ref.current?.querySelectorAll<SVGTextElement>('[data-measure]') ?? []) {
        if (!text.getBBox) continue
        // SVG getBBox includes a font's ascent/descent cell in Chromium. Canvas
        // actualBoundingBox measures the painted glyphs, matching resvg's ink box.
        const context =
          typeof CanvasRenderingContext2D === 'undefined'
            ? null
            : window.document.createElement('canvas').getContext('2d')
        if (context) {
          const font = window.getComputedStyle(text)
          context.font = `${font.fontWeight} ${font.fontSize} ${font.fontFamily}`
          context.letterSpacing = font.letterSpacing
          const box = context.measureText(text.textContent ?? '')
          next[text.dataset.measure!] = {
            x: -box.actualBoundingBoxLeft,
            y: -box.actualBoundingBoxAscent,
            width: box.actualBoundingBoxLeft + box.actualBoundingBoxRight,
            height: box.actualBoundingBoxAscent + box.actualBoundingBoxDescent,
          }
        } else {
          const box = text.getBBox()
          next[text.dataset.measure!] = { x: box.x, y: box.y, width: box.width, height: box.height }
        }
      }
      setBounds((old) => (JSON.stringify(old) === JSON.stringify(next) ? old : next))
    }
    measure()
    void document.fonts?.ready.then(measure)
    return () => {
      cancelled = true
    }
  }, [signature])
  return { ref, bounds }
}

function typeLine(
  key: string,
  text: string,
  type: TypeName,
  ratio: ClipRatioId,
  options: Partial<Line> = {},
): Line {
  const role = CLIP_DESIGN.type[type]
  return {
    key,
    text,
    type,
    size: type === 'hook' ? CLIP_DESIGN.ratios[ratio].hook_size : role.size,
    tracking: role.tracking,
    alpha: 1,
    stroke: 0,
    shadow: false,
    ...options,
  }
}

export function CompositionDesignFrame({
  document,
  entries,
  ratio,
  label,
  sampleAI,
  presets = CLIP_DEFAULT_REGION_PRESETS,
}: {
  document: ClipComposition
  entries: ResolvedCompositionElement[]
  ratio: ClipRatioId
  label: string
  sampleAI: string
  /** The presets the intro and the outro are drawn in. A template declares
   *  none, so a surface with no project selection draws the shared defaults
   *  (CLIP-14, CLIP-147). */
  presets?: ClipRegionPresets
}) {
  const shape = CLIP_DESIGN.ratios[ratio]
  const scale = shape.canvas.height / CLIP_DESIGN.ratios.vertical.canvas.height
  /** CDS-79: a region block keeps its 9:16 spacing on every ratio and moves as
   * one piece — its centre, the midpoint of the preset's own y values, lands on
   * the same fraction of canvas height. The renderer resolves every region y
   * through the same offset, so the preview cannot drift from the export. */
  const regionOffset = (v: Visual) => {
    const ys = [...(v.slots ?? []).map((s) => s.y), ...(v.rules ?? []).map((r) => r.y)]
    if (!ys.length) return 0
    const centre = (Math.min(...ys) + Math.max(...ys)) / 2
    return centre * scale - centre
  }
  const shadowID = useId()
  const visuals: Visual[] = entries.map((entry) => {
    const e = entry.element
    const kind = e.role === 'hook' ? 'intro' : e.role === 'ending' ? 'outro' : undefined
    const preset =
      kind === 'intro'
        ? CLIP_REGIONS.intro[presets.intro]
        : kind === 'outro'
          ? CLIP_REGIONS.outro[presets.outro]
          : undefined
    const rows = entry.rows.length
      ? entry.rows
      : [{ role: e.role === 'info' ? 'caption' : '', text: entry.text }]
    const lines = rows
      .map((row, i) => {
        const text = (e.rows[i]?.kind ?? e.kind) === 'ai' ? sampleAI : row.text
        const slot: Slot | undefined = preset?.slots[i]
        const type = (slot?.type ??
          (e.role === 'badge'
            ? 'badge'
            : e.role === 'info'
              ? row.role || 'caption'
              : 'title')) as TypeName
        const colour =
          CLIP_DESIGN.color[
            slot?.fill === 'text_muted' || (!kind && type === 'label') ? 'text_muted' : 'text_white'
          ]
        return typeLine(`${entry.instanceId}/${i}`, text, type, ratio, {
          alpha: colour.alpha * (slot?.alpha ?? 1),
          tracking:
            slot?.tracking ??
            (e.role === 'info' && type === 'label'
              ? CLIP_DESIGN.information.label_tracking
              : CLIP_DESIGN.type[type].tracking),
          stroke: kind
            ? slot?.stroke === 'text'
              ? CLIP_DESIGN.spacing.stroke_text
              : slot?.stroke === 'small'
                ? CLIP_DESIGN.spacing.stroke_small
                : 0
            : e.role === 'badge'
              ? 0
              : e.role === 'info'
                ? CLIP_DESIGN.spacing.stroke_small
                : CLIP_DESIGN.spacing.stroke_text,
          shadow: kind ? !!slot?.shadow : e.role !== 'badge',
          slot: kind ? i : undefined,
        })
      })
      // A line past the last slot of the chosen preset is drawn by nobody, here
      // as in the render (CLIP-147).
      .filter(
        (line) => line.text.trim() !== '' && (!kind || line.slot! < (preset?.slots.length ?? 0)),
      )
    return { entry, lines, kind, slots: preset?.slots, rules: preset?.rules }
  })
  const lines = visuals.flatMap((v) => v.lines)
  const { ref, bounds } = useInkBounds(JSON.stringify(lines))
  const ink = (line: Line) =>
    bounds[line.key] ?? {
      x: 0,
      y: -line.size,
      width: (Array.from(line.text).length * line.size) / 2,
      height: line.size,
    }
  const props = (line: Line) => ({
    fontFamily:
      CLIP_DESIGN.faces[CLIP_DESIGN.type[line.type].face as keyof typeof CLIP_DESIGN.faces],
    fontSize: line.size,
    fontWeight: CLIP_DESIGN.type[line.type].weight,
    letterSpacing: line.tracking * line.size,
    xmlSpace: 'preserve' as const,
  })
  const size = (v: Visual) => {
    const width = Math.ceil(Math.max(0, ...v.lines.map((line) => ink(line).width)))
    if (v.entry.element.role === 'caption') {
      const stroke = CLIP_DESIGN.spacing.stroke_text
      return {
        width: width + stroke,
        height: v.lines.reduce(
          (height, line, i) =>
            height +
            ink(line).height +
            (i ? line.size * (CLIP_DESIGN.type[line.type].line_height - 1) : 0),
          stroke,
        ),
      }
    }
    if (v.entry.element.role === 'badge')
      return {
        width: width + 2 * CLIP_DESIGN.spacing.pad_chip.h,
        height: CLIP_DESIGN.type.badge.size + 2 * CLIP_DESIGN.spacing.pad_chip.v,
      }
    return {
      width,
      height: v.lines.reduce(
        (sum, line, i) =>
          sum + Math.max(line.size, ink(line).height) + (i ? CLIP_DESIGN.spacing.gap_stack : 0),
        0,
      ),
    }
  }
  const isHeader = (v: Visual) =>
    ['badge', 'info'].includes(v.entry.element.role) &&
    ['auto', 'header'].includes(v.entry.element.position)
  const header = visuals.filter(isHeader)
  const headerHeight = Math.max(0, ...header.map((v) => size(v).height))
  const badge = header.find((v) => v.entry.element.role === 'badge')
  const right = badge
    ? shape.badge.right - size(badge).width - CLIP_DESIGN.spacing.gap_stack
    : shape.badge.right
  let headerX: number = shape.anchor.left,
    headerY: number = shape.badge.top,
    headerCount = 0
  const positions = new Map<string, { x: number; y: number }>()
  for (const v of header.filter((v) => v.entry.element.role === 'info')) {
    const box = size(v)
    if (
      headerCount >= 2 ||
      headerX + box.width > (headerY === shape.badge.top ? right : shape.badge.right)
    ) {
      headerX = shape.anchor.left
      headerY += headerHeight + CLIP_DESIGN.spacing.gap_stack
      headerCount = 0
    }
    positions.set(v.entry.instanceId, { x: headerX, y: headerY + (headerHeight - box.height) / 2 })
    headerX += box.width + CLIP_DESIGN.spacing.gap_stack
    headerCount++
  }
  const occupiedRegions = visuals
    .filter((v) => v.kind && v.lines.length)
    .flatMap((v) => [
      ...v.lines.map((line) => ({
        x: shape.anchor.center - ink(line).width / 2,
        y: v.slots![line.slot!].y + regionOffset(v) + ink(line).y,
        width: ink(line).width,
        height: ink(line).height,
      })),
      ...(v.rules ?? []).map((line) => {
        const rule = CLIP_RULES[line.kind as keyof typeof CLIP_RULES]
        return {
          x: shape.anchor.center - rule.w / 2,
          y: line.y + regionOffset(v),
          width: rule.w,
          height: rule.h,
        }
      }),
    ])
  const captionAnchor = (v: Visual) => {
    const box = size(v),
      x = shape.anchor.center - box.width / 2
    // No footage subject exists in this representative frame. Use the renderer's
    // default/alternative order and reserve the visible preset parts first.
    return (
      [CLIP_REGIONS.caption.bold.anchor, CLIP_REGIONS.caption.bold.anchor_alt].find((anchor) => {
        const y = shape.anchor[anchor as keyof typeof shape.anchor] - box.height / 2
        return !occupiedRegions.some(
          (r) =>
            x < r.x + r.width && x + box.width > r.x && y < r.y + r.height && y + box.height > r.y,
        )
      }) ?? CLIP_REGIONS.caption.bold.anchor
    )
  }
  const paintedText = (line: Line, center: number, y: number, accented = false) => (
    <text
      key={line.key}
      {...props(line)}
      x={center - ink(line).x - ink(line).width / 2}
      y={y}
      data-slot={line.slot === undefined ? undefined : line.slot + 1}
      fill={CLIP_DESIGN.color.text_white.hex}
      fillOpacity={line.alpha}
      stroke={line.stroke ? CLIP_DESIGN.color.stroke_dark.hex : 'none'}
      strokeOpacity={CLIP_DESIGN.color.stroke_dark.alpha}
      strokeWidth={line.stroke}
      strokeLinejoin="round"
      paintOrder="stroke fill"
      filter={line.shadow ? `url(#${shadowID})` : undefined}
    >
      {accented && document.accent ? (
        <>
          <tspan fill={CLIP_DESIGN.accent[document.accent as keyof typeof CLIP_DESIGN.accent]}>
            {line.text.split(/\s/)[0]}
          </tspan>
          {line.text.slice(line.text.split(/\s/)[0].length)}
        </>
      ) : (
        line.text
      )}
    </text>
  )
  return (
    <svg
      ref={ref}
      viewBox={`0 0 ${shape.canvas.width} ${shape.canvas.height}`}
      className="mx-auto max-h-80 w-full"
      role="img"
      aria-label={label}
    >
      <defs>
        <filter id={shadowID} x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow
            dx={CLIP_DESIGN.shadow.text.dx}
            dy={CLIP_DESIGN.shadow.text.dy}
            stdDeviation={CLIP_DESIGN.shadow.text.blur / 2}
            floodColor={CLIP_DESIGN.shadow.text.hex}
            floodOpacity={CLIP_DESIGN.shadow.text.alpha}
          />
        </filter>
      </defs>
      <rect
        width={shape.canvas.width}
        height={shape.canvas.height}
        fill={CLIP_DESIGN.color.stroke_dark.hex}
      />
      <g visibility="hidden" aria-hidden="true">
        {lines.map((line) => (
          <text key={line.key} data-measure={line.key} x={0} y={0} {...props(line)}>
            {line.text}
          </text>
        ))}
      </g>
      {visuals.map((v) => {
        if (!v.lines.length) return null
        const e = v.entry.element
        if (v.kind)
          return (
            <g
              key={v.entry.instanceId}
              data-element={e.id}
              data-region={v.kind}
              data-preset={presets[v.kind]}
            >
              {v.lines.map((line) =>
                paintedText(line, shape.anchor.center, v.slots![line.slot!].y + regionOffset(v)),
              )}
              {v.rules?.map((r, i) => {
                const rule = CLIP_RULES[r.kind as keyof typeof CLIP_RULES]
                return (
                  <rect
                    key={i}
                    data-rule={r.kind}
                    x={shape.anchor.center - rule.w / 2}
                    y={r.y + regionOffset(v)}
                    width={rule.w}
                    height={rule.h}
                    fill={CLIP_DESIGN.color.text_white.hex}
                    fillOpacity={rule.alpha}
                  />
                )
              })}
            </g>
          )
        const box = size(v)
        const pos =
          e.position === 'auto' ? (e.role === 'caption' ? captionAnchor(v) : 'header') : e.position
        const anchor = shape.anchor[pos as keyof typeof shape.anchor] ?? shape.anchor.upper_mid
        let x = shape.anchor.center - box.width / 2
        let y =
          pos === 'top' ? anchor : pos === 'bottom' ? anchor - box.height : anchor - box.height / 2
        if (isHeader(v)) {
          const at = positions.get(v.entry.instanceId)
          x = at?.x ?? shape.badge.right - box.width
          y = at?.y ?? shape.badge.top + (headerHeight - box.height) / 2
        } else if (e.role === 'caption' || e.role === 'badge') {
          if (e.align === 'left') x = shape.anchor.left
          if (e.align === 'right') x = shape.anchor.right - box.width
        }
        let top = y + (e.role === 'caption' ? CLIP_DESIGN.spacing.stroke_text / 2 : 0)
        return (
          <g key={v.entry.instanceId} data-element={e.id} data-role={e.role} data-position={pos}>
            {e.role === 'badge' && (
              <rect
                data-disclosure="true"
                x={x}
                y={y}
                width={box.width}
                height={box.height}
                rx={CLIP_DESIGN.spacing.radius_chip}
                fill={CLIP_DESIGN.color.badge_ad.hex}
                fillOpacity={CLIP_DESIGN.color.badge_ad.alpha}
              />
            )}
            {v.lines.map((line) => {
              const y =
                e.role === 'badge'
                  ? top + (box.height - ink(line).height) / 2 - ink(line).y
                  : top - ink(line).y
              top +=
                e.role === 'caption'
                  ? ink(line).height + line.size * (CLIP_DESIGN.type[line.type].line_height - 1)
                  : Math.max(line.size, ink(line).height) + CLIP_DESIGN.spacing.gap_stack
              return paintedText(line, x + box.width / 2, y, e.role === 'caption')
            })}
          </g>
        )
      })}
    </svg>
  )
}
