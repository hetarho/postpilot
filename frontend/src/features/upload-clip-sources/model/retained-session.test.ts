import { afterEach, expect, it, vi } from 'vitest'
import type { ClipSourceBatch, ClipSourceMetadata } from '@/entities/clip-project'
import { ClipSourceSession, discardClipSourceSessions, type SourcePipeline } from './session'

afterEach(() => {
  discardClipSourceSessions()
  vi.useRealTimers()
})

function retainedFixture() {
  const metadata: ClipSourceMetadata = {
    filename: 'original.mov',
    contentType: 'video/quicktime',
    fingerprint: 'a'.repeat(64),
    bytes: 100,
    durationMs: 2000,
    width: 1920,
    height: 1080,
  }
  const expiresAt = new Date(Date.now() + 60 * 60 * 1000).toISOString()
  const batch: ClipSourceBatch = {
    id: 'retained',
    projectId: 'project',
    current: true,
    state: 'ready',
    expiresAt,
    sources: [
      {
        id: 'source',
        state: 'ready',
        actualBytes: 100,
        metadata,
        availability: 'available',
        retentionExpiresAt: expiresAt,
      },
    ],
  }
  const pipeline = {
    read: vi.fn(async () => [metadata]),
    reserve: vi.fn(async () => ({
      batch,
      uploads: [{ sourceId: 'source', putUrl: 'https://private.test/put', headers: {} }],
    })),
    put: vi.fn(async () => {}),
    confirm: vi.fn(async () => batch),
    discard: vi.fn(async () => {}),
    retained: vi.fn(async () => [batch]),
    playback: vi.fn<NonNullable<SourcePipeline['playback']>>(async () => ({
      url: 'https://private.test/original?capability=temporary',
      expiresAt: new Date(Date.now() + 60000).toISOString(),
    })),
    createURL: vi.fn(() => 'blob:local-original'),
    revokeURL: vi.fn(),
  } satisfies SourcePipeline
  const session = new ClipSourceSession('project', pipeline)
  session.activate()
  return { session, pipeline, batch, metadata }
}

it('rehydrates a reusable manifest without upload and keeps playback URLs out of the ready batch', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  expect(session.getSnapshot().readyBatch?.id).toBe('retained')
  expect(session.getSnapshot().entries[0]?.file).toBeUndefined()
  expect(session.getSnapshot().entries[0]?.previewURL).toBe('')
  expect(pipeline.read).not.toHaveBeenCalled()
  expect(pipeline.reserve).not.toHaveBeenCalled()
  const [first, second] = await Promise.all([
    session.ensurePlayback(metadata.fingerprint),
    session.ensurePlayback(metadata.fingerprint),
  ])
  expect(first).toBe(second)
  expect(pipeline.playback).toHaveBeenCalledTimes(1)
  expect(pipeline.playback).toHaveBeenCalledWith(
    'project',
    'source',
    metadata.fingerprint,
    expect.any(AbortSignal),
  )
  expect(JSON.stringify(session.getSnapshot().readyBatch)).not.toContain('capability')
  session.dispose()
  expect(session.getSnapshot().entries).toEqual([])
  expect(pipeline.revokeURL).not.toHaveBeenCalled()
})

it('keeps matching local pixels through completion and editable step changes, releasing them on leave', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  const file = new File(['pixels'], metadata.filename, { type: metadata.contentType })
  await session.select([file])
  expect(session.beginAttempt('retained')).toBe(true)
  session.markOwned('retained', 'job')
  session.finishAttempt('job', 'done')
  await session.refreshRetained()
  session.requireSources([metadata.fingerprint])
  expect(await session.ensurePlayback(metadata.fingerprint)).toBe('blob:local-original')
  expect(pipeline.playback).not.toHaveBeenCalled()
  expect(pipeline.revokeURL).not.toHaveBeenCalled()
  expect(session.getSnapshot().readyBatch?.id).toBe('retained')
  session.dispose()
  expect(pipeline.revokeURL).toHaveBeenCalledExactlyOnceWith('blob:local-original')
})

it('refreshes a failed playback capability once and refuses further automatic retries', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  await session.ensurePlayback(metadata.fingerprint)
  pipeline.playback.mockResolvedValueOnce({
    url: 'https://private.test/refreshed',
    expiresAt: new Date(Date.now() + 60000).toISOString(),
  })
  expect(await session.ensurePlayback(metadata.fingerprint, true)).toContain('refreshed')
  await expect(session.ensurePlayback(metadata.fingerprint, true)).rejects.toMatchObject({
    reason: 'unavailable',
  })
  expect(pipeline.playback).toHaveBeenCalledTimes(2)
  expect(session.getSnapshot().entries[0]?.playbackError).toBe('unavailable')
  expect(session.getSnapshot().entries[0]?.previewURL).toBe('')
})

it('distinguishes missing originals and exact retention expiry without fetching another file', async () => {
  vi.useFakeTimers()
  const { session, pipeline, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  batch.sources[0]!.availability = 'missing'
  await session.refreshRetained()
  await expect(session.ensurePlayback(metadata.fingerprint)).rejects.toMatchObject({
    reason: 'missing',
  })
  batch.sources[0]!.availability = 'available'
  vi.setSystemTime(new Date(batch.expiresAt))
  await session.refreshRetained()
  expect(session.getSnapshot().readyBatch).toBeUndefined()
  await expect(session.ensurePlayback(metadata.fingerprint)).rejects.toMatchObject({
    reason: 'expired',
  })
  await expect(session.ensurePlayback('wrong fingerprint')).rejects.toMatchObject({
    reason: 'unavailable',
  })
  expect(pipeline.playback).not.toHaveBeenCalled()
})

it('drops an in-flight playback response when the page session is disposed', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  let resolve!: (value: { url: string; expiresAt: string }) => void
  pipeline.playback.mockImplementation(
    () =>
      new Promise((r) => {
        resolve = r
      }),
  )
  const pending = session.ensurePlayback(metadata.fingerprint)
  session.dispose()
  resolve({
    url: 'https://private.test/stale',
    expiresAt: new Date(Date.now() + 60000).toISOString(),
  })
  await expect(pending).rejects.toMatchObject({ reason: 'unavailable' })
  expect(session.getSnapshot().entries).toEqual([])
})

it('keeps an in-flight playback request valid across metadata refresh and accepted work', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  let resolve!: (value: { url: string; expiresAt: string }) => void
  let signal: AbortSignal | undefined
  pipeline.playback.mockImplementation((...args) => {
    signal = args[3]
    return new Promise((r) => {
      resolve = r
    })
  })
  const pending = session.ensurePlayback(metadata.fingerprint)
  session.requireSources([metadata.fingerprint])
  await session.refreshRetained()
  expect(signal?.aborted).toBe(false)
  expect(session.beginAttempt('retained')).toBe(true)
  session.markOwned('retained', 'job')
  resolve({
    url: 'https://private.test/live',
    expiresAt: new Date(Date.now() + 60000).toISOString(),
  })
  expect(await pending).toBe('https://private.test/live')
  expect(session.getSnapshot().phase).toBe('owned')
  expect(session.getSnapshot().entries[0]?.previewURL).toContain('/live')
})
