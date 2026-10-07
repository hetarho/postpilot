import { describe, expect, it } from 'vitest'
import { CLIP_CAPTION_INK } from '@/entities/clip-design/@x/clip-preview'
import type { InkCaptionLayout } from './ink-layout'
import {
  inkCaptionEmberScene,
  inkEmberCurveBounds,
  inkEmberGeometry,
  inkEmberFilterBounds,
} from './ink-caption-ember'
import { BrowserCaptionEmberPixi } from './ink-caption-ember-pixi'

const style = CLIP_CAPTION_INK.ember
const layout: InkCaptionLayout = {
  style,
  role: { ...style.role, face: style.face, weight: style.weight },
  region: { x: 200, y: 540, width: 680, height: 200 },
  notice: '',
  lines: [0, 1].map((i) => ({
    text: '갂 AV 정확한 불꽃',
    x: 200,
    y: 630 + i * 100,
    top: 540 + i * 100,
    width: 680,
    height: 100,
    keyword: '',
    keywordX: 0,
    keywordWidth: 0,
    words: [],
  })),
}
describe('native ember geometry and owned resources', () => {
  it('caches one text/stroke/double-wide-glow document and preserves layer order through seeks', () => {
    const scene = inkCaptionEmberScene('vertical', layout)
    expect(scene.nodes.map((n) => n.id)).toEqual(['back', 'text', 'front', 'sparks'])
    expect(scene.nodes.filter((n) => n.document)).toHaveLength(1)
    const text = scene.nodes[1]!.document!
    expect(text.svg.match(/<feMergeNode in="g0"/g)).toHaveLength(2)
    expect(text.svg).not.toContain('<path')
    const half = scene.nodes.map((n) => n.pose(0.5, 4500))
    for (const p of [0.99, 0, 0.25, 0.007, 0.75]) scene.nodes.map((n) => n.pose(p, 4500))
    expect(scene.nodes.map((n) => n.pose(0.5, 4500))).toEqual(half)
    expect(scene.nodes[1]!.document).toBe(text)
    expect(scene.opacity(0, 4500)).toBe(0)
    expect(scene.opacity(0.5, 4500)).toBe(1)
    expect(scene.opacity(1, 4500)).toBe(0)
  })
  it('derives true cubic extrema rather than expanding the gradient to control points', () => {
    const b = inkEmberCurveBounds([0, 10, -10, 8, -10, 2, 0, 0, 10, 2, 10, 8, 0, 10])
    expect(b).toEqual({ x: -7.5, y: 0, width: 15, height: 10 })
  })
  it('bounds every native index and spark lifecycle even at maximum horizontal copy width', () => {
    for (const width of [1, 680, 1920])
      for (const p of [0, 0.007, 0.16, 0.25, 0.5, 0.74, 0.999]) {
        const g = inkEmberGeometry({ ...layout.region, width }, p)
        expect(g.back.length).toBeLessThanOrEqual(21)
        expect(g.front.length).toBeLessThanOrEqual(33)
        expect(g.sparks.map((s) => s.index)).toEqual(Array.from({ length: 16 }, (_, i) => i))
        expect(new Set(g.sparks.map((s) => s.cx)).size).toBeGreaterThan(1)
        for (const f of [...g.back, ...g.front]) {
          expect(f.points).toHaveLength(14)
          expect(f.points.every(Number.isFinite)).toBe(true)
          expect(f.bounds.width).toBeGreaterThan(0)
          expect(f.bounds.height).toBeLessThan(414)
        }
        expect(
          g.sparks.every((s) => s.radius >= 0 && s.radius < 5.1 && s.alpha >= 0 && s.alpha <= 0.85),
        ).toBe(true)
      }
  })
  it('uses native default SVG filter areas and reuses GPU quads/uniform storage across time', () => {
    const scene = inkCaptionEmberScene('vertical', layout),
      node = scene.nodes[0]!,
      renderer = new BrowserCaptionEmberPixi(node)
    const first = node.pose(0.01, 4500),
      b = inkEmberFilterBounds(first.flames!)
    expect(b.width).toBeGreaterThan(Math.max(...first.flames!.map((f) => f.bounds.width)))
    renderer.update(first)
    const children = [...renderer.group.children],
      resources = renderer.measurements()
    for (const p of [0.07, 0.5, 0.99, 0.01]) renderer.update(node.pose(p, 4500))
    expect(renderer.group.children).toEqual(children)
    expect(renderer.measurements()).toEqual(resources)
    renderer.destroy()
    renderer.destroy()
    expect(renderer.measurements()).toEqual({
      shapes: 0,
      filters: 0,
      geometryBytes: 0,
      uniformBytes: 0,
    })
  })
})
