import { describe, expect, it, vi } from 'vitest'
import { freezeBrowserPreviewComposition } from './browser-composition'
import { BrowserCompositionOriginals } from './composition-originals'
const snapshot = () =>
  freezeBrowserPreviewComposition({
    ownerId: 'owner',
    projectId: 'project',
    projectRevision: 1,
    planRevision: 1,
    ratio: 'vertical',
    design: { hideDisclosure: true },
    plan: {
      nativeComposition: true,
      elements: [],
      durationMs: 15000,
      cuts: [
        {
          id: 'cut',
          sourceId: 'source',
          fingerprint: 'a'.repeat(64),
          startMs: 0,
          endMs: 15000,
          transitionMs: 0,
          volumePermille: 0,
          playbackRatePermille: 1000,
          copies: [],
        },
      ],
    },
    sources: [
      {
        sourceId: 'source',
        fingerprint: 'a'.repeat(64),
        durationMs: 15000,
        width: 320,
        height: 180,
        allowedRatePermille: [1000],
      },
    ],
  })
describe('owned original runtime access', () => {
  it('cannot use a File from another source id, even with the same fingerprint', async () => {
    const load = vi.fn(async () => 'https://authorized.test/original'),
      owner = new AbortController()
    const originals = new BrowserCompositionOriginals(
      await snapshot(),
      [{ sourceId: 'other', fingerprint: 'a'.repeat(64), file: new File(['wrong'], 'same.mp4') }],
      load,
      owner.signal,
    )
    expect(await originals.source('source', 'a'.repeat(64), new AbortController().signal)).toEqual({
      kind: 'url',
      url: 'https://authorized.test/original',
    })
    expect(load).toHaveBeenCalledOnce()
    originals.dispose()
  })
  it('fences an old seek without poisoning compatible authorized access for the new seek', async () => {
    let answer!: (url: string) => void
    const load = vi.fn(
        () =>
          new Promise<string>((resolve) => {
            answer = resolve
          }),
      ),
      life = new AbortController(),
      before = new AbortController(),
      after = new AbortController()
    const originals = new BrowserCompositionOriginals(await snapshot(), [], load, life.signal)
    const old = originals.source('source', 'a'.repeat(64), before.signal)
    before.abort()
    const current = originals.source('source', 'a'.repeat(64), after.signal)
    answer('https://authorized.test/current')
    await expect(old).rejects.toHaveProperty('name', 'AbortError')
    expect(await current).toEqual({ kind: 'url', url: 'https://authorized.test/current' })
    expect(load).toHaveBeenCalledOnce()
    originals.dispose()
  })
  it('keeps expired/missing names and rejects late ownership callbacks after disposal', async () => {
    const life = new AbortController()
    const missing = new BrowserCompositionOriginals(
      await snapshot(),
      [],
      async () => {
        throw { reason: 'missing' }
      },
      life.signal,
    )
    await expect(
      missing.source('source', 'a'.repeat(64), new AbortController().signal),
    ).rejects.toMatchObject({ code: 'CLIP_SOURCE_MISSING' })
    let answer!: (url: string) => void
    const late = new BrowserCompositionOriginals(
      await snapshot(),
      [],
      () =>
        new Promise((resolve) => {
          answer = resolve
        }),
      life.signal,
    )
    const pending = late.source('source', 'a'.repeat(64), new AbortController().signal)
    late.dispose()
    answer('https://authorized.test/old')
    await expect(pending).rejects.toMatchObject({ code: 'CLIP_SNAPSHOT_SUPERSEDED' })
  })
})
