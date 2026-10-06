import { it, expect, vi } from 'vitest'
import { Texture, Sprite, type Container, type WebGLRenderer } from 'pixi.js'
import type { BrowserCaptionPreparedScene } from './ink-caption-draw'
import { BrowserCaptionScenePixi } from './ink-caption-pixi'
import { inkEmberTongue } from './ink-caption-ember'
import { CLIP_CAPTION_EFFECT_PAINT as PAINT } from '@/entities/clip-design/@x/clip-preview'
const palette = PAINT.ember.stops
const pose = { opacity: 1, matrix: [1, 0, 0, 1, 0, 0] },
  bounds = { x: 0, y: 0, width: 160, height: 180 }
const scene = (key: string) => ({
  style: 'ember',
  opacity: 1,
  bounds,
  nodes: [
    {
      id: 'back',
      flames: { sigma: 12, stops: palette },
      pose: { ...pose, opacity: 0.9, flames: [inkEmberTongue(80, 170, 140, 20, 0, 0.5)] },
    },
    {
      id: 'text',
      document: { key, bounds: { x: 40, y: 80, width: 80, height: 20 } },
      ink: { offset: { x: 0, y: 0 }, gpu: () => Texture.WHITE },
      pose,
    },
    {
      id: 'front',
      flames: { sigma: 4.5, stops: palette },
      pose: { ...pose, opacity: 0.86, flames: [inkEmberTongue(80, 120, 80, 10, 0, 0.5)] },
    },
    {
      id: 'sparks',
      sparks: { fill: PAINT.ember.spark },
      pose: { ...pose, sparks: [{ index: 0, cx: 70, cy: 40, radius: 2, alpha: 0.5 }] },
    },
  ],
})
const renderer = {
  context: { webGLVersion: 2 },
  gl: { NO_ERROR: 0, getError: () => 0, isContextLost: () => false },
  texture: { managedTextures: [] },
  render: vi.fn(),
}
it('retains native back/text/front/spark paint order when a cached scene changes text', () => {
  const resized = vi.spyOn(Sprite.prototype, 'onViewUpdate')
  const s = new BrowserCaptionScenePixi(renderer as unknown as WebGLRenderer, vi.fn())
  const internals = s as unknown as {
    nodes: Map<string, { container: Container; ember?: object }>
    group: Container
    surface: Texture
    quad: { onViewUpdate: () => void }
  }
  const order = () => {
    const nodes = [...internals.nodes.entries()]
    return internals.group.children.map((c) => nodes.find(([, v]) => v.container === c)![0])
  }
  try {
    s.render(scene('first-text') as unknown as BrowserCaptionPreparedScene)
    expect(order()).toEqual(['back/rect', 'text/first-text', 'front/rect', 'sparks/rect'])
    expect(internals.surface.dynamic).toBe(true)
    const stable = [...internals.nodes.values()].filter((n) => n.ember).map((n) => n.container)
    for (const key of [
      'second-text',
      'rapid-cue',
      'first-text',
      ...Array.from({ length: 72 }, (_, i) => 'evicted-' + i),
      'first-text',
    ]) {
      s.render(scene(key) as unknown as BrowserCaptionPreparedScene)
      expect(order()).toEqual(['back/rect', 'text/' + key, 'front/rect', 'sparks/rect'])
      expect([...internals.nodes.values()].filter((n) => n.ember).map((n) => n.container)).toEqual(
        stable,
      )
    }
    resized.mockClear()
    const bigger = scene('first-text')
    bigger.bounds = { ...bounds, width: 200, height: 210 }
    s.render(bigger as unknown as BrowserCaptionPreparedScene)
    expect(internals.surface.width).toBe(200)
    expect(internals.surface.height).toBe(210)
    expect(resized).toHaveBeenCalled()
    s.render(scene('first-text') as unknown as BrowserCaptionPreparedScene)
    expect(internals.surface.width).toBe(160)
    expect(internals.surface.height).toBe(180)
    const reversed = scene('first-text')
    reversed.nodes.reverse()
    s.render(reversed as unknown as BrowserCaptionPreparedScene)
    expect(order()).toEqual(['sparks/rect', 'front/rect', 'text/first-text', 'back/rect'])
  } finally {
    s.destroy()
    resized.mockRestore()
  }
})
