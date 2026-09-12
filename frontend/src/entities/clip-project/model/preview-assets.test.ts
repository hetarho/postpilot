import { afterEach, expect, it, vi } from 'vitest'
import { PreviewAssetCache, PreviewPreparation } from './preview-assets'
import type { PreviewAsset, PreviewPage } from './draft-preview'

afterEach(() => vi.useRealTimers())
const asset = (key = 'glyph', size = 4): PreviewAsset => ({
  key,
  instanceId: 'text',
  png: new Uint8Array(size),
  x: 0,
  y: 0,
  width: 1,
  height: 1,
  startMs: 0,
  endMs: 300,
  inMs: 0,
  outMs: 0,
  dy: 0,
  layer: 0,
})
const page = (hash: string, assets = [asset()]): PreviewPage => ({
  draftHash: hash,
  canvasWidth: 1080,
  canvasHeight: 1920,
  assets,
  nextOffset: -1,
  parity: ['frameTiming'],
})
function setup(limit?: number) {
  const urls = {
    create: vi.fn((bytes: Uint8Array) => `blob:${bytes.length}:${Math.random()}`),
    revoke: vi.fn(),
  }
  const cache = new PreviewAssetCache(urls, limit)
  return { cache, urls, preparation: new PreviewPreparation(cache) }
}

it('reuses glyph assets across trim-only manifests and revokes on LRU eviction and disposal', () => {
  const { cache, urls } = setup(8)
  const first = cache.put(asset('a'))
  expect(cache.put({ ...asset('a'), startMs: 500 }).url).toBe(first.url)
  cache.put(asset('b'))
  cache.put(asset('c'))
  expect(urls.create).toHaveBeenCalledTimes(3)
  expect(urls.revoke).toHaveBeenCalledWith(first.url)
  cache.clear()
  expect(urls.revoke).toHaveBeenCalledTimes(3)
})

it('debounces rapid edits and rejects a stale in-flight response even when abort is ignored', async () => {
  vi.useFakeTimers()
  const { preparation, urls } = setup()
  let finish: (page: PreviewPage) => void = () => {}
  const old = vi.fn((_ids: string[], _offset: number, signal: AbortSignal) => {
    void signal
    return new Promise<PreviewPage>((resolve) => {
      finish = resolve
    })
  })
  const latest = vi.fn(async () => page('new', [asset('new')]))
  preparation.update('old', 'old', ['text'], old)
  await vi.advanceTimersByTimeAsync(250)
  const oldSignal = old.mock.calls[0]?.[2]
  preparation.update('new', 'new', ['text'], latest)
  finish(page('old'))
  await Promise.resolve()
  expect(preparation.getSnapshot().assets).toEqual([])
  expect(oldSignal?.aborted).toBe(true)
  await vi.advanceTimersByTimeAsync(249)
  expect(latest).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(1)
  expect(preparation.getSnapshot()).toMatchObject({ key: 'new', status: 'ready' })
  expect(preparation.getSnapshot().assets[0]?.key).toBe('new')
  preparation.dispose()
  expect(urls.revoke).toHaveBeenCalledTimes(1)
})

it('never paints a server response with a mismatched body hash', async () => {
  vi.useFakeTimers()
  const { preparation, urls } = setup()
  preparation.update('edit', 'expected', [], async () => page('different'))
  await vi.advanceTimersByTimeAsync(250)
  expect(preparation.getSnapshot()).toMatchObject({ status: 'failed', assets: [] })
  expect(urls.create).not.toHaveBeenCalled()
  preparation.dispose()
})

it('keeps the already prepared current cut visible while prefetching the next cut of the same draft', async () => {
  vi.useFakeTimers()
  const { preparation } = setup()
  preparation.update('cut-a', 'same-body', ['text'], async () => page('same-body'), 'draft')
  await vi.advanceTimersByTimeAsync(250)
  const current = preparation.getSnapshot().assets[0]?.url
  preparation.update('cut-b', 'same-body', ['text', 'next'], async () => page('same-body'), 'draft')
  expect(preparation.getSnapshot()).toMatchObject({ status: 'updating', contentKey: 'draft' })
  expect(preparation.getSnapshot().assets[0]?.url).toBe(current)
  preparation.update('edited', 'new-body', ['text'], async () => page('new-body'), 'edited-draft')
  expect(preparation.getSnapshot().assets).toEqual([])
  preparation.dispose()
})

it('pages rapid cue assets and element selections without exceeding the RPC page', async () => {
  vi.useFakeTimers()
  const { preparation } = setup()
  const load = vi.fn(async (ids: string[], offset: number) => ({
    ...page('hash', [asset(`${ids[0]}-${offset}`)]),
    nextOffset: ids.length === 8 && offset === 0 ? 8 : -1,
  }))
  preparation.update(
    'edit',
    'hash',
    Array.from({ length: 10 }, (_, i) => `id${i}`),
    load,
  )
  await vi.advanceTimersByTimeAsync(250)
  expect(load.mock.calls.map(([ids, offset]) => [ids.length, offset])).toEqual([
    [8, 0],
    [8, 8],
    [2, 0],
  ])
  expect(preparation.getSnapshot().assets).toHaveLength(3)
  preparation.dispose()
})

it('returns explicit preparation failure for an oversize page without allocating URLs', async () => {
  vi.useFakeTimers()
  const { preparation, urls } = setup()
  preparation.update('edit', 'hash', [], async () =>
    page('hash', [asset('too-big', 512 * 1024 + 1)]),
  )
  await vi.advanceTimersByTimeAsync(250)
  expect(preparation.getSnapshot()).toMatchObject({ status: 'failed', assets: [] })
  expect(urls.create).not.toHaveBeenCalled()
  preparation.dispose()
})
