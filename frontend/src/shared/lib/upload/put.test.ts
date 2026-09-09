import { afterEach, describe, expect, it, vi } from 'vitest'
import { DirectUploadError, putBlobWithProgress } from './put'

class Request extends EventTarget {
  static last: Request
  upload = new EventTarget()
  status = 200
  withCredentials = true
  open = vi.fn()
  setRequestHeader = vi.fn()
  send = vi.fn()
  abort = vi.fn(() => this.dispatchEvent(new Event('abort')))
  constructor() {
    super()
    Request.last = this
  }
}
afterEach(() => vi.unstubAllGlobals())
describe('shared direct PUT adapter', () => {
  it('sends the original bytes with every signed header and reports bounded progress', async () => {
    vi.stubGlobal('XMLHttpRequest', Request)
    const body = new Blob(['clip'])
    const progress = vi.fn()
    const promise = putBlobWithProgress(
      'https://storage.test/opaque',
      { 'Content-Type': 'video/mp4', 'If-None-Match': '*' },
      body,
      progress,
    )
    const req = Request.last
    expect(req.open).toHaveBeenCalledWith('PUT', 'https://storage.test/opaque')
    expect(req.withCredentials).toBe(false)
    expect(req.setRequestHeader.mock.calls).toEqual([
      ['Content-Type', 'video/mp4'],
      ['If-None-Match', '*'],
    ])
    expect(req.send).toHaveBeenCalledWith(body)
    req.upload.dispatchEvent(
      new ProgressEvent('progress', { lengthComputable: true, loaded: 1, total: 4 }),
    )
    req.upload.dispatchEvent(
      new ProgressEvent('progress', { lengthComputable: true, loaded: 5, total: 4 }),
    )
    req.dispatchEvent(new Event('load'))
    await promise
    expect(progress.mock.calls).toEqual([[25], [100], [100]])
  })
  it.each(['error', 'timeout', 'load'])(
    'classifies %s without leaking the signed URL',
    async (event) => {
      vi.stubGlobal('XMLHttpRequest', Request)
      const promise = putBlobWithProgress(
        'https://storage.test/private-signature',
        {},
        new Blob(),
        vi.fn(),
      )
      Request.last.status = 412
      Request.last.dispatchEvent(new Event(event))
      await expect(promise).rejects.toBeInstanceOf(DirectUploadError)
      await expect(promise).rejects.toThrow('Direct upload failed')
    },
  )
  it('cancels in flight and refuses an already aborted upload', async () => {
    vi.stubGlobal('XMLHttpRequest', Request)
    const controller = new AbortController()
    const promise = putBlobWithProgress(
      'https://storage.test/',
      {},
      new Blob(),
      vi.fn(),
      controller.signal,
    )
    controller.abort()
    await expect(promise).rejects.toMatchObject({ name: 'AbortError' })
    expect(Request.last.abort).toHaveBeenCalledOnce()
    await expect(
      putBlobWithProgress('', {}, new Blob(), vi.fn(), controller.signal),
    ).rejects.toMatchObject({ name: 'AbortError' })
  })
})
