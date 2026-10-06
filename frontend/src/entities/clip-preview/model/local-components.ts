import {
  CLIP_DESIGN,
  CLIP_INK_FONT_DATA,
  CLIP_TRANSITION,
} from '@/entities/clip-design/@x/clip-preview'
import {
  evaluateBrowserFrame,
  type BrowserCompositionSnapshot,
  type BrowserEvaluatedFrame,
} from './browser-composition'
import { BrowserInkCache, type BrowserInkLease } from './ink-cache'
import { ResvgBrowserInk, type BrowserInkRasterizer, type InkDocument } from './ink-raster'
import { ClipInkError, inkCaptionStyle } from './ink-typography'
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

type Component = BrowserCompositionSnapshot['components'][number]
type State = BrowserEvaluatedFrame['components'][number]
interface Description {
  document?: InkDocument
  caption?: InkCaptionLayout
  layer: number
  motion: { inMs: number; outMs: number; dy: number }
}
export interface BrowserLocalComponent extends BrowserInkLease {
  readonly component: State
  readonly document: InkDocument
  readonly caption?: InkCaptionLayout
  readonly layer: number
  draw(context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D): void
}

/** Local owned drawing; its only inputs are a frozen component/time contract and fixed assets. */
export class BrowserLocalComponents {
  private descriptions = new Map<string, Promise<Description>>()
  private cache: BrowserInkCache
  private headers?: Promise<Map<string, InkDocument | undefined>>
  private components: Set<Component>
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
        if (style.rendering !== 'static')
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
    return new Map(items.map((item) => [item.component.instanceId, item.document]))
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
      const style = component.componentId.slice('caption/'.length)
      const position =
        e.position === 'auto'
          ? e.effectivePosition ||
            (e.ownerPosition
              ? CLIP_DESIGN.regions.caption[style as keyof typeof CLIP_DESIGN.regions.caption]
                  .anchor
              : '')
          : e.position
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
        document: inkStaticCaption(ratio, caption, paint),
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
        resources.push({
          ...lease,
          component: state,
          document: doc,
          caption: value.caption,
          layer: value.layer,
          draw: (context) => {
            context.save()
            context.globalAlpha *= motion.opacity
            context.drawImage(
              lease.bitmap,
              doc.bounds.x + lease.offset.x,
              doc.bounds.y + lease.offset.y + motion.dy,
            )
            context.restore()
          },
        })
      }
      return resources.sort((a, b) => a.layer - b.layer)
    } catch (error) {
      resources.forEach((resource) => resource.close())
      throw error
    }
  }
  measurements() {
    return this.cache.measurements()
  }
  dropGPU(owner: object) {
    this.cache.dropGPU(owner)
  }
  destroy() {
    this.cache.destroy()
    this.rasterizer.destroy()
    this.descriptions.clear()
    this.headers = undefined
  }
}
