import { describe, expect, it } from 'vitest'
import { createFakeAuthTransport } from '@/test/session'
import { UploadRejected } from '../model/types'
import { createUploadHandshake } from './upload-handshake'

describe('createUploadHandshake', () => {
  it('preserves the stable reason and allowlisted filename parameter from CreateUpload', async () => {
    const handshake = createUploadHandshake(
      createFakeAuthTransport({
        user: { id: 'alice' },
        posts: {
          posts: [
            {
              slug: 'post-1',
              images: [{ id: 'image-1', filename: 'taken.jpg' }],
            },
          ],
        },
      }),
    )

    const error = await handshake.createUpload('post-1', 'taken.jpg').catch((cause) => cause)

    expect(error).toBeInstanceOf(UploadRejected)
    expect((error as UploadRejected).failure).toEqual({
      reason: 'POST_FILENAME_TAKEN',
      params: { filename: 'taken.jpg' },
    })
    expect(error.message).not.toContain('private backend prose')
  })

  it('carries ConfirmUpload not-found as a final structured answer', async () => {
    const handshake = createUploadHandshake(
      createFakeAuthTransport({ user: { id: 'alice' }, posts: { posts: [] } }),
    )

    const error = await handshake
      .confirmUpload('missing', { width: 1, height: 1 })
      .catch((cause) => cause)

    expect(error).toBeInstanceOf(UploadRejected)
    expect((error as UploadRejected).failure).toEqual({ reason: 'UPLOAD_NOT_FOUND', params: {} })
  })

  // VIDEO-5: the reservation says which KIND it is, and the confirm carries the duration the
  // container declared. Which half comes back is the upload's own kind.
  it('reserves a video and confirms it with its duration', async () => {
    const handshake = createUploadHandshake(
      createFakeAuthTransport({
        user: { id: 'alice' },
        posts: { posts: [{ slug: 'post-1', images: [] }] },
      }),
    )

    const reserved = await handshake.createUpload('post-1', 'clip.mp4', 'video')
    // The PUT is signed for the container's own type, not for image/jpeg.
    expect(reserved.contentType).toBe('video/mp4')

    const confirmed = await handshake.confirmUpload(reserved.uploadId, {
      width: 1920,
      height: 1080,
      durationMs: 8_000,
    })
    expect(confirmed).toEqual({
      kind: 'video',
      video: {
        id: reserved.uploadId,
        filename: 'clip.mp4',
        width: 1920,
        height: 1080,
        bytes: 12_000_000,
        durationMs: 8_000,
        contentType: 'video/mp4',
        viewUrl: '',
      },
    })
  })

  // One filename namespace across both kinds: a name a clip holds is taken for a photo too.
  it('refuses a photo named like an attached video', async () => {
    const handshake = createUploadHandshake(
      createFakeAuthTransport({
        user: { id: 'alice' },
        posts: { posts: [{ slug: 'post-1', videos: [{ id: 'video-1', filename: 'clip.mp4' }] }] },
      }),
    )

    const error = await handshake.createUpload('post-1', 'clip.mp4').catch((cause) => cause)
    expect(error).toBeInstanceOf(UploadRejected)
  })

  // A photo's call is unchanged on the wire: no kind is a photo on the server, and the default
  // keeps every existing caller working.
  it('confirms a photo with no duration', async () => {
    const handshake = createUploadHandshake(
      createFakeAuthTransport({
        user: { id: 'alice' },
        posts: { posts: [{ slug: 'post-1', images: [] }] },
      }),
    )

    const reserved = await handshake.createUpload('post-1', 'IMG_1.jpg')
    expect(reserved.contentType).toBe('image/jpeg')
    const confirmed = await handshake.confirmUpload(reserved.uploadId, { width: 1024, height: 768 })
    expect(confirmed.kind).toBe('photo')
  })
})
