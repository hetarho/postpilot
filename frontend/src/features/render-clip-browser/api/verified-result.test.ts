import { expect, it, vi } from 'vitest'
import type { ClipProject } from '@/entities/clip-project'
import {
  storeVerifiedBrowserResult,
  type BrowserResultStore,
  type VerifiedBrowserResult,
} from './store-result'

const fixture = () => {
  const result: VerifiedBrowserResult = {
    renderId: 'render',
    file: new Blob(['verified MP4']),
    verdict: { passed: true, measurements: {} } as VerifiedBrowserResult['verdict'],
    dispose: vi.fn(async () => {}),
  }
  const project = { id: 'project', result: { id: 'durable' } } as ClipProject
  const store: BrowserResultStore = {
    prepare: vi.fn(async () => ({ putUrl: 'private-owner-upload', headers: {} })),
    put: vi.fn(async () => {}),
    report: vi.fn(async () => ({ passed: true, notices: [] })),
    complete: vi.fn(async () => project),
  }
  return { result, project, store, signal: new AbortController().signal }
}
it('retries a failed PUT with identical valid bytes and retains local ownership', async () => {
  const f = fixture()
  vi.mocked(f.store.put).mockRejectedValueOnce(new Error('network failed'))
  await expect(storeVerifiedBrowserResult(f.result, f.store, f.signal)).rejects.toThrow(
    'network failed',
  )
  expect(f.store.report).not.toHaveBeenCalled()
  expect(f.result.dispose).not.toHaveBeenCalled()
  expect(await storeVerifiedBrowserResult(f.result, f.store, f.signal)).toBe(f.project)
  expect(vi.mocked(f.store.put).mock.calls.map((call) => call[2])).toEqual([
    f.result.file,
    f.result.file,
  ])
})
it('retries report and idempotent completion without reserving or putting uploaded bytes again', async () => {
  const f = fixture()
  vi.mocked(f.store.report).mockRejectedValueOnce(new Error('lost report response'))
  await expect(storeVerifiedBrowserResult(f.result, f.store, f.signal)).rejects.toThrow(
    'lost report response',
  )
  vi.mocked(f.store.complete).mockRejectedValueOnce(new Error('lost completion response'))
  await expect(storeVerifiedBrowserResult(f.result, f.store, f.signal)).rejects.toThrow(
    'lost completion response',
  )
  expect(await storeVerifiedBrowserResult(f.result, f.store, f.signal)).toBe(f.project)
  expect(f.store.prepare).toHaveBeenCalledTimes(1)
  expect(f.store.put).toHaveBeenCalledTimes(1)
  expect(f.store.report).toHaveBeenCalledTimes(2)
  expect(f.store.complete).toHaveBeenCalledTimes(2)
})
it('cannot report or promote when cancelled PUT resolves late', async () => {
  const f = fixture(),
    controller = new AbortController()
  vi.mocked(f.store.put).mockImplementation(async () => {
    controller.abort(new Error('left page'))
  })
  await expect(storeVerifiedBrowserResult(f.result, f.store, controller.signal)).rejects.toThrow(
    'left page',
  )
  expect(f.store.report).not.toHaveBeenCalled()
  expect(f.store.complete).not.toHaveBeenCalled()
})
it('refuses failed local or authoritative verdict without promoting a provisional result', async () => {
  const f = fixture()
  f.result.verdict.passed = false
  await expect(storeVerifiedBrowserResult(f.result, f.store, f.signal)).rejects.toThrow(
    'Browser output failed validation',
  )
  expect(f.store.prepare).not.toHaveBeenCalled()
  f.result.verdict.passed = true
  vi.mocked(f.store.report).mockResolvedValue({
    passed: false,
    notices: [{ code: 'stale', action: 'correct' }],
  })
  await expect(storeVerifiedBrowserResult(f.result, f.store, f.signal)).rejects.toThrow(
    'Browser output failed validation',
  )
  expect(f.store.complete).not.toHaveBeenCalled()
})
