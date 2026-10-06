import {
  CLIP_DESIGN,
  CLIP_INK_FONT_DATA,
  CLIP_TRANSITION,
  type ClipRegionLayout,
  CLIP_CAPTION_TRANSFORM_STYLES,
  CLIP_CAPTION_EFFECT_STYLES,
} from '@/entities/clip-design/@x/clip-preview'
import {
  evaluateBrowserFrame,
  type BrowserCompositionSnapshot,
  type BrowserEvaluatedFrame,
} from './browser-composition'
import { BrowserInkCache, type BrowserInkLease } from './ink-cache'
import { ResvgBrowserInk, type BrowserInkRasterizer, type InkDocument } from './ink-raster'
import { ClipInkError, inkCaptionStyle, inkResolveCaption } from './ink-typography'
import { inkLayoutCaption, type InkCaptionLayout } from './ink-layout'
import {
  inkRegionCapacity,
  inkStaticBadge,
  inkStaticCaption,
  inkStaticInfo,
  inkStaticRegion,
  type InkPaint,
} from './ink-static'
import { previewMotion } from './draft-preview'
import { backgroundBoxUnion } from './background-math'
import type { InkBox } from './ink-typography'
import { coveredBrowserFootage, selectBrowserAnchor } from './composition-placement'
import { inkCaptionScene, type InkCaptionScene } from './ink-caption-scene'
import { inkCaptionEffectsScene } from './ink-caption-effects'
import { BrowserCaptionSceneCanvas, type BrowserCaptionPreparedScene } from './ink-caption-draw'

type Component = BrowserCompositionSnapshot['components'][number]
type State = BrowserEvaluatedFrame['components'][number]
interface Description {
  document?: InkDocument
  caption?: InkCaptionLayout
  regionBlock?: { layout: ClipRegionLayout; owner: boolean; offset: number; count: number }
  scene?: InkCaptionScene
  layer: number
  motion: { inMs: number; outMs: number; dy: number }
}
export interface BrowserBackgroundGeometry {
  region: InkBox
  caption?: InkCaptionLayout
  anchor: string
  plate: boolean
  motion?: { inMs: number; outMs: number; dy: number }
  contrastParts?: { box: InkBox; fill: string; alpha: number; stroke: boolean }[]
  regionBlock?: { layout: ClipRegionLayout; owner: boolean; offset: number; count: number }
}
interface BrowserLocalComponentBase {
  readonly component: State
  readonly caption?: InkCaptionLayout
  readonly layer: number
  draw(context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D): void
  close(): void
}
export type BrowserLocalComponent = BrowserLocalComponentBase &
  (
    | ({ readonly kind: 'ink'; readonly document: InkDocument } & BrowserInkLease)
    | { readonly kind: 'scene'; readonly scene: BrowserCaptionPreparedScene }
  )

/** Local owned drawing; its only inputs are a frozen component/time contract and fixed assets. */
export class BrowserLocalComponents {
  private descriptions = new Map<string, Promise<Description>>()
  private layout?: Promise<void>
  private placements = new Map<string, { anchor: string; style: string; advisories: string[] }>()
  private headerBounds = new Map<string, InkBox>()
  private cache: BrowserInkCache
  private headers?: Promise<Map<string, InkDocument | undefined>>
  private components: Set<Component>
  private sceneCanvas = new BrowserCaptionSceneCanvas()
  constructor(
    private snapshot: BrowserCompositionSnapshot,
    private rasterizer: BrowserInkRasterizer = new ResvgBrowserInk(),
  ) {
    this.verifyFonts(snapshot)
    this.components = new Set(snapshot.components)
    this.cache = new BrowserInkCache((document, signal) => rasterizer.render(document, signal))
  }
  private verifyFonts(snapshot: BrowserCompositionSnapshot) {
    evaluateBrowserFrame(snapshot, 0)
    const expected = Object.values(CLIP_INK_FONT_DATA.sources)
      .map((f) => f.sha256)
      .sort()
    if (JSON.stringify(snapshot.fonts.map((f) => f.sha256).sort()) !== JSON.stringify(expected))
      throw new ClipInkError('CLIP_INK_FONT_CHANGED', 'snapshot')
    for (const component of snapshot.components)
      if (component.element.role === 'caption') {
        const style = inkCaptionStyle(component.componentId.slice('caption/'.length))
        if (
          style.rendering !== 'static' &&
          ![...CLIP_CAPTION_TRANSFORM_STYLES, ...CLIP_CAPTION_EFFECT_STYLES].some(
            (id) => id === style.id,
          )
        )
          throw new ClipInkError('CLIP_INK_STYLE_NOT_IMPLEMENTED', style.id)
      }
  }
  /** Plan edits retain compatible ink, while callers fence old frame callbacks with T591 epochs. */
  updateSnapshot(snapshot: BrowserCompositionSnapshot) {
    if (
      snapshot.ownerId !== this.snapshot.ownerId ||
      snapshot.projectId !== this.snapshot.projectId
    )
      throw new ClipInkError('CLIP_INK_INCOMPATIBLE_OWNER')
    this.verifyFonts(snapshot)
    this.snapshot = snapshot
    this.components = new Set(snapshot.components)
    this.descriptions.clear()
    this.headers = undefined
    this.layout = undefined
    this.placements.clear()
    this.headerBounds.clear()
  }
  private region(component: Component) {
    const kind = component.element.role === 'hook' ? 'intro' : 'outro'
    const id =
      kind === 'intro' ? this.snapshot.design.introPreset : this.snapshot.design.outroPreset
    const capacity = inkRegionCapacity(kind, id)
    const rows = Array<string>(capacity).fill('')
    let taken = 0,
      ruled = false
    let part = { offset: 0, count: 0, rules: false }
    for (const entry of this.snapshot.components) {
      if (entry.element.role !== component.element.role) continue
      const texts = entry.element.rows.length
        ? entry.element.rows.map((row) => row.text)
        : [entry.element.text]
      const count = Math.max(0, Math.min(texts.length, capacity - taken))
      const rules = !ruled && texts.slice(0, count).some((value) => value.trim())
      if (rules) ruled = true
      for (let i = 0; i < count; i++) rows[taken + i] = texts[i]!
      if (entry === component) part = { offset: taken, count, rules }
      taken += texts.length
    }
    return { kind: kind as 'intro' | 'outro', id, rows, part }
  }
  private async headerDocuments(
    signal?: AbortSignal,
  ): Promise<Map<string, InkDocument | undefined>> {
    const items: {
      component: Component
      document?: InkDocument
      box: { x: number; y: number; width: number; height: number }
    }[] = []
    for (const component of this.snapshot.components) {
      const e = component.element
      if (
        !['badge', 'info', 'disclosure'].includes(e.role) ||
        !['auto', 'header'].includes(e.position)
      )
        continue
      if (e.role === 'info') {
        const info = await inkStaticInfo(
          this.snapshot.ratio,
          e.rows.length ? e.rows : [{ role: 'caption', text: e.text }],
          e.position,
          this.rasterizer,
          signal,
        )
        if (info.box) items.push({ component, document: info.document, box: info.box })
      } else {
        const document = await inkStaticBadge(
          this.snapshot.ratio,
          e.text,
          e.position,
          e.align,
          this.rasterizer,
          signal,
        )
        items.push({ component, document, box: { ...document.placement! } })
      }
    }
    items.sort((a, b) => a.component.startMs - b.component.startMs)
    const geometry = CLIP_DESIGN.ratios[this.snapshot.ratio]
    const together = (a: Component, b: Component) => a.startMs < b.endMs && b.startMs < a.endMs
    const move = (item: (typeof items)[number], x: number, y: number, height: number) => {
      const dx = x - item.box.x,
        dy = y + (height - item.box.height) / 2 - item.box.y
      if (item.document)
        item.document = {
          ...item.document,
          bounds: {
            ...item.document.bounds,
            x: item.document.bounds.x + dx,
            y: item.document.bounds.y + dy,
          },
          sampledBounds: item.document.sampledBounds
            ? {
                ...item.document.sampledBounds,
                x: item.document.sampledBounds.x + dx,
                y: item.document.sampledBounds.y + dy,
              }
            : undefined,
          contrastParts: item.document.contrastParts?.map((p) => ({
            ...p,
            box: { ...p.box, x: p.box.x + dx, y: p.box.y + dy },
          })),
        }
      item.box = { x, y, width: item.box.width, height }
    }
    for (let start = 0; start < items.length;) {
      let end = start + 1,
        until = items[start]!.component.endMs
      while (end < items.length && items[end]!.component.startMs < until) {
        until = Math.max(until, items[end]!.component.endMs)
        end++
      }
      const group = items.slice(start, end),
        height = Math.max(...group.map((item) => item.box.height)),
        placed: typeof items = []
      for (const item of group)
        if (item.component.element.role !== 'info')
          move(item, item.box.x, geometry.badge.top, height)
      for (const item of group) {
        if (item.component.element.role !== 'info') continue
        for (let row = 0; ; row++) {
          const y = geometry.badge.top + row * (height + CLIP_DESIGN.spacing.gap_stack)
          let right = geometry.badge.right,
            x = geometry.anchor.left,
            count = 0
          if (!row)
            for (const badge of group)
              if (
                badge.component.element.role !== 'info' &&
                together(item.component, badge.component)
              )
                right = Math.min(right, badge.box.x - CLIP_DESIGN.spacing.gap_stack)
          for (const prior of placed)
            if (
              Math.abs(prior.box.y + prior.box.height / 2 - (y + height / 2)) < 0.01 &&
              together(item.component, prior.component)
            ) {
              x = Math.max(x, prior.box.x + prior.box.width + CLIP_DESIGN.spacing.gap_stack)
              count++
            }
          if ((count < 2 && x + item.box.width <= right) || row > group.length) {
            move(item, x, y, height)
            placed.push(item)
            break
          }
        }
      }
      start = end
    }
    this.headerBounds = new Map(items.map((item) => [item.component.instanceId, { ...item.box }]))
    return new Map(items.map((item) => [item.component.instanceId, item.document]))
  }
  private representative(component: Component): State {
    return {
      component,
      localFrame: 0,
      localTimeMs: 0,
      durationMs: component.endMs - component.startMs,
      progress: 0.5,
      animationProgress: 0.5,
      phraseIndex: component.phraseIndex,
      text:
        component.phraseIndex === undefined
          ? component.element.text
          : component.element.phrases![component.phraseIndex]!.text,
    }
  }
  /** Whole native layout order is fixed before random seeking or background sampling. */
  async resolveLayout(signal?: AbortSignal): Promise<void> {
    const snapshot = this.snapshot
    if (!this.layout) {
      const pending = (async () => {
        const placements = new Map<
          string,
          { anchor: string; style: string; advisories: string[] }
        >()
        const first = [
          ...new Map(
            snapshot.components.map((component) => [
              component.instanceId,
              snapshot.components.find((c) => c.instanceId === component.instanceId)!,
            ]),
          ).values(),
        ]
        const priority = (role: string) =>
          ['badge', 'disclosure'].includes(role)
            ? 0
            : ['hook', 'ending'].includes(role)
              ? 1
              : role === 'info'
                ? 2
                : 3
        first.sort((a, b) => priority(a.element.role) - priority(b.element.role))
        const obstacles: { component: Component; parts: InkBox[] }[] = []
        let previous = ''
        for (const component of first) {
          signal?.throwIfAborted()
          if (snapshot !== this.snapshot) throw new ClipInkError('CLIP_INK_SUPERSEDED')
          const e = component.element
          const state = this.representative(component)
          if (e.role !== 'caption') {
            const described = await this.describe(state, {}, signal)
            const document = described.document
            if (document)
              obstacles.push({
                component,
                parts: document.contrastParts?.map((part) => part.box) ?? [
                  document.placement ?? document.sampledBounds ?? document.bounds,
                ],
              })
            continue
          }
          // First rapid phrase chooses the common style/anchor against its own interval.
          const { style: selected } = inkResolveCaption(
            component.componentId.slice('caption/'.length),
            state.text,
          )
          const pinned = e.position !== 'auto' || !!e.effectivePosition || !!e.ownerPosition
          const anchors = pinned
            ? [e.position === 'auto' ? e.effectivePosition || selected.rule.anchor : e.position]
            : [selected.rule.anchor, selected.rule.anchor_alt].filter(
                (anchor, index, all) => !!anchor && all.indexOf(anchor) === index,
              )
          const fits: { anchor: string; layout: InkCaptionLayout }[] = []
          for (const anchor of anchors) {
            try {
              const layout = await inkLayoutCaption(
                {
                  text: state.text,
                  style: selected.id,
                  ratio: snapshot.ratio,
                  position: anchor,
                  align: e.align,
                  keyword: e.keyword,
                  pace: e.pace,
                  ownerPosition: e.ownerPosition,
                  ownerSizePx: e.ownerSizePx,
                },
                this.rasterizer,
                signal,
              )
              fits.push({ anchor, layout })
              if (pinned) break
            } catch (error) {
              if (
                !(error instanceof ClipInkError) ||
                !['CLIP_INK_COPY_LIMIT', 'CLIP_INK_SAFE_AREA'].includes(error.code)
              )
                throw error
            }
          }
          if (!fits.length) throw new ClipInkError('CLIP_INK_COPY_LIMIT', e.instanceId)
          const geometry = coveredBrowserFootage(snapshot, component)
          const placed = obstacles
            .filter(
              ({ component: other }) =>
                other.startMs < component.endMs && component.startMs < other.endMs,
            )
            .flatMap(({ parts }) => parts)
          const index = pinned
            ? 0
            : selectBrowserAnchor(
                fits.map(({ anchor, layout }) => ({
                  anchor,
                  align: e.align,
                  authoredAlign: e.align !== 'center',
                  plate: layout.region,
                  fits: true,
                })),
                geometry.subject,
                placed,
                geometry.readableText,
                previous,
                geometry.captionSafe,
              )
          const repairable =
            e.kind === 'ai' &&
            !e.ownerEdited &&
            !e.effectivePosition &&
            !e.ownerPosition &&
            !e.ownerSizePx &&
            !e.ownerStyle &&
            e.style === 'auto' &&
            e.position === 'auto' &&
            e.basis === 'cut' &&
            e.startMs === undefined &&
            e.endMs === undefined
          if (index < 0 && repairable)
            throw new ClipInkError('CLIP_INK_AUTOMATIC_PLACEMENT', e.instanceId)
          const chosen = fits[Math.max(0, index)]!
          placements.set(component.instanceId, {
            anchor: chosen.anchor,
            style: chosen.layout.style.id,
            advisories: index < 0 ? ['overlap'] : [],
          })
          previous = chosen.anchor
        }
        if (snapshot !== this.snapshot || signal?.aborted)
          throw new ClipInkError('CLIP_INK_SUPERSEDED')
        this.placements = placements
      })()
      this.layout = pending.catch((error) => {
        if (snapshot === this.snapshot) this.layout = undefined
        throw error
      })
    }
    await this.layout
    if (snapshot !== this.snapshot || signal?.aborted) throw new ClipInkError('CLIP_INK_SUPERSEDED')
  }
  async captionGeometry(instanceId: string, signal?: AbortSignal) {
    await this.resolveLayout(signal)
    const component = this.snapshot.components.find(
      (c) => c.instanceId === instanceId && c.element.role === 'caption',
    )
    if (!component) return undefined
    const description = await this.describe(this.representative(component), {}, signal)
    if (!description.caption) return undefined
    return {
      instanceId,
      box: description.caption.region,
      fontSize: description.caption.role.size,
      style: description.caption.style.id,
      representativeFrame: description.caption.style.rendering !== 'static',
      advisories: [...(this.placements.get(instanceId)?.advisories ?? [])],
    }
  }
  private async describe(
    state: State,
    paint: InkPaint,
    signal?: AbortSignal,
  ): Promise<Description> {
    const component = state.component,
      e = component.element,
      ratio = this.snapshot.ratio
    if (e.role === 'caption') {
      const placed = this.placements.get(component.instanceId)
      const style = placed?.style ?? component.componentId.slice('caption/'.length)
      const position =
        placed?.anchor ??
        (e.position === 'auto'
          ? e.effectivePosition ||
            (e.ownerPosition
              ? CLIP_DESIGN.regions.caption[style as keyof typeof CLIP_DESIGN.regions.caption]
                  .anchor
              : '')
          : e.position)
      const caption = await inkLayoutCaption(
        {
          text: state.text,
          style,
          ratio,
          position,
          align: e.align,
          keyword: e.keyword,
          pace: e.pace,
          ownerPosition: e.ownerPosition,
          ownerSizePx: e.ownerSizePx,
        },
        this.rasterizer,
        signal,
      )
      return {
        caption,
        ...(caption.style.rendering === 'static'
          ? { document: inkStaticCaption(ratio, caption, paint) }
          : {
              scene: CLIP_CAPTION_EFFECT_STYLES.some((id) => id === caption.style.id)
                ? inkCaptionEffectsScene(ratio, caption)
                : inkCaptionScene(ratio, caption, paint),
            }),
        layer: 1,
        motion: e.pace === 'rapid' ? { inMs: 0, outMs: 0, dy: 0 } : caption.style.motion,
      }
    }
    if (e.role === 'hook' || e.role === 'ending') {
      const region = this.region(component)
      const result = await inkStaticRegion(
        ratio,
        region.kind,
        region.id,
        region.rows,
        region.part,
        this.rasterizer,
        signal,
      )
      const transition = state.durationMs >= CLIP_TRANSITION.fade_ms ? CLIP_TRANSITION.fade_ms : 0
      return {
        document: result.document,
        regionBlock: {
          layout: result.layout,
          owner: region.part.rules,
          offset: region.part.offset,
          count: region.part.count,
        },
        layer: 2,
        motion: {
          inMs: e.role === 'ending' ? transition : 0,
          outMs: e.role === 'hook' ? transition : 0,
          dy: 0,
        },
      }
    }
    if (e.role === 'badge' || e.role === 'disclosure' || e.role === 'info') {
      let document: InkDocument | undefined
      if (e.position === 'auto' || e.position === 'header') {
        this.headers ??= this.headerDocuments(signal).catch((error) => {
          this.headers = undefined
          throw error
        })
        document = (await this.headers).get(component.instanceId)
      } else if (e.role === 'info')
        document = (
          await inkStaticInfo(
            ratio,
            e.rows.length ? e.rows : [{ role: 'caption', text: e.text }],
            e.position,
            this.rasterizer,
            signal,
          )
        ).document
      else
        document = await inkStaticBadge(ratio, e.text, e.position, e.align, this.rasterizer, signal)
      return { document, layer: e.role === 'info' ? 3 : 4, motion: { inMs: 0, outMs: 0, dy: 0 } }
    }
    throw new ClipInkError('CLIP_INK_UNSUPPORTED_ROLE', e.role)
  }
  async prepare(
    frame: BrowserEvaluatedFrame,
    signal?: AbortSignal,
    paintFor?: (instanceId: string) => InkPaint,
  ): Promise<BrowserLocalComponent[]> {
    await this.resolveLayout(signal)
    const resources: BrowserLocalComponent[] = []
    try {
      for (const state of frame.components) {
        if (!this.components.has(state.component)) throw new ClipInkError('CLIP_INK_SUPERSEDED')
        const paint = paintFor?.(state.component.instanceId) ?? {
          accent:
            CLIP_DESIGN.accent[state.component.element.accent as keyof typeof CLIP_DESIGN.accent],
        }
        const key = JSON.stringify([state.component.instanceId, state.phraseIndex, paint])
        let description = this.descriptions.get(key)
        if (!description) {
          description = this.describe(state, paint, signal).catch((error) => {
            this.descriptions.delete(key)
            throw error
          })
          this.descriptions.set(key, description)
        }
        const value = await description
        if (signal?.aborted || !this.components.has(state.component))
          throw new ClipInkError('CLIP_INK_SUPERSEDED')
        if (value.scene) {
          const scene = value.scene,
            progress = state.component.element.pace === 'rapid' ? 0.5 : state.progress
          const nodes: BrowserCaptionPreparedScene['nodes'][number][] = []
          let closed = false
          try {
            for (const node of scene.nodes) {
              const ink = node.document
                ? await this.cache.acquire(node.document, signal)
                : undefined
              nodes.push({
                id: node.id,
                document: node.document,
                ink,
                rect: node.rect,
                light: node.light,
                flames: node.flames,
                sparks: node.sparks,
                pose: node.pose(progress, state.durationMs),
              })
            }
            if (!this.components.has(state.component)) throw new ClipInkError('CLIP_INK_SUPERSEDED')
          } catch (error) {
            nodes.forEach((node) => node.ink?.close())
            throw error
          }
          const prepared = {
            style: scene.layout.style.id,
            bounds: scene.bounds,
            nodes,
            opacity: scene.opacity(progress, state.durationMs),
          }
          resources.push({
            kind: 'scene',
            component: state,
            caption: value.caption,
            layer: value.layer,
            scene: prepared,
            draw: (context) => {
              if (closed || !this.components.has(state.component))
                throw new ClipInkError('CLIP_INK_SUPERSEDED')
              this.sceneCanvas.draw(context, prepared)
            },
            close: () => {
              if (!closed) {
                closed = true
                nodes.forEach((node) => node.ink?.close())
              }
            },
          })
          continue
        }
        if (!value.document) continue
        const doc = value.document,
          lease = await this.cache.acquire(doc, signal)
        if (!this.components.has(state.component)) {
          lease.close()
          throw new ClipInkError('CLIP_INK_SUPERSEDED')
        }
        const motion = previewMotion(
          { startMs: state.component.startMs, endMs: state.component.endMs, ...value.motion },
          frame.timeMs,
        )
        let closed = false
        resources.push({
          ...lease,
          kind: 'ink',
          component: state,
          document: doc,
          caption: value.caption,
          layer: value.layer,
          draw: (context) => {
            if (closed || !this.components.has(state.component))
              throw new ClipInkError('CLIP_INK_SUPERSEDED')
            context.save()
            context.globalAlpha *= motion.opacity
            context.drawImage(
              lease.bitmap,
              doc.bounds.x + lease.offset.x,
              doc.bounds.y + lease.offset.y + motion.dy,
            )
            context.restore()
          },
          close: () => {
            if (!closed) {
              closed = true
              lease.close()
            }
          },
        })
      }
      return resources.sort((a, b) => a.layer - b.layer)
    } catch (error) {
      resources.forEach((resource) => resource.close())
      throw error
    }
  }
  /** Resolve native sampling geometry before any background-dependent paint. */
  async backgroundGeometry(
    state: State,
    signal?: AbortSignal,
  ): Promise<BrowserBackgroundGeometry | undefined> {
    await this.resolveLayout(signal)
    if (!this.components.has(state.component)) throw new ClipInkError('CLIP_INK_SUPERSEDED')
    const role = state.component.element.role
    if (!['caption', 'hook', 'ending', 'info'].includes(role)) return undefined
    const description = await this.describe(state, {}, signal)
    if (signal?.aborted || !this.components.has(state.component))
      throw new ClipInkError('CLIP_INK_SUPERSEDED')
    if (description.caption) {
      const lines = description.caption.lines
      if (lines.some((line) => !line.glyphBounds)) throw new ClipInkError('CLIP_BACKGROUND_MISSING')
      return {
        region: backgroundBoxUnion(lines.map((line) => line.glyphBounds!)),
        caption: description.caption,
        anchor:
          this.placements.get(state.component.instanceId)?.anchor ??
          (state.component.element.effectivePosition || state.component.element.position),
        plate: !!(description.caption.style.rule.plate || description.caption.style.paint.plate),
        motion: description.motion,
        contrastParts: lines.map((line) => ({
          box: line.glyphBounds!,
          fill: description.caption!.style.paint.fill,
          alpha: 1,
          stroke:
            !!description.caption!.style.rule.stroke &&
            description.caption!.style.paint.stroke === CLIP_DESIGN.color.stroke_dark.hex,
        })),
      }
    }
    const region = description.document?.sampledBounds
    if (!region) return undefined
    return {
      region,
      anchor: state.component.element.position,
      plate: false,
      motion: description.motion,
      regionBlock: description.regionBlock,
      contrastParts: description.document?.contrastParts,
    }
  }
  measurements() {
    return { ...this.cache.measurements(), canvas: this.sceneCanvas.measurements() }
  }
  dropGPU(owner: object) {
    this.cache.dropGPU(owner)
  }
  destroy() {
    this.components.clear()
    this.cache.destroy()
    this.rasterizer.destroy()
    this.descriptions.clear()
    this.headers = undefined
    this.sceneCanvas.destroy()
  }
}
