import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { type PostImage, UploadObjectMissing, UploadRejected } from '@/entities/image'
import { DecodeError } from '@/shared/lib'
import {
  type UploadItem,
  type UploadPipeline,
  discardUploadBatches,
  peekUploadState,
  subscribeUploadBatch,
  uploadBatch,
} from './upload-batch'

/** A pipeline whose every step can be made to fail on demand. */
function fakePipeline(options: { failPuts?: number; failConvert?: (file: File) => boolean } = {}) {
  let putsLeftToFail = options.failPuts ?? 0
  let uploads = 0
  const pipeline: UploadPipeline = {
    convert: vi.fn(async (file: File) => {
      if (options.failConvert?.(file)) throw new DecodeError('unreadable')
      return { blob: new Blob(['jpeg'], { type: 'image/jpeg' }), width: 1024, height: 768 }
    }),
    createUpload: vi.fn(async (_slug: string, filename: string) => {
      if (filename === 'taken.jpg')
        throw new UploadRejected({
          reason: 'POST_FILENAME_TAKEN',
          params: { filename },
        })
      uploads += 1
      return {
        uploadId: `upload-${uploads}`,
        putUrl: `https://storage.test/${uploads}`,
        contentType: 'image/jpeg',
      }
    }),
    put: vi.fn(async () => {
      if (putsLeftToFail > 0) {
        putsLeftToFail -= 1
        throw new TypeError('network')
      }
    }),
    confirm: vi.fn(async (uploadId: string, width: number, height: number): Promise<PostImage> => ({
      id: uploadId,
      filename: `${uploadId}.jpg`,
      width,
      height,
      bytes: 1,
      viewUrl: '',
    })),
  }
  return pipeline
}

function file(name: string, size = 100) {
  return new File([new Uint8Array(size)], name)
}

function statuses(items: readonly UploadItem[]) {
  return items.map((item) => item.status)
}

let slugSequence = 0
let slug: string

beforeEach(() => {
  slugSequence += 1
  slug = `20260828-post-${slugSequence}`
  // jsdom's URL lacks these two.
  URL.createObjectURL = vi.fn(() => `blob:preview-${Math.random()}`)
  URL.revokeObjectURL = vi.fn()
})

afterEach(() => {
  discardUploadBatches()
})

describe('an upload batch', () => {
  // A2 / A4 in miniature: convert → CreateUpload → PUT → ConfirmUpload, then the photo
  // belongs to the post and leaves the batch.
  it('walks every accepted file through the pipeline and hands each confirmed photo over', async () => {
    const pipeline = fakePipeline()
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline, onConfirmed })
    // Watched, like an open editor — an unwatched batch with nothing left is collected.
    subscribeUploadBatch(slug, () => {})

    batch.add([file('IMG_1.HEIC'), file('IMG_2.jpg')], [])

    await vi.waitFor(() => expect(onConfirmed).toHaveBeenCalledTimes(2))
    expect(pipeline.put).toHaveBeenCalledTimes(2)
    expect(pipeline.put).toHaveBeenCalledWith(
      expect.stringContaining('storage.test'),
      'image/jpeg',
      expect.any(Blob),
    )
    expect(pipeline.confirm).toHaveBeenCalledWith('upload-1', 1024, 768)
    // The confirm answer has no view URL; the local copy stands in for it.
    expect(onConfirmed).toHaveBeenCalledWith(
      slug,
      expect.objectContaining({ id: 'upload-1', viewUrl: expect.stringMatching(/^blob:/) }),
    )
    expect(peekUploadState(slug)).toEqual({ items: [], completed: 2 })
  })

  it('files photos under .jpg names, de-duplicated against the post and the batch', async () => {
    const batch = uploadBatch(slug, { pipeline: fakePipeline(), onConfirmed: vi.fn() })

    batch.add([file('IMG_1.HEIC'), file('IMG_1.png')], ['IMG_1.jpg'])

    expect(peekUploadState(slug).items.map((item) => item.filename)).toEqual([
      'IMG_1 (2).jpg',
      'IMG_1 (3).jpg',
    ])
  })

  // A1: rejected at selection — nothing is read, converted or sent.
  it('lists a file that fails the filter as skipped without touching the pipeline', async () => {
    const pipeline = fakePipeline()
    const batch = uploadBatch(slug, { pipeline, onConfirmed: vi.fn() })

    batch.add([file('setup.exe'), file('huge.heic', 26 * 1024 * 1024)], [])

    expect(peekUploadState(slug).items).toMatchObject([
      { status: 'skipped', reason: 'extension', name: 'setup.exe' },
      { status: 'skipped', reason: 'too-large', name: 'huge.heic' },
    ])
    expect(pipeline.convert).not.toHaveBeenCalled()
  })

  // A9: one undecodable file does not stop the others.
  it('skips a file the decoder cannot read, with the reason, and carries on with the rest', async () => {
    const pipeline = fakePipeline({ failConvert: (candidate) => candidate.name === 'bad.heic' })
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline, onConfirmed })

    batch.add([file('bad.heic'), file('good.heic')], [])

    await vi.waitFor(() => expect(onConfirmed).toHaveBeenCalledTimes(1))
    expect(peekUploadState(slug).items).toMatchObject([
      { status: 'skipped', reason: 'unreadable', name: 'bad.heic' },
    ])
  })

  // A8: a failed PUT is retryable, and the retry starts over at CreateUpload.
  it('marks a failed PUT as failed, leaves the others alone, and retries from CreateUpload', async () => {
    const pipeline = fakePipeline({ failPuts: 1 })
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline, onConfirmed })

    batch.add([file('first.jpg'), file('second.jpg')], [])

    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['failed']))
    expect(onConfirmed).toHaveBeenCalledTimes(1)
    const failed = peekUploadState(slug).items[0]
    expect(failed.failure).toBe('network')
    expect(pipeline.createUpload).toHaveBeenCalledTimes(2)

    batch.retry(failed.id)

    await vi.waitFor(() => expect(onConfirmed).toHaveBeenCalledTimes(2))
    // A third CreateUpload, i.e. a fresh upload_id — never the stale URL.
    expect(pipeline.createUpload).toHaveBeenCalledTimes(3)
    expect(pipeline.confirm).toHaveBeenLastCalledWith('upload-3', 1024, 768)
    expect(peekUploadState(slug).items).toEqual([])
  })

  // A confirm whose answer was lost may have landed: asking again with the same id is
  // idempotent, while starting over would be refused as a duplicate filename.
  it('retries a lost confirm by confirming the same upload id again', async () => {
    const pipeline = fakePipeline()
    pipeline.confirm = vi
      .fn<UploadPipeline['confirm']>()
      .mockRejectedValueOnce(new TypeError('network'))
      .mockResolvedValueOnce({
        id: 'upload-1',
        filename: 'a.jpg',
        width: 1024,
        height: 768,
        bytes: 1,
        viewUrl: '',
      })
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline, onConfirmed })

    batch.add([file('a.jpg')], [])
    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['failed']))

    batch.retry(peekUploadState(slug).items[0].id)

    await vi.waitFor(() => expect(onConfirmed).toHaveBeenCalledTimes(1))
    expect(pipeline.createUpload).toHaveBeenCalledTimes(1)
    expect(pipeline.put).toHaveBeenCalledTimes(1)
    expect(pipeline.confirm).toHaveBeenLastCalledWith('upload-1', 1024, 768)
  })

  it('starts over from CreateUpload when the server says the object never landed', async () => {
    const pipeline = fakePipeline()
    pipeline.confirm = vi
      .fn<UploadPipeline['confirm']>()
      .mockRejectedValueOnce(new UploadObjectMissing())
      .mockResolvedValueOnce({
        id: 'upload-2',
        filename: 'a.jpg',
        width: 1024,
        height: 768,
        bytes: 1,
        viewUrl: '',
      })
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline, onConfirmed })

    batch.add([file('a.jpg')], [])
    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['failed']))
    expect(peekUploadState(slug).items[0].failure).toBe('network')

    batch.retry(peekUploadState(slug).items[0].id)

    await vi.waitFor(() => expect(onConfirmed).toHaveBeenCalledTimes(1))
    expect(pipeline.createUpload).toHaveBeenCalledTimes(2)
    expect(pipeline.put).toHaveBeenCalledTimes(2)
    expect(pipeline.confirm).toHaveBeenLastCalledWith('upload-2', 1024, 768)
  })

  it("treats the server's own answer as final rather than retryable", async () => {
    const batch = uploadBatch(slug, { pipeline: fakePipeline(), onConfirmed: vi.fn() })

    batch.add([file('taken.heic')], [])

    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['failed']))
    expect(peekUploadState(slug).items[0].failure).toBe('duplicate-filename')
    expect(peekUploadState(slug).items[0].appFailure).toEqual({
      reason: 'POST_FILENAME_TAKEN',
      params: { filename: 'taken.jpg' },
    })
  })

  it('notifies subscribers on every change and lets a failed item be dismissed', async () => {
    const batch = uploadBatch(slug, {
      pipeline: fakePipeline({ failPuts: 1 }),
      onConfirmed: vi.fn(),
    })
    const listener = vi.fn()
    subscribeUploadBatch(slug, listener)

    batch.add([file('a.jpg')], [])
    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['failed']))
    // selected → converting → preview → uploading → failed, at least.
    expect(listener.mock.calls.length).toBeGreaterThanOrEqual(4)

    batch.dismiss(peekUploadState(slug).items[0].id)
    expect(peekUploadState(slug).items).toEqual([])
    expect(URL.revokeObjectURL).toHaveBeenCalled()
  })

  it('starts the progress count over when a new selection lands on an idle batch', async () => {
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline: fakePipeline(), onConfirmed })
    subscribeUploadBatch(slug, () => {})

    batch.add([file('a.jpg'), file('b.jpg')], [])
    await vi.waitFor(() => expect(peekUploadState(slug).completed).toBe(2))

    batch.add([file('c.jpg')], [])
    expect(peekUploadState(slug).completed).toBe(0)
    await vi.waitFor(() => expect(peekUploadState(slug).completed).toBe(1))
  })

  it('goes quiet once the session ends', async () => {
    let releaseConfirm: () => void = () => {}
    const pipeline = fakePipeline()
    pipeline.confirm = vi.fn(
      () =>
        new Promise<PostImage>((resolve) => {
          releaseConfirm = () =>
            resolve({
              id: 'late',
              filename: 'late.jpg',
              width: 1,
              height: 1,
              bytes: 1,
              viewUrl: '',
            })
        }),
    )
    const onConfirmed = vi.fn()
    const batch = uploadBatch(slug, { pipeline, onConfirmed })

    batch.add([file('a.jpg')], [])
    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['confirming']))

    discardUploadBatches()
    releaseConfirm()
    await Promise.resolve()

    expect(onConfirmed).not.toHaveBeenCalled()
    expect(peekUploadState(slug)).toEqual({ items: [], completed: 0 })
  })

  it('tells a still-mounted subscriber that its items are gone when the session ends', async () => {
    const batch = uploadBatch(slug, {
      pipeline: fakePipeline({ failPuts: 1 }),
      onConfirmed: vi.fn(),
    })
    const listener = vi.fn()
    subscribeUploadBatch(slug, listener)
    batch.add([file('a.jpg')], [])
    await vi.waitFor(() => expect(statuses(peekUploadState(slug).items)).toEqual(['failed']))
    listener.mockClear()

    discardUploadBatches()

    expect(listener).toHaveBeenCalled()
    expect(peekUploadState(slug).items).toEqual([])
  })
})
