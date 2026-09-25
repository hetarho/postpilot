import { useId, useLayoutEffect, useRef, useState } from 'react'
import {
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_DESIGN,
  CLIP_REGIONS,
  clipLayoutRegion,
  clipRegionSlots,
} from '@/entities/clip-design/@x/clip-template'
import {
  type ClipRatioId,
  type ClipRegionLayout,
  type ClipRegionPresets,
} from '@/entities/clip-design/@x/clip-template'
import type { ClipComposition, ResolvedCompositionElement } from '../model/composition'

type TypeName = keyof typeof CLIP_DESIGN.type
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
  /** A region line's face, weight and baseline come from the block layout. */
  face?: string
  weight?: number
  baseline?: number
}
type Visual = {
  entry: ResolvedCompositionElement
  lines: Line[]
  kind?: 'intro' | 'outro'
  /** Whether this entry paints its region's rules: the first one with a line. */
  rules?: boolean
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
  const shadowID = useId()
  const kindOf = (e: ResolvedCompositionElement['element']) =>
    e.role === 'hook' ? 'intro' : e.role === 'ending' ? 'outro' : undefined
  const shown = (entry: ResolvedCompositionElement) => {
    const e = entry.element
    const rows = entry.rows.length
      ? entry.rows
      : [{ role: e.role === 'info' ? 'caption' : '', text: entry.text }]
    return rows.map((row, i) => ({
      role: row.role,
      text: (e.rows[i]?.kind ?? e.kind) === 'ai' ? sampleAI : row.text,
    }))
  }
  /** CLIP-147 and CDS-87: a region's entries fill its slots in order, and the block lays
   *  out once from all of their lines, exactly as the renderer lays it out. */
  const placement = new Map<string, { offset: number; drawn: number; rules: boolean }>()
  const regionRows: Record<'intro' | 'outro', string[]> = {
    intro: clipRegionSlots('intro', presets.intro).map(() => ''),
    outro: clipRegionSlots('outro', presets.outro).map(() => ''),
  }
  const taken = { intro: 0, outro: 0 }
  const ruled = { intro: false, outro: false }
  for (const entry of entries) {
    const kind = kindOf(entry.element)
    if (!kind) continue
    const rows = shown(entry)
    const drawn = Math.max(0, Math.min(rows.length, regionRows[kind].length - taken[kind]))
    const paints = rows.slice(0, drawn).some((row) => row.text.trim() !== '')
    placement.set(entry.instanceId, { offset: taken[kind], drawn, rules: !ruled[kind] && paints })
    if (paints) ruled[kind] = true
    rows.slice(0, drawn).forEach((row, i) => (regionRows[kind][taken[kind] + i] = row.text))
    taken[kind] += rows.length
  }
  const blocks: Record<'intro' | 'outro', ClipRegionLayout | undefined> = {
    intro: clipLayoutRegion('intro', presets.intro, ratio, regionRows.intro),
    outro: clipLayoutRegion('outro', presets.outro, ratio, regionRows.outro),
  }
  const visuals: Visual[] = entries.map((entry) => {
    const e = entry.element
    const kind = kindOf(e)
    if (kind) {
      const at = placement.get(entry.instanceId)!
      const block = blocks[kind]
      const lines: Line[] = []
      for (let i = 0; i < at.drawn; i++) {
        const slot = block?.slots.find((s) => s.index === at.offset + i)
        if (!slot) continue
        const colour = CLIP_DESIGN.color[slot.spec.fill as keyof typeof CLIP_DESIGN.color]
        slot.lines.forEach((line, k) =>
          lines.push({
            key: `${entry.instanceId}/${i}/${k}`,
            text: line.text,
            type: slot.spec.role as TypeName,
            size: line.size,
            tracking: slot.type.tracking,
            face: slot.type.face,
            weight: slot.type.weight,
            baseline: line.baseline,
            alpha: colour.alpha * (slot.spec.alpha ?? 1),
            stroke:
              slot.spec.stroke === 'text'
                ? CLIP_DESIGN.spacing.stroke_text
                : slot.spec.stroke === 'small'
                  ? CLIP_DESIGN.spacing.stroke_small
                  : 0,
            shadow: !!slot.spec.shadow,
            slot: slot.index,
          }),
        )
      }
      return { entry, lines, kind, rules: at.rules }
    }
    const lines = shown(entry)
      .map((row, i) => {
        const type = (
          e.role === 'badge' ? 'badge' : e.role === 'info' ? row.role || 'caption' : 'title'
        ) as TypeName
        const colour = CLIP_DESIGN.color[type === 'label' ? 'text_muted' : 'text_white']
        return typeLine(`${entry.instanceId}/${i}`, row.text, type, ratio, {
          alpha: colour.alpha,
          tracking:
            e.role === 'info' && type === 'label'
              ? CLIP_DESIGN.information.label_tracking
              : CLIP_DESIGN.type[type].tracking,
          stroke:
            e.role === 'badge'
              ? 0
              : e.role === 'info'
                ? CLIP_DESIGN.spacing.stroke_small
                : CLIP_DESIGN.spacing.stroke_text,
          shadow: e.role !== 'badge',
        })
      })
      .filter((line) => line.text.trim() !== '')
    return { entry, lines }
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
      CLIP_DESIGN.faces[
        (line.face ?? CLIP_DESIGN.type[line.type].face) as keyof typeof CLIP_DESIGN.faces
      ],
    fontSize: line.size,
    fontWeight: line.weight ?? CLIP_DESIGN.type[line.type].weight,
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
  const occupiedRegions = (['intro', 'outro'] as const)
    .filter((kind) => visuals.some((v) => v.kind === kind && v.lines.length))
    .flatMap((kind) => [
      ...(blocks[kind]?.slots ?? []).map((slot) => slot.box),
      ...(blocks[kind]?.rules ?? []).map((rule) => rule.box),
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
  const paintedText = (
    line: Line,
    center: number,
    y: number,
    accented = false,
    align: 'centre' | 'left' = 'centre',
  ) => (
    <text
      key={line.key}
      {...props(line)}
      x={align === 'left' ? center - ink(line).x : center - ink(line).x - ink(line).width / 2}
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
        if (v.kind) {
          const block = blocks[v.kind]
          return (
            <g
              key={v.entry.instanceId}
              data-element={e.id}
              data-region={v.kind}
              data-preset={presets[v.kind]}
            >
              {v.lines.map((line) =>
                paintedText(line, block!.anchorX, line.baseline!, false, block!.align),
              )}
              {v.rules &&
                block?.rules.map((r, i) => (
                  <rect
                    key={i}
                    data-rule={r.kind}
                    x={r.box.x}
                    y={r.box.y}
                    width={r.box.width}
                    height={r.box.height}
                    fill={CLIP_DESIGN.color.text_white.hex}
                    fillOpacity={r.alpha}
                  />
                ))}
            </g>
          )
        }
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
