import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { freezeBrowserComposition } from './browser-composition'
import { BrowserLocalComponents } from './local-components'
import { BrowserFootageResources } from './browser-footage'
import { measureBrowserBackground } from './background-sampling'
import type { DecodedVideoResource } from '@/shared/lib'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})
async function fixture() {
  const plan = clipTimelineFixture().plan
  plan.elements = [
    plan.elements![0]!,
    { ...plan.elements![0]!, instanceId: 'another-caption', elementId: 'another-caption' },
  ]
  const snapshot = await freezeBrowserComposition({
    ownerId: 'owner',
    projectId: 'project',
    projectRevision: 1,
    planRevision: 1,
    plan,
    ratio: 'vertical',
    design: { hideDisclosure: true },
    sources: plan.cuts.map((c) => ({
      sourceId: c.sourceId,
      fingerprint: c.fingerprint,
      durationMs: 20000,
      width: 1920,
      height: 1080,
      hasAudio: false,
      allowedRatePermille: [1000],
    })),
  })
  const components = new BrowserLocalComponents(snapshot)
  const geometry = vi.spyOn(components, 'backgroundGeometry').mockResolvedValue({
    region: { x: 10, y: 20, width: 4, height: 4 },
    anchor: 'bottom',
    plate: false,
    contrastParts: [
      { box: { x: 10, y: 20, width: 4, height: 4 }, fill: '#FFFFFF', alpha: 1, stroke: true }, // style-escape: fixed white diagnostic ground
    ],
  })
  let level = 0
  const pixelRead = vi.fn((width: number, height: number) => ({
    data: new Uint8ClampedArray(width * height * 4).fill(level),
  }))
  const canvases: { width: number; height: number }[] = []
  vi.stubGlobal(
    'OffscreenCanvas',
    class {
      constructor(
        public width: number,
        public height: number,
      ) {
        canvases.push(this)
      }
      getContext() {
        return { setTransform: vi.fn(), getImageData: () => pixelRead(this.width, this.height) }
      }
    },
  )
  const close = vi.fn()
  const frames: number[] = []
  const prepare = vi
    .spyOn(BrowserFootageResources.prototype, 'prepare')
    .mockImplementation(async (frame) => {
      frames.push(frame.frame)
      const resource = {
        draw: () => {
          level = frames.length === 2 ? 255 : 0
        },
        close,
      } as unknown as DecodedVideoResource
      return [{ resource, layer: frame.footageLayers[0]! }]
    })
  const dispose = vi.spyOn(BrowserFootageResources.prototype, 'dispose')
  vi.spyOn(BrowserFootageResources.prototype, 'measurements').mockReturnValue({
    originals: [{ colorVersion: 'native-source-color-v1' }],
  } as ReturnType<BrowserFootageResources['measurements']>)
  return { snapshot, components, geometry, pixelRead, canvases, close, frames, prepare, dispose }
}
it('shares compatible original frame/ROI reads while preserving three temporal samples per visual', async () => {
  const f = await fixture()
  const observed = vi.fn()
  try {
    const evidence = await measureBrowserBackground(
      f.snapshot,
      f.components,
      vi.fn(),
      new AbortController().signal,
      observed,
    )
    expect(f.frames).toEqual([4, 60, 117])
    expect(f.pixelRead).toHaveBeenCalledTimes(3)
    expect(f.close).toHaveBeenCalledTimes(3)
    expect(f.canvases.every((c) => c.width === 0 && c.height === 0)).toBe(true)
    expect(evidence.measurements).toHaveLength(2)
    for (const m of evidence.measurements) {
      expect(m.ground.mean).toBeCloseTo(1 / 3)
      expect(m.scrim).toBe(true)
      expect(m.accentWhite).toBe(false)
    }
    expect(evidence.snapshotFingerprint).toBe(f.snapshot.snapshotFingerprint)
    expect(evidence.digest).toMatch(/^[a-f0-9]{64}$/u)
    expect(observed).toHaveBeenCalledWith(
      expect.objectContaining({ roiReads: 3, peakRegionBytes: 64 }),
    )
  } finally {
    f.components.destroy()
  }
})
it('refuses a late prepared frame after cancellation and closes its resources without publishing evidence', async () => {
  const f = await fixture()
  const c = new AbortController()
  const prepare = f.prepare.getMockImplementation()!
  f.prepare.mockImplementation(async (frame) => {
    const resources = await prepare(frame)
    c.abort(new DOMException('cancel', 'AbortError'))
    return resources
  })
  try {
    await expect(
      measureBrowserBackground(f.snapshot, f.components, vi.fn(), c.signal),
    ).rejects.toMatchObject({ name: 'AbortError' })
    expect(f.close).toHaveBeenCalledOnce()
    expect(f.pixelRead).not.toHaveBeenCalled()
    expect(f.dispose).toHaveBeenCalled()
  } finally {
    f.components.destroy()
  }
})
it('refuses missing region capability before accessing originals', async () => {
  const f = await fixture()
  vi.stubGlobal('OffscreenCanvas', undefined)
  try {
    await expect(
      measureBrowserBackground(f.snapshot, f.components, vi.fn(), new AbortController().signal),
    ).rejects.toThrow('CLIP_BACKGROUND_UNSUPPORTED')
    expect(f.prepare).not.toHaveBeenCalled()
  } finally {
    f.components.destroy()
  }
})
it('does not accept an original decode with a different color profile as background evidence', async () => {
  const f = await fixture()
  vi.mocked(BrowserFootageResources.prototype.measurements).mockReturnValue({
    originals: [{ colorVersion: 'foreign' }],
  } as ReturnType<BrowserFootageResources['measurements']>)
  try {
    await expect(
      measureBrowserBackground(f.snapshot, f.components, vi.fn(), new AbortController().signal),
    ).rejects.toThrow('CLIP_SOURCE_COLOR_UNSUPPORTED')
    expect(f.close).toHaveBeenCalledTimes(3)
    expect(f.dispose).toHaveBeenCalled()
  } finally {
    f.components.destroy()
  }
})
it('refuses nonfinite sampled geometry and releases the prepared original frame', async () => {
  const f = await fixture()
  f.geometry.mockResolvedValue({
    region: { x: 0, y: 0, width: Infinity, height: 100000 },
    anchor: 'bottom',
    plate: false,
  })
  try {
    await expect(
      measureBrowserBackground(f.snapshot, f.components, vi.fn(), new AbortController().signal),
    ).rejects.toThrow('CLIP_BACKGROUND_INVALID')
    expect(f.close).toHaveBeenCalledOnce()
    expect(f.dispose).toHaveBeenCalled()
  } finally {
    f.components.destroy()
  }
})
