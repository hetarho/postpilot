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
        retainOriginalAudio: false,
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

it('waits for initial retained metadata before a reopened preview requests playback', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  const [first, second] = await Promise.all([
    session.ensurePlayback(metadata.fingerprint),
    session.ensurePlayback(metadata.fingerprint),
  ])
  expect(first).toContain('capability=temporary')
  expect(second).toBe(first)
  expect(pipeline.retained).toHaveBeenCalledTimes(1)
  expect(pipeline.playback).toHaveBeenCalledTimes(1)
})

it('does not acquire playback if the page leaves while retained metadata loads', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  const pending = session.ensurePlayback(metadata.fingerprint)
  session.dispose()
  await expect(pending).rejects.toMatchObject({ reason: 'unavailable' })
  expect(pipeline.playback).not.toHaveBeenCalled()
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

it('projects default/reopened/inherited sound and applies only an owned matching batch acknowledgment', async () => {
  const { session, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  expect(session.getSnapshot().entries[0]).toMatchObject({
    batchId: batch.id,
    current: true,
    retainOriginalAudio: false,
  })
  batch.sources[0].retainOriginalAudio = true
  await session.refreshRetained()
  expect(session.getSnapshot().entries[0].retainOriginalAudio).toBe(true)
  await session.select([new File(['video'], metadata.filename)])
  expect(session.getSnapshot().entries[0]).toMatchObject({
    sourceId: 'source',
    retainOriginalAudio: true,
  })
  const changed = structuredClone(batch)
  changed.sources[0].retainOriginalAudio = false
  session.acceptSoundBatch({ ...changed, projectId: 'foreign' })
  expect(session.getSnapshot().entries[0].retainOriginalAudio).toBe(true)
  session.acceptSoundBatch(changed)
  expect(session.getSnapshot().entries[0].retainOriginalAudio).toBe(false)
  expect(session.getSnapshot().readyBatch?.sources[0].retainOriginalAudio).toBe(false)
  expect(session.getSnapshot().entries[0].previewURL).toBe('blob:local-original')
})

it('does not let a retained read started before a sound acknowledgment restore the old setting', async () => {
  const { session, pipeline, batch } = retainedFixture()
  await session.refreshRetained()
  const old = structuredClone(batch)
  let finish!: (value: ClipSourceBatch[]) => void
  pipeline.retained.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const read = session.refreshRetained()
  const changed = structuredClone(batch)
  changed.sources[0].retainOriginalAudio = true
  session.acceptSoundBatch(changed)
  finish([old])
  await read
  expect(session.getSnapshot().entries[0].retainOriginalAudio).toBe(true)
  expect(session.getSnapshot().readyBatch?.sources[0].retainOriginalAudio).toBe(true)
})

it('publishes an authoritative first capability and preserves it across identical metadata and URL refreshes', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  expect(session.getSnapshot().entries[0]?.playbackGeneration).toBe(0)
  const url = await session.ensurePlayback(metadata.fingerprint)
  expect(session.getSnapshot().entries[0]?.playbackGeneration).toBe(1)
  await session.refreshRetained()
  expect(await session.ensurePlayback(metadata.fingerprint)).toBe(url)
  expect(pipeline.playback).toHaveBeenCalledTimes(1)
  expect(session.getSnapshot().entries[0]).toMatchObject({ previewURL: url, playbackGeneration: 1 })
  await session.ensurePlayback(metadata.fingerprint, true)
  expect(pipeline.playback).toHaveBeenCalledTimes(2)
  expect(session.getSnapshot().entries[0]?.playbackGeneration).toBe(1)
})

it('records acquisition and a distinct renewal even if the consumer coalesces their publications', async () => {
  const { session, pipeline, metadata } = retainedFixture()
  await session.refreshRetained()
  const before = session.getSnapshot()
  await session.ensurePlayback(metadata.fingerprint)
  pipeline.playback.mockResolvedValueOnce({
    url: 'https://private.test/new',
    expiresAt: new Date(Date.now() + 60000).toISOString(),
  })
  await session.ensurePlayback(metadata.fingerprint, true)
  expect(before.entries[0]?.playbackGeneration).toBe(0)
  expect(session.getSnapshot().entries[0]).toMatchObject({
    previewURL: 'https://private.test/new',
    playbackGeneration: 2,
  })
  await session.refreshRetained()
  expect(session.getSnapshot().entries[0]?.playbackGeneration).toBe(2)
})

it('increments withdrawal once and preserves the latest metadata through repeated quota refusals', async () => {
  const { session, pipeline, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  await session.ensurePlayback(metadata.fingerprint)
  await session.ensurePlayback(metadata.fingerprint, true)
  batch.sources[0]!.retainOriginalAudio = true
  await session.refreshRetained()
  await expect(session.ensurePlayback(metadata.fingerprint, true)).rejects.toMatchObject({
    reason: 'unavailable',
  })
  expect(session.getSnapshot().entries[0]).toMatchObject({
    previewURL: '',
    playbackGeneration: 2,
    playbackError: 'unavailable',
    retainOriginalAudio: true,
  })
  await expect(session.ensurePlayback(metadata.fingerprint, true)).rejects.toMatchObject({
    reason: 'unavailable',
  })
  expect(session.getSnapshot().entries[0]?.playbackGeneration).toBe(2)
  expect(pipeline.playback).toHaveBeenCalledTimes(2)
})

it('does not overwrite current metadata when an older in-flight capability fails', async () => {
  const { session, pipeline, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  let fail!: (error: unknown) => void
  pipeline.playback.mockImplementationOnce(
    () =>
      new Promise((_resolve, reject) => {
        fail = reject
      }),
  )
  const pending = session.ensurePlayback(metadata.fingerprint)
  batch.sources[0]!.retainOriginalAudio = true
  await session.refreshRetained()
  fail(new Error('unavailable'))
  await expect(pending).rejects.toMatchObject({ reason: 'unavailable' })
  expect(session.getSnapshot().entries[0]).toMatchObject({
    retainOriginalAudio: true,
    playbackGeneration: 2,
    playbackError: 'unavailable',
  })
})

it.each(['missing', 'expired', 'cleanup_pending', 'retention'] as const)(
  'withdraws a cached capability on existing retained authority refusal: %s',
  async (state) => {
    const { session, pipeline, batch, metadata } = retainedFixture()
    await session.refreshRetained()
    await session.ensurePlayback(metadata.fingerprint)
    if (state === 'retention')
      batch.sources[0]!.retentionExpiresAt = new Date(Date.now() - 1).toISOString()
    else batch.sources[0]!.availability = state
    await session.refreshRetained()
    const reason = state === 'missing' ? 'missing' : 'expired'
    await expect(session.ensurePlayback(metadata.fingerprint)).rejects.toMatchObject({ reason })
    expect(session.getSnapshot().entries[0]).toMatchObject({
      previewURL: '',
      playbackError: reason,
      playbackGeneration: 2,
    })
    await expect(session.ensurePlayback(metadata.fingerprint)).rejects.toMatchObject({ reason })
    expect(session.getSnapshot().entries[0]?.playbackGeneration).toBe(2)
    expect(pipeline.playback).toHaveBeenCalledOnce()
  },
)

it('rejects pending access after retained authority withdrawal without resurrecting metadata or URL', async () => {
  const { session, pipeline, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  let finish!: (value: { url: string; expiresAt: string }) => void
  pipeline.playback.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const pending = session.ensurePlayback(metadata.fingerprint)
  batch.sources[0]!.availability = 'missing'
  batch.sources[0]!.retainOriginalAudio = true
  await session.refreshRetained()
  await expect(session.ensurePlayback(metadata.fingerprint)).rejects.toMatchObject({
    reason: 'missing',
  })
  finish({
    url: 'https://private.test/too-late',
    expiresAt: new Date(Date.now() + 60000).toISOString(),
  })
  await expect(pending).rejects.toMatchObject({ reason: 'missing' })
  expect(session.getSnapshot().entries[0]).toMatchObject({
    availability: 'missing',
    retainOriginalAudio: true,
    previewURL: '',
    playbackGeneration: 2,
    playbackError: 'missing',
  })
  expect(pipeline.playback).toHaveBeenCalledOnce()
})

it('cannot overwrite a replacement source entry with an old source response or failure', async () => {
  const { session, pipeline, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  let finish!: (value: { url: string; expiresAt: string }) => void
  pipeline.playback.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const pending = session.ensurePlayback(metadata.fingerprint)
  batch.sources[0]!.id = 'replacement'
  await session.refreshRetained()
  finish({ url: 'https://private.test/old', expiresAt: new Date(Date.now() + 60000).toISOString() })
  await expect(pending).rejects.toMatchObject({ reason: 'unavailable' })
  expect(session.getSnapshot().entries[0]).toMatchObject({
    sourceId: 'replacement',
    previewURL: '',
    playbackGeneration: 0,
  })
  expect(session.getSnapshot().entries[0]?.playbackError).toBeUndefined()
})

it('keeps local File precedence over retained expiry and preserves conservative legacy generation', async () => {
  const { session, pipeline, batch, metadata } = retainedFixture()
  await session.refreshRetained()
  await session.ensurePlayback(metadata.fingerprint)
  delete session.getSnapshot().entries[0]!.playbackGeneration
  pipeline.playback.mockResolvedValueOnce({
    url: 'https://private.test/legacy-renewed',
    expiresAt: new Date(Date.now() + 60000).toISOString(),
  })
  await session.ensurePlayback(metadata.fingerprint, true)
  expect(session.getSnapshot().entries[0]?.playbackGeneration).toBeUndefined()
  await session.select([new File(['pixels'], metadata.filename)])
  batch.sources[0]!.availability = 'missing'
  await session.refreshRetained()
  expect(await session.ensurePlayback(metadata.fingerprint)).toBe('blob:local-original')
  expect(pipeline.playback).toHaveBeenCalledTimes(2)
})
