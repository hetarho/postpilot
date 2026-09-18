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
