import { expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { freezeBrowserComposition, evaluateBrowserFrame } from './browser-composition'
import { BrowserFootageResources } from './browser-footage'
import type { OriginalVideoInput } from '@/shared/lib'

async function snapshot(width = 2, height = 2) {
  const plan = clipTimelineFixture().plan
  return freezeBrowserComposition({
    ownerId: 'alice',
    projectId: 'project',
    projectRevision: 2,
    planRevision: 2,
    plan,
    ratio: 'vertical',
    design: { hideDisclosure: true },
    sources: plan.cuts.map((cut) => ({
      sourceId: cut.sourceId,
      fingerprint: cut.fingerprint,
      durationMs: 20000,
      width,
      height,
      hasAudio: false,
      allowedRatePermille: [1000],
    })),
  })
}
function open() {
  const inputs: OriginalVideoInput[] = [],
    frames: { close: ReturnType<typeof vi.fn> }[] = []
  const factory = vi.fn(async (): Promise<OriginalVideoInput> => {
    const input: OriginalVideoInput = {
      metadata: {
        provenance: 'browser_client',
        codec: 'avc',
        codedWidth: 2,
        codedHeight: 2,
        displayWidth: 2,
        displayHeight: 2,
        timeResolution: 60,
        firstTimestamp: 0,
        durationFromMetadata: 20,
        rotation: 0,
        flip: false,
      },
      samples: (start, end) =>
        (async function* () {
          for (let n = Math.floor(start * 60); n / 60 < end; n++) {
            yield {
              timestamp: n / 60,
              duration: 1 / 60,
              displayWidth: 2,
              displayHeight: 2,
              rotation: 0,
              flip: false,
              close: vi.fn(),
              toVideoFrame: () => {
                const frame = { close: vi.fn(), allocationSize: () => 6 }
                frames.push(frame)
                return frame as unknown as VideoFrame
              },
            }
          }
        })(),
      dispose: vi.fn(),
    }
    inputs.push(input)
    return input
  })
  return { factory, inputs, frames }
}
it('opens lazy independent cut ranges, holds at most2 transitions and releases all source resources', async () => {
  const saved = await snapshot(),
    fake = open(),
    access = vi.fn(async () => ({ kind: 'blob' as const, blob: new Blob(['original']) }))
  const resources = new BrowserFootageResources(saved, access, new AbortController().signal, {
    open: fake.factory,
  })
  expect(access).not.toHaveBeenCalled()
  for (const frame of [0, 1, 294, 295, 300]) {
    const prepared = await resources.prepare(evaluateBrowserFrame(saved, frame))
    expect(prepared.length).toBeLessThanOrEqual(2)
    prepared.forEach(({ resource }) => resource.close())
  }
  expect(access).toHaveBeenCalledTimes(2)
  expect(resources.measurements().peakActiveCuts).toBe(2)
  await resources.dispose()
  expect(fake.inputs.every((input) => vi.mocked(input.dispose).mock.calls.length === 1)).toBe(true)
  expect(fake.frames.every((frame) => frame.close.mock.calls.length === 1)).toBe(true)
  expect(resources.measurements()).toMatchObject({
    activeCuts: 0,
    decoderReservedBytes: 0,
    physicalPeakBytes: null,
    presentation: { liveFrames: 0, liveBytes: 0 },
  })
})
it('refuses excessive original decode reservations before source access or decoder creation', async () => {
  const saved = await snapshot(3840, 2160),
    access = vi.fn(),
    fake = open()
  const resources = new BrowserFootageResources(saved, access, new AbortController().signal, {
    open: fake.factory,
  })
  await expect(resources.prepare(evaluateBrowserFrame(saved, 0))).rejects.toThrow(
    'CLIP_SOURCE_MEMORY_LIMIT',
  )
  expect(access).not.toHaveBeenCalled()
  expect(fake.factory).not.toHaveBeenCalled()
  expect(resources.measurements().decoderReservedBytes).toBe(0)
})
it('stops sibling decoders on failure and cannot publish into a superseded epoch', async () => {
  const saved = await snapshot(),
    fake = open(),
    controller = new AbortController()
  const resources = new BrowserFootageResources(
    saved,
    async () => ({ kind: 'blob', blob: new Blob(['original']) }),
    controller.signal,
    { open: fake.factory },
  )
  const prepared = await resources.prepare(evaluateBrowserFrame(saved, 0))
  await resources.supersede()
  await expect(resources.prepare(evaluateBrowserFrame(saved, 1))).rejects.toThrow()
  expect(prepared[0]!.resource.frame.close).toHaveBeenCalledOnce()
  expect(resources.measurements().presentation.liveFrames).toBe(0)
})
it('closes a late decoder opened after cancellation without negative reservations', async () => {
  const saved = await snapshot(),
    fake = open(),
    controller = new AbortController()
  let finish!: (input: OriginalVideoInput) => void
  const resources = new BrowserFootageResources(
    saved,
    async () => ({ kind: 'blob', blob: new Blob(['original']) }),
    controller.signal,
    {
      open: () =>
        new Promise<OriginalVideoInput>((resolve) => {
          finish = resolve
        }),
    },
  )
  const pending = resources.prepare(evaluateBrowserFrame(saved, 0))
  await Promise.resolve()
  await Promise.resolve()
  controller.abort()
  const late = await fake.factory()
  finish(late)
  await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
  expect(late.dispose).toHaveBeenCalledOnce()
  expect(resources.measurements()).toMatchObject({
    decoderReservedBytes: 0,
    presentation: { liveFrames: 0, liveBytes: 0 },
  })
})
it('rejects unavailable video ranges and metadata without opening sample iterators', async () => {
  const saved = await snapshot(),
    fake = open()
  const candidate = await fake.factory()
  candidate.metadata.durationFromMetadata = 1
  const samples = vi.spyOn(candidate, 'samples')
  const resources = new BrowserFootageResources(
    saved,
    async () => ({ kind: 'blob', blob: new Blob(['original']) }),
    new AbortController().signal,
    { open: async () => candidate },
  )
  await expect(resources.prepare(evaluateBrowserFrame(saved, 0))).rejects.toThrow(
    'CLIP_SOURCE_RANGE_UNSUPPORTED',
  )
  expect(samples).not.toHaveBeenCalled()
  expect(candidate.dispose).toHaveBeenCalledOnce()
})
