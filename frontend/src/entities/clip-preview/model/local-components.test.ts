import { expect, it, vi } from 'vitest'
import {
  CLIP_CAPTION_TRANSFORM_STYLES,
  CLIP_CAPTION_INK,
  type ClipRatioId,
} from '@/entities/clip-design/@x/clip-preview'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  freezeBrowserComposition,
  evaluateBrowserFrame,
  readBrowserCompositionSnapshot,
} from './browser-composition'
import { BrowserLocalComponents } from './local-components'
import type { BrowserInkRasterizer } from './ink-raster'

async function snapshot(
  style = 'bold',
  pace: 'steady' | 'rapid' = 'steady',
  ratio: ClipRatioId = 'vertical',
) {
  const plan = clipTimelineFixture().plan
  plan.elements = [plan.elements![0]!]
  plan.elements[0]!.style = style
  plan.elements[0]!.text = '여기 정말 좋아요'
  plan.elements[0]!.keyword = '정말'
  if (pace === 'rapid')
    plan.elements[0]!.phrases = [
      { text: '여기 정말', startMs: 120, endMs: 620 },
      { text: '좋아요', startMs: 620, endMs: 1120 },
    ]
  return freezeBrowserComposition({
    ownerId: 'owner',
    projectId: 'project',
    projectRevision: 1,
    planRevision: 1,
    plan,
    ratio,
    design: { hideDisclosure: true, captionStyles: [style], captionPace: pace },
    sources: [
      ...new Map(
        plan.cuts.map((cut) => [
          cut.sourceId,
          {
            sourceId: cut.sourceId,
            fingerprint: cut.fingerprint,
            durationMs: 20000,
            width: 1920,
            height: 1080,
            hasAudio: false,
            allowedRatePermille: [1000],
          },
        ]),
      ).values(),
    ],
  })
}
it('binds all shipped derived ink assets and refuses changed resource definitions on replay', async () => {
  const saved = await snapshot()
  expect(saved.ink.fontAssets.filter((f) => f.id.startsWith('wantedsans:'))).toHaveLength(4)
  expect(saved.versions.assets).toBe(saved.ink.assetVersion)
  const changed = JSON.parse(JSON.stringify(saved))
  changed.ink.fontAssets[0].sha256 = 'a'.repeat(64)
  await expect(readBrowserCompositionSnapshot(changed)).rejects.toThrow(
    'CLIP_SNAPSHOT_INVALID:derived contract',
  )
  expect(() => evaluateBrowserFrame(changed, 0)).toThrow('CLIP_SNAPSHOT_INCOMPATIBLE_VERSION:ink')
})
it.each(CLIP_CAPTION_TRANSFORM_STYLES)(
  '%s reuses prepared masks in every ratio and pace across seek/replay',
  async (style) => {
    for (const ratio of ['vertical', 'square', 'horizontal'] as const)
      for (const pace of ['steady', 'rapid'] as const) {
        const frozen = await snapshot(style, pace, ratio),
          bitmaps: { close: ReturnType<typeof vi.fn> }[] = []
        const raster: BrowserInkRasterizer = {
          measure: vi.fn(async (text) => ({
            x: 0,
            y: -80,
            width: [...text].length * 50,
            height: 100,
          })),
          render: vi.fn(async (doc) => {
            const value = {
              width: Math.ceil(doc.bounds.width) * (doc.rasterScale ?? 1),
              height: Math.ceil(doc.bounds.height) * (doc.rasterScale ?? 1),
              close: vi.fn(),
            }
            bitmaps.push(value)
            return value as unknown as ImageBitmap
          }),
          destroy: vi.fn(),
        }
        const components = new BrowserLocalComponents(frozen, raster)
        const sample = await components.prepare(evaluateBrowserFrame(frozen, 7))
        expect(sample).toHaveLength(1)
        expect(sample[0]!.kind).toBe('scene')
        const scene = sample[0]!.kind === 'scene' ? sample[0]!.scene : undefined
        const poses = scene!.nodes.map((node) => node.pose)
        sample.forEach((resource) => resource.close())
        const count = components.measurements().rasters
        for (const frame of [8, 14, 4, 7]) {
          const resources = await components.prepare(evaluateBrowserFrame(frozen, frame))
          if (frame === 7)
            expect(
              resources[0]!.kind === 'scene' && resources[0]!.scene.nodes.map((node) => node.pose),
            ).toEqual(poses)
          resources.forEach((resource) => resource.close())
        }
        expect(components.measurements()).toMatchObject({ rasters: count, leases: 0 })
        expect(() => sample[0]!.draw({} as CanvasRenderingContext2D)).toThrow('CLIP_INK_SUPERSEDED')
        components.destroy()
        components.destroy()
        expect(components.measurements()).toMatchObject({ entries: 0, bytes: 0, leases: 0 })
        bitmaps.forEach((bitmap) => expect(bitmap.close).toHaveBeenCalledOnce())
        expect(raster.destroy).toHaveBeenCalled()
      }
  },
)
it('rejects changed mask scale in a frozen snapshot', async () => {
  const frozen = JSON.parse(JSON.stringify(await snapshot()))
  frozen.ink.transformInkScale = 3
  expect(() => evaluateBrowserFrame(frozen, 0)).toThrow('CLIP_SNAPSHOT_INCOMPATIBLE_VERSION:ink')
  await expect(readBrowserCompositionSnapshot(frozen)).rejects.toThrow(
    'CLIP_SNAPSHOT_INVALID:derived contract',
  )
})
it.each(['bold', 'keynote', 'film'])(
  '%s preserves static fades, movement and rapid replacement without changing its ink',
  async (style) => {
    for (const pace of ['steady', 'rapid'] as const) {
      const frozen = await snapshot(style, pace)
      const raster: BrowserInkRasterizer = {
        measure: vi.fn(async (text) => ({
          x: 0,
          y: -80,
          width: [...text].length * 50,
          height: 100,
        })),
        render: vi.fn(
          async (doc) =>
            ({
              width: Math.ceil(doc.bounds.width),
              height: Math.ceil(doc.bounds.height),
              close: vi.fn(),
            }) as unknown as ImageBitmap,
        ),
        destroy: vi.fn(),
      }
      const components = new BrowserLocalComponents(frozen, raster)
      expect(await components.prepare(evaluateBrowserFrame(frozen, 3))).toEqual([])
      const first = await components.prepare(evaluateBrowserFrame(frozen, 4))
      expect(first[0]!.kind).toBe('ink')
      const context = {
        globalAlpha: 1,
        save: vi.fn(),
        restore: vi.fn(),
        drawImage: vi.fn(),
      } as unknown as CanvasRenderingContext2D
      first[0]!.draw(context)
      const image = first[0]!.kind === 'ink' ? first[0]!.document : undefined
      if (pace === 'rapid') {
        expect(context.globalAlpha).toBe(1)
        expect(vi.mocked(context.drawImage).mock.calls[0]![2]).toBe(image!.bounds.y)
      } else {
        const motion = CLIP_CAPTION_INK[style as 'bold' | 'keynote' | 'film'].motion
        expect(context.globalAlpha).toBeCloseTo((4000 / 30 - 120) / motion.inMs, 10)
        expect(
          Number(vi.mocked(context.drawImage).mock.calls[0]![2]) - image!.bounds.y,
        ).toBeCloseTo(motion.dy * (1 - (4000 / 30 - 120) / motion.inMs) ** 3, 10)
      }
      first.forEach((resource) => resource.close())
      expect(() => first[0]!.draw(context)).toThrow('CLIP_INK_SUPERSEDED')
      const mid = await components.prepare(evaluateBrowserFrame(frozen, 10))
      mid.forEach((resource) => resource.close())
      expect(raster.render).toHaveBeenCalledOnce()
      components.destroy()
    }
  },
)
it('reuses one raster across output frames and closes it at lifecycle end', async () => {
  const frozen = await snapshot()
  const raster: BrowserInkRasterizer = {
    measure: vi.fn(async (text) => ({ x: 0, y: -100, width: [...text].length * 70, height: 120 })),
    render: vi.fn(),
    destroy: vi.fn(),
  }
  const close = vi.fn()
  raster.render = vi.fn(
    async (doc) =>
      ({ width: doc.bounds.width, height: doc.bounds.height, close }) as unknown as ImageBitmap,
  )
  const components = new BrowserLocalComponents(frozen, raster)
  for (const index of [4, 5, 8, 20, 4]) {
    const frame = await components.prepare(evaluateBrowserFrame(frozen, index))
    expect(frame).toHaveLength(1)
    frame.forEach((ink) => ink.close())
  }
  expect(raster.render).toHaveBeenCalledOnce()
  components.destroy()
  expect(close).toHaveBeenCalledOnce()
})
