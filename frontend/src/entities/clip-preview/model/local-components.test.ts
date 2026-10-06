import { expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  freezeBrowserComposition,
  evaluateBrowserFrame,
  readBrowserCompositionSnapshot,
} from './browser-composition'
import { BrowserLocalComponents } from './local-components'
import type { BrowserInkRasterizer } from './ink-raster'

async function snapshot() {
  const plan = clipTimelineFixture().plan
  plan.elements = [plan.elements![0]!]
  return freezeBrowserComposition({
    ownerId: 'owner',
    projectId: 'project',
    projectRevision: 1,
    planRevision: 1,
    plan,
    ratio: 'vertical',
    design: { hideDisclosure: true },
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
