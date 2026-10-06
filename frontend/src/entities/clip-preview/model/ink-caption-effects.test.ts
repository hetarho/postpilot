import { describe, expect, it } from 'vitest'
import {
  CLIP_CAPTION_INK,
  CLIP_CAPTION_EFFECT_STYLES,
} from '@/entities/clip-design/@x/clip-preview'
import type { InkCaptionLayout } from './ink-layout'
import { inkCaptionEffectsScene } from './ink-caption-effects'
import { captionGaussian, captionGaussianSigma } from './ink-caption-filter-pixi'

function layout(id: string): InkCaptionLayout {
  const style = CLIP_CAPTION_INK[id as keyof typeof CLIP_CAPTION_INK]
  return {
    style,
    role: { ...style.role, face: style.face, weight: style.weight },
    region: { x: 290, y: 640, width: 500, height: 200 },
    notice: '',
    lines: [0, 1].map((i) => ({
      text: '갂 AV',
      x: 290,
      y: 720 + i * 100,
      top: 640 + i * 100,
      width: 500,
      height: 100,
      keyword: '',
      keywordX: 0,
      keywordWidth: 0,
      words: [],
    })),
  }
}
describe('native filter state and reusable ink', () => {
  it.each(CLIP_CAPTION_EFFECT_STYLES)(
    '%s preserves deterministic frame state and all document keys across seeks',
    (id) => {
      const scene = inkCaptionEffectsScene('vertical', layout(id)),
        keys = scene.nodes.map((n) => n.document?.key)
      const states = [0.007, 0.031, 0.083, 0.196, 0.25, 0.5, 0.583, 0.75, 0.99].map((p) =>
        scene.nodes.map((n) => n.pose(p, 4500)),
      )
      expect(states[5]).toEqual(scene.nodes.map((n) => n.pose(0.5, 4500)))
      expect(keys).toEqual(scene.nodes.map((n) => n.document?.key))
      expect(scene.bounds.x).toBeGreaterThanOrEqual(0)
      expect(scene.bounds.width).toBeLessThanOrEqual(1080)
    },
  )
  it('blur-in uses authored settled cubic, sigma and pivot instead of frame rate', () => {
    const scene = inkCaptionEffectsScene('vertical', layout('blur-in'))
    const first = scene.nodes[0]!.pose(0, 4500),
      settled = scene.nodes[0]!.pose(0.36, 1000)
    expect(first.effect).toEqual({ kind: 'blur', sigma: 26 })
    expect(first.matrix[0]).toBe(1.05)
    expect(settled.effect).toEqual({ kind: 'blur', sigma: 0.01 })
    expect(settled.matrix.map((n) => n || 0)).toEqual([1, 0, 0, 1, 0, 0])
  })
  it('neon retains pulse/flicker and two wide plus tight merges', () => {
    const scene = inkCaptionEffectsScene('vertical', layout('neon'))
    const peak = scene.nodes[0]!.pose(1 / 12, 4500).effect
    expect(peak).toMatchObject({
      kind: 'neon',
      wideSigma: 26,
      tightSigma: 8,
      wideAlpha: 0.9,
      tightAlpha: 0.95,
    })
    const p = 19 / 97 + 0.00001,
      e = scene.nodes[0]!.pose(p, 4500).effect
    const pulse = (0.8 + 0.2 * Math.sin(p * 2 * Math.PI * 3)) * 0.42
    expect(e).toMatchObject({ wideSigma: Number((26 * pulse).toFixed(3)) })
  })
  it('glitch repeats the modular RGB and six world-clipped bands for each line', () => {
    const scene = inkCaptionEffectsScene('vertical', layout('glitch'))
    expect(scene.nodes).toHaveLength(16)
    for (const p of [0.007, 0.12, 0.5, 0.99])
      for (const node of scene.nodes.filter((n) => n.id.includes('band'))) {
        const pose = node.pose(p, 4500)
        expect(pose.clip!.x + pose.matrix[4]).toBe(0)
        expect(pose.clip!.width).toBe(1080)
      }
  })
  it('iridescent caches a group SourceAlpha halo and varies only the native clamped stops', () => {
    const scene = inkCaptionEffectsScene('vertical', layout('iridescent'))
    expect(scene.nodes.filter((n) => n.id === 'glow')).toHaveLength(1)
    const node = scene.nodes.find((n) => n.id === '0/fill')!,
      effect = node.pose(0.5, 4500).effect
    expect(effect).toMatchObject({
      kind: 'gradient',
      stops: [{ at: 0 }, { at: 0.27 }, { at: 0.55 }, { at: 0.83 }, { at: 1 }, { at: 1 }],
    })
  })
  it('calibrates the qualified Pixi15 kernel variance while retaining finite cropped padding', () => {
    const filter = captionGaussian(26)
    expect(filter.quality).toBe(4)
    expect(filter.strength).toBeCloseTo(12.92, 1)
    captionGaussianSigma(filter, 8)
    expect(filter.padding).toBe(0)
    filter.destroy()
  })
})
