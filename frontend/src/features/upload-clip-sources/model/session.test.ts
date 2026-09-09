import { describe, expect, it, vi } from 'vitest'
import type { ClipSourceBatch, ClipSourceMetadata } from '@/entities/clip-project'
import { ClipSourceSession, type SourcePipeline } from './session'

const files = [
  new File(['one'], 'one.mp4', { type: 'video/mp4' }),
  new File(['two'], 'two.mp4', { type: 'video/mp4' }),
]
const manifest: ClipSourceMetadata[] = files.map((f, i) => ({
  filename: f.name,
  bytes: f.size,
  contentType: f.type,
  durationMs: 1000,
  width: 1920,
  height: 1080,
  fingerprint: String(i).repeat(64),
}))
function fixture() {
  const batch: ClipSourceBatch = {
    id: 'batch',
    projectId: 'project',
    state: 'uploading',
    expiresAt: '2099-01-01T00:00:00Z',
    sources: manifest.map((metadata, i) => ({
      id: `source-${i}`,
      state: 'pending',
      actualBytes: 0,
      metadata,
    })),
  }
  const reservation = {
    batch,
    uploads: batch.sources.map((s) => ({
      sourceId: s.id,
      putUrl: `https://storage.test/${s.id}`,
      headers: { 'If-None-Match': '*', 'Content-Type': 'video/mp4' },
    })),
  }
  const pipeline = {
    read: vi.fn(async () => manifest),
    reserve: vi.fn(async () => reservation),
    put: vi.fn<SourcePipeline['put']>(async (_u, _h, _f, progress) => {
      progress(25)
    }),
    confirm: vi.fn(async (_id, sourceId) => {
      batch.sources = batch.sources.map((s) =>
        s.id === sourceId ? { ...s, state: 'ready', actualBytes: s.metadata.bytes } : s,
      )
      if (batch.sources.every((s) => s.state === 'ready')) batch.state = 'ready'
      return { ...batch }
    }),
    discard: vi.fn(async () => {}),
    createURL: vi.fn((f: File) => `blob:${f.name}`),
    revokeURL: vi.fn(),
  } satisfies SourcePipeline
  const session = new ClipSourceSession('project', pipeline)
  session.activate()
  return { session, pipeline, reservation }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((r) => {
    resolve = r
  })
  return { promise, resolve }
}
describe('page-owned clip source session', () => {
  it('reserves metadata once, uploads in order, confirms each and exposes only a typed ready batch', async () => {
    const { session, pipeline } = fixture()
    const snapshots: number[] = []
    session.subscribe(() => snapshots.push(session.getSnapshot().entries[0]?.percent ?? 0))
    await session.select(files)
    expect(pipeline.reserve).toHaveBeenCalledExactlyOnceWith('project', manifest)
    expect(pipeline.put.mock.calls.map((c) => c[2])).toEqual(files)
    expect(pipeline.put.mock.calls[0]?.[1]).toEqual({
      'If-None-Match': '*',
      'Content-Type': 'video/mp4',
    })
    expect(pipeline.confirm.mock.calls).toEqual([
      ['batch', 'source-0'],
      ['batch', 'source-1'],
    ])
    expect(snapshots).toContain(25)
    expect(session.getSnapshot()).toMatchObject({
      phase: 'ready',
      readyBatch: { state: 'ready' },
      entries: [
        { percent: 100, confirmed: true },
        { percent: 100, confirmed: true },
      ],
    })
    expect(JSON.stringify(session.getSnapshot().readyBatch)).not.toContain('blob:')
    session.finishAttempt()
    expect(pipeline.revokeURL.mock.calls).toEqual([['blob:one.mp4'], ['blob:two.mp4']])
    expect(pipeline.discard).not.toHaveBeenCalled()
    expect(session.getSnapshot()).toEqual({ phase: 'idle', entries: [] })
  })
  it('rejects a client gate before reservation or previews', async () => {
    const { session, pipeline } = fixture()
    pipeline.read.mockRejectedValueOnce(new Error('limit'))
    await session.select(files)
    expect(session.getSnapshot().phase).toBe('failed')
    expect(pipeline.reserve).not.toHaveBeenCalled()
    expect(pipeline.createURL).not.toHaveBeenCalled()
  })
  it.each(['put', 'confirm'] as const)(
    'cleans up a partial %s failure without losing project state',
    async (method) => {
      const { session, pipeline } = fixture()
      if (method === 'put')
        pipeline.put.mockResolvedValueOnce().mockRejectedValueOnce(new Error('network'))
      else
        pipeline.confirm
          .mockResolvedValueOnce({ ...fixture().reservation.batch })
          .mockRejectedValueOnce(new Error('confirm'))
      await session.select(files)
      expect(session.getSnapshot()).toMatchObject({
        phase: 'failed',
        entries: [],
        error: expect.any(Error),
      })
      expect(pipeline.discard).toHaveBeenCalledExactlyOnceWith('batch')
      expect(pipeline.revokeURL).toHaveBeenCalledTimes(2)
    },
  )
  it('retries a failed discard before allowing another reservation', async () => {
    const { session, pipeline } = fixture()
    await session.select(files)
    pipeline.discard.mockRejectedValueOnce(new Error('offline'))
    await session.cancel()
    expect(session.getSnapshot().phase).toBe('failed')
    expect(session.getSnapshot().entries).toEqual([])
    await session.select(files)
    expect(pipeline.discard).toHaveBeenCalledTimes(2)
    expect(pipeline.reserve).toHaveBeenCalledTimes(2)
    await session.cancel()
    expect(session.getSnapshot().phase).toBe('idle')
  })
  it('discards the old selection before replacing it', async () => {
    const { session, pipeline } = fixture()
    await session.select(files)
    await session.select(files)
    expect(pipeline.discard).toHaveBeenCalledWith('batch')
    expect(pipeline.discard.mock.invocationCallOrder[0]).toBeLessThan(
      pipeline.reserve.mock.invocationCallOrder[1]!,
    )
    expect(pipeline.revokeURL).toHaveBeenCalledTimes(2)
  })
  it('aborts the actual upload and discards when explicitly cancelled', async () => {
    const { session, pipeline } = fixture()
    const started = deferred<AbortSignal>()
    pipeline.put.mockImplementation(
      (_u, _h, _f, _p, signal) =>
        new Promise((_resolve, reject) => {
          started.resolve(signal)
          signal.addEventListener('abort', () => reject(new DOMException('', 'AbortError')))
        }),
    )
    const attempt = session.select(files)
    const signal = await started.promise
    await session.cancel()
    await attempt
    expect(signal.aborted).toBe(true)
    expect(pipeline.discard).toHaveBeenCalledExactlyOnceWith('batch')
    expect(pipeline.confirm).not.toHaveBeenCalled()
  })
  it.each([false, true])(
    'handles a late reservation after cancellation/unmount (unmount=%s)',
    async (unmount) => {
      const { session, pipeline, reservation } = fixture()
      const started = deferred<void>()
      const response = deferred<typeof reservation>()
      pipeline.reserve.mockImplementation(() => {
        started.resolve()
        return response.promise
      })
      const attempt = session.select(files)
      await started.promise
      if (unmount) session.dispose()
      else await session.cancel()
      response.resolve(reservation)
      await attempt
      expect(pipeline.discard).toHaveBeenCalledTimes(unmount ? 0 : 1)
      expect(pipeline.put).not.toHaveBeenCalled()
      expect(pipeline.revokeURL).toHaveBeenCalledTimes(2)
      expect(session.getSnapshot()).toEqual({ phase: 'idle', entries: [] })
    },
  )
  it('unmounts a ready batch without an asynchronous delete and starts fresh on remount', async () => {
    const { session, pipeline } = fixture()
    await session.select(files)
    session.dispose()
    session.activate()
    expect(pipeline.discard).not.toHaveBeenCalled()
    expect(pipeline.revokeURL).toHaveBeenCalledTimes(2)
    expect(session.getSnapshot()).toEqual({ phase: 'idle', entries: [] })
  })
})
