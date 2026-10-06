import { afterEach, describe, expect, it, vi } from 'vitest'
import { webcrypto } from 'node:crypto'
import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import {
  freezeBrowserComposition,
  freezeBrowserPreviewComposition,
  type BrowserCompositionInput,
} from './browser-composition'
import { compositeBrowserVideo } from './composite-video'

const input = (): BrowserCompositionInput => ({
  ownerId: 'owner',
  projectId: 'project',
  projectRevision: 1,
  planRevision: 1,
  ratio: 'vertical',
  design: { hideDisclosure: true },
  sources: [
    {
      sourceId: 'source',
      fingerprint: 'a'.repeat(64),
      durationMs: 1000,
      width: 16,
      height: 16,
      allowedRatePermille: [1000],
    },
  ],
  plan: {
    nativeComposition: true,
    durationMs: 1000,
    elements: [],
    cuts: [
      {
        id: 'cut',
        sourceId: 'source',
        fingerprint: 'a'.repeat(64),
        startMs: 0,
        endMs: 1000,
        transitionMs: 0,
        volumePermille: 0,
        playbackRatePermille: 1000,
        copies: [],
      },
    ],
  },
})
const ports = () => ({
  context: { globalAlpha: 1, fillStyle: '', fillRect: vi.fn(), drawImage: vi.fn() },
  source: vi.fn(async () => ({ width: 16, height: 16, close: vi.fn() }) as unknown as ImageBitmap),
  asset: vi.fn(),
  captionFrame: vi.fn(),
  releaseAssets: vi.fn(),
  encode: vi.fn(async () => {}),
  progress: vi.fn(),
})
afterEach(() => vi.unstubAllGlobals())
describe('strict export purpose boundary', () => {
  it('rejects a real weak preview snapshot before drawing, source access or encoding', async () => {
    vi.stubGlobal('crypto', webcrypto)
    const snapshot = await freezeBrowserPreviewComposition(input()),
      collaborators = ports()
    expect(snapshot.sources[0]!.hasAudio).toBeUndefined()
    await expect(
      compositeBrowserVideo(
        { ratio: 'vertical', plan: snapshot.plan as ClipEditPlan, assets: [], snapshot },
        collaborators,
      ),
    ).rejects.toThrow('CLIP_SNAPSHOT_EXPORT_PURPOSE_REQUIRED')
    expect(collaborators.context.fillRect).not.toHaveBeenCalled()
    expect(collaborators.source).not.toHaveBeenCalled()
    expect(collaborators.encode).not.toHaveBeenCalled()
  })
  it('keeps explicit complete export snapshots on the full30-frame output clock', async () => {
    vi.stubGlobal('crypto', webcrypto)
    const value = input()
    value.sources[0]!.hasAudio = false
    const snapshot = await freezeBrowserComposition(value),
      collaborators = ports()
    expect(
      await compositeBrowserVideo(
        { ratio: 'vertical', plan: snapshot.plan as ClipEditPlan, assets: [], snapshot },
        collaborators,
      ),
    ).toEqual({ frameCount: 30, durationUs: 1000000 })
    expect(collaborators.encode).toHaveBeenCalledTimes(30)
  })
})
