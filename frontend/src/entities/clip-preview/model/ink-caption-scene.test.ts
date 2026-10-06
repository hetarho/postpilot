import { describe, expect, it } from 'vitest'
import { CLIP_CAPTION_INK } from '@/entities/clip-design/@x/clip-preview'
import { inkCaptionScene, inkPopProgress } from './ink-caption-scene'
import { BrowserCaptionSceneCanvas } from './ink-caption-draw'
import type { InkCaptionLayout } from './ink-layout'

function layout(id: string, count = 3): InkCaptionLayout {
  const style = CLIP_CAPTION_INK[id as keyof typeof CLIP_CAPTION_INK],
    role = { ...style.role, face: style.face, weight: style.weight }
  return {
    style,
    role,
    region: { x: 290, y: 640, width: 500, height: 100 },
    notice: '',
    lines: [
      {
        text: '여기 진짜 좋아요',
        x: 290,
        y: 720,
        top: 640,
        width: 500,
        height: 100,
        keyword: '진짜',
        keywordX: 400,
        keywordWidth: 120,
        words: Array.from({ length: count }, (_, index) => ({
          text: '가',
          x: 290 + index * 60,
          width: 50,
        })),
      },
    ],
  }
}
describe('frozen caption scene state', () => {
  it('refuses use after a Canvas scene owner is destroyed',()=>{
    const canvas=new BrowserCaptionSceneCanvas()
    canvas.destroy()
    expect(()=>canvas.draw({} as CanvasRenderingContext2D,{bounds:{x:0,y:0,width:1,height:1},opacity:1,nodes:[]})).toThrow('CLIP_INK_CANCELLED')
  })
  it('retains the old four-word envelope and exposes every long word in rapid and settled snapshots', () => {
    expect(inkPopProgress(0.5, 3, 4)).toBeCloseTo(0.2, 12)
    expect(inkPopProgress(0.18, 1, 4)).toBe(0)
    for (const count of [9, 10, 22]) {
      const scene = inkCaptionScene('vertical', layout('pop', count))
      expect(scene.nodes.every((node) => node.pose(0.5, 800).opacity >= 0.44)).toBe(true)
      expect(scene.nodes.every((node) => node.pose(0.74, 4500).matrix[0] === 1)).toBe(true)
    }
  })
  it.each(['word-pop', 'pop', 'stack', 'sticker', 'bubble', 'serif', 'outline'])(
    '%s keeps reusable ink stable across first/last, replay and different processing order',
    (id) => {
      const scene = inkCaptionScene('vertical', layout(id), { accent: 'coral' })
      const keys = scene.nodes.map((node) => node.document?.key)
      const state = (p: number) =>
        JSON.stringify({
          opacity: scene.opacity(p, 4500),
          nodes: scene.nodes.map((node) => node.pose(p, 4500)),
        })
      const expected = state(0.5037037037037037)
      for (const p of [0, 1, 0.1, 0.74, 0.007407407407407408, 0.9925925925925926]) state(p)
      expect(state(0.5037037037037037)).toBe(expected)
      expect(scene.nodes.map((node) => node.document?.key)).toEqual(keys)
      expect(scene.opacity(1, 4500)).toBe(0)
    },
  )
  it('keeps actual cue duration and the native non-accent sticker border', () => {
    const scene = inkCaptionScene('vertical', layout('sticker'), { accent: 'coral' })
    expect(scene.opacity(0.5, 300)).not.toBe(scene.opacity(0.5, 1000))
    expect(scene.nodes[0]!.document!.svg).toContain(CLIP_CAPTION_INK.pop.paint.stroke)
  })
  it('reveals only the authored keyword rectangle without changing the text ink', () => {
    const scene = inkCaptionScene('vertical', layout('outline'))
    const fill = scene.nodes.find((node) => node.id.endsWith('/fill'))!
    expect(fill.pose(0.28, 3200).clip?.width).toBe(0)
    expect(fill.pose(0.56, 3200).clip).toMatchObject({ x: 394, y: 636, width: 132, height: 134.4 })
    expect(fill.document!.svg).toContain('여기 진짜 좋아요')
  })
})
