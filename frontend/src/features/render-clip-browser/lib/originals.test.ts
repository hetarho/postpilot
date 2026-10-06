import { expect, it, vi } from 'vitest'
import { BrowserOriginals } from './originals'

it('shares one local or authorized remote blob across video and audio readers', async () => {
  const resolve = vi.fn(async () => 'signed-playback')
  const load = vi.fn(async (url: string) => new Blob([url]))
  const originals = new BrowserOriginals(
    [{ fingerprint: 'local', url: 'blob:local' }],
    resolve,
    new AbortController().signal,
    load,
  )
  const [video, audio] = await Promise.all([originals.get('remote'), originals.get('remote')])
  expect(video).toBe(audio)
  expect(resolve).toHaveBeenCalledOnce()
  expect(load).toHaveBeenCalledOnce()
  await originals.get('local')
  expect(resolve).toHaveBeenCalledOnce()
  expect(load.mock.calls[1][0]).toBe('blob:local')
  originals.dispose()
})
it('returns lazy File or refreshed URL access without eagerly downloading an original', async () => {
  const file = new File(['held original'], 'one.mp4'),
    resolve = vi.fn(async () => 'fresh-url'),
    load = vi.fn()
  const originals = new BrowserOriginals(
    [{ fingerprint: 'local', url: 'blob:local', file }],
    resolve,
    new AbortController().signal,
    load,
  )
  expect(await originals.source('local')).toEqual({ kind: 'blob', blob: file })
  const [first, second] = await Promise.all([
    originals.source('remote'),
    originals.source('remote'),
  ])
  expect(first).toBe(second)
  expect(first).toEqual({ kind: 'url', url: 'fresh-url' })
  expect(resolve).toHaveBeenCalledOnce()
  expect(load).not.toHaveBeenCalled()
  originals.dispose()
})
it('never reads bytes after access resolution was cancelled', async () => {
  const controller = new AbortController()
  const load = vi.fn()
  const originals = new BrowserOriginals(
    [],
    async () => {
      controller.abort()
      return 'url'
    },
    controller.signal,
    load,
  )
  await expect(originals.get('remote')).rejects.toMatchObject({ name: 'AbortError' })
  expect(load).not.toHaveBeenCalled()
})

// A server-held original's entry carries whatever URL the page last showed — empty
// until a scene asked for it, or a presigned read that may have expired — so only a
// blob URL is read as local and everything else is resolved afresh.
it('resolves a server-held original rather than reading its stale or empty URL', async () => {
  const resolve = vi.fn(async (fingerprint: string) => `signed-${fingerprint}`)
  const load = vi.fn(async (url: string) => new Blob([url]))
  const originals = new BrowserOriginals(
    [
      { fingerprint: 'unseen', url: '' },
      { fingerprint: 'shown', url: 'http://storage.example/expired?X-Amz-Expires=60' },
      { fingerprint: 'held', url: 'blob:held' },
    ],
    resolve,
    new AbortController().signal,
    load,
  )
  await Promise.all([originals.get('unseen'), originals.get('shown'), originals.get('held')])
  expect(load.mock.calls.map(([url]) => url).sort()).toEqual([
    'blob:held',
    'signed-shown',
    'signed-unseen',
  ])
  expect(resolve.mock.calls.map(([fingerprint]) => fingerprint).sort()).toEqual(['shown', 'unseen'])
})
