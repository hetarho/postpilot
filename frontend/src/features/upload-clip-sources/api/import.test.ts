import { File as NodeFile } from 'node:buffer'
import { webcrypto } from 'node:crypto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clipSourceCalls } from '@/entities/clip-project'
import { readVideoMetadata } from '@/shared/lib/media'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { createFakeAuthTransport } from '@/test/session'
import type { FakeClipsOptions } from '@/test/clips'
import { fetchClipSourceFile, transferClipSourceFiles, type ClipImportSource } from './import'

vi.mock('@/shared/lib/media', async (original) => ({
  ...(await original<object>()),
  readVideoMetadata: vi.fn(),
}))
vi.mock('@/shared/lib/upload', () => ({ putBlobWithProgress: vi.fn(async () => {}) }))
beforeEach(() => {
  vi.stubGlobal('File', NodeFile)
  vi.stubGlobal('crypto', webcrypto)
  vi.mocked(readVideoMetadata).mockResolvedValue({ durationMs: 1000, width: 1920, height: 1080 })
  vi.mocked(putBlobWithProgress).mockResolvedValue(undefined)
})
afterEach(() => {
  vi.resetAllMocks()
  vi.unstubAllGlobals()
})
function fixture(options: FakeClipsOptions = {}) {
  const requests: unknown[] = [],
    names: string[] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    clips: {
      projects: [
        {
          id: 'clip',
          title: 'visit',
          videoTemplateId: '',
          ratio: 'vertical',
          targetDurationMs: 30000,
          disclosure: 'ad',
        },
      ],
      sourceRequests: requests,
      calls: names,
      ...options,
    },
  })
  return { calls: clipSourceCalls(transport), requests, names }
}
const source = (filename: string, content = filename): ClipImportSource => ({
  filename,
  load: vi.fn(
    async () => new NodeFile([content], filename, { type: 'video/mp4' }) as unknown as File,
  ),
})

describe('confirmed source import', () => {
  it('sends a fetched blob through the existing manifest, fingerprint and upload pipeline', async () => {
    const f = fixture()
    const fetch = vi.fn(
      async () => new Response('owned video', { headers: { 'content-type': 'video/mp4' } }),
    )
    vi.stubGlobal('fetch', fetch)
    const result = await transferClipSourceFiles(
      'clip',
      [
        {
          filename: 'owned.mp4',
          load: (signal) =>
            fetchClipSourceFile(
              {
                filename: 'owned.mp4',
                contentType: 'video/mp4',
                url: 'https://storage.test/private-view',
              },
              signal,
            ),
        },
      ],
      f.calls,
      new AbortController().signal,
    )
    expect(result.uploaded).toEqual(['owned.mp4'])
    expect(result.problems).toEqual([])
    expect(result.uncertain).toBe(false)
    expect(fetch).toHaveBeenCalledWith(
      'https://storage.test/private-view',
      expect.objectContaining({ credentials: 'omit', signal: expect.any(AbortSignal) }),
    )
    expect(readVideoMetadata).toHaveBeenCalledWith(expect.any(NodeFile), expect.any(AbortSignal))
    expect(putBlobWithProgress).toHaveBeenCalledTimes(1)
    expect(f.requests[0]).toMatchObject({
      sources: [{ filename: 'owned.mp4', fingerprint: expect.stringMatching(/^[0-9a-f]{64}$/u) }],
    })
    expect(f.requests[1]).toMatchObject({ preserveConfirmedImport: true })
    expect(
      f.names.some((name) => name.startsWith('Start') || name === 'DiscardClipSourceBatch'),
    ).toBe(false)
    expect(
      JSON.stringify(f.requests, (_k, value) =>
        typeof value === 'bigint' ? value.toString() : value,
      ),
    ).not.toContain('private-view')
  })

  it('keeps verified siblings and reports a failed transfer without uploading good files again', async () => {
    const f = fixture({ importConfirmationFailures: ['bad.mp4'] })
    vi.mocked(putBlobWithProgress).mockImplementation(async (_url, _headers, file) => {
      if ('name' in file && file.name === 'bad.mp4') throw new Error('transfer refused')
    })
    const result = await transferClipSourceFiles(
      'clip',
      [source('one.mp4'), source('bad.mp4'), source('three.mp4')],
      f.calls,
      new AbortController().signal,
    )
    expect(result.uploaded).toEqual(['one.mp4', 'three.mp4'])
    expect(result.batch?.state).toBe('ready')
    expect(result.problems).toEqual([{ filename: 'bad.mp4', reason: 'transfer' }])
    expect(putBlobWithProgress).toHaveBeenCalledTimes(3)
    expect(f.names.filter((name) => name === 'CreateClipSourceBatch')).toHaveLength(1)
    expect(f.names).not.toContain('DiscardClipSourceBatch')
  })

  it('reconciles lost PUT, confirmation and completion acknowledgements against one receipt', async () => {
    const f = fixture({ importConfirmationLostAck: ['one.mp4'], importCompletionLostAcks: 1 })
    vi.mocked(putBlobWithProgress).mockRejectedValueOnce(new Error('lost PUT acknowledgement'))
    const result = await transferClipSourceFiles(
      'clip',
      [source('one.mp4')],
      f.calls,
      new AbortController().signal,
    )
    expect(result.uploaded).toEqual(['one.mp4'])
    expect(result.problems).toEqual([])
    expect(result.uncertain).toBe(false)
    expect(putBlobWithProgress).toHaveBeenCalledTimes(1)
    expect(f.names.filter((name) => name === 'CompleteClipSourceImport')).toHaveLength(2)
  })

  it('stops loading beyond the count bound and rejects duplicates before reserving', async () => {
    const f = fixture()
    const sources = Array.from({ length: 22 }, (_, i) => source(`${i}.mp4`))
    sources.splice(1, 0, source('same.mp4', '0.mp4'), source('0.mp4', 'another video'))
    const result = await transferClipSourceFiles(
      'clip',
      sources,
      f.calls,
      new AbortController().signal,
    )
    expect(result.uploaded).toHaveLength(20)
    expect(result.problems).toEqual([
      { filename: 'same.mp4', reason: 'duplicate' },
      { filename: '0.mp4', reason: 'duplicate' },
      { filename: '20.mp4', reason: 'limit' },
      { filename: '21.mp4', reason: 'limit' },
    ])
    expect(sources.at(-1)!.load).not.toHaveBeenCalled()
    expect(f.requests[0]).toMatchObject({ sources: expect.any(Array) })
    expect((f.requests[0] as { sources: unknown[] }).sources).toHaveLength(20)
  })

  it('uses actual duration and byte bounds while leaving out named failures', async () => {
    const f = fixture()
    vi.mocked(readVideoMetadata)
      .mockResolvedValueOnce({ durationMs: 30 * 60 * 1000, width: 1920, height: 1080 })
      .mockResolvedValue({ durationMs: 1000, width: 1920, height: 1080 })
    const result = await transferClipSourceFiles(
      'clip',
      [
        source('long.mp4'),
        source('over.mp4'),
        {
          filename: 'missing.mp4',
          load: async () => {
            throw new Error('gone')
          },
        },
      ],
      f.calls,
      new AbortController().signal,
    )
    expect(result.uploaded).toEqual(['long.mp4'])
    expect(result.problems).toEqual([
      { filename: 'over.mp4', reason: 'limit' },
      { filename: 'missing.mp4', reason: 'read' },
    ])
  })
  it('preserves originals when both completion responses are uncertain', async () => {
    const f = fixture({ importConfirmationFailures: ['bad.mp4'], importCompletionLostAcks: 2 })
    const result = await transferClipSourceFiles(
      'clip',
      [source('one.mp4'), source('bad.mp4')],
      f.calls,
      new AbortController().signal,
    )
    expect(result.uncertain).toBe(true)
    expect(result.uploaded).toEqual(['one.mp4'])
    expect(result.problems).toEqual([{ filename: 'bad.mp4', reason: 'unconfirmed' }])
    const retained = await f.calls.retained('clip')
    expect(
      retained.find((batch) => batch.current)?.sources.map((item) => item.metadata.filename),
    ).toEqual(['one.mp4'])
    expect(f.names).not.toContain('DiscardClipSourceBatch')
  })

  it('keeps the aggregate byte bound without allocating huge fixture files', async () => {
    const f = fixture()
    const sources = Array.from({ length: 5 }, (_, index) => ({
      filename: `large-${index}.mp4`,
      load: async () => {
        const file = new NodeFile([`fixture-${index}`], `large-${index}.mp4`, { type: 'video/mp4' })
        Object.defineProperty(file, 'size', { value: 2 * 1024 * 1024 * 1024 })
        return file as unknown as File
      },
    }))
    const result = await transferClipSourceFiles(
      'clip',
      sources,
      f.calls,
      new AbortController().signal,
    )
    expect(result.uploaded).toHaveLength(4)
    expect(result.problems).toEqual([{ filename: 'large-4.mp4', reason: 'limit' }])
    expect(putBlobWithProgress).toHaveBeenCalledTimes(4)
  })

  it('never returns another project’s completion receipt or discards verified originals', async () => {
    const f = fixture()
    const complete = f.calls.completeImport
    f.calls.completeImport = async (...args) => {
      const batch = await complete(...args)
      return batch ? { ...batch, projectId: 'another-project' } : batch
    }
    const result = await transferClipSourceFiles(
      'clip',
      [source('one.mp4')],
      f.calls,
      new AbortController().signal,
    )
    expect(result).toMatchObject({
      uploaded: ['one.mp4'],
      problems: [],
      uncertain: true,
    })
    expect(result.batch).toBeUndefined()
    expect(f.names).not.toContain('DiscardClipSourceBatch')
  })
})
