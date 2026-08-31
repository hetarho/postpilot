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

    const error = await handshake.confirmUpload('missing', 1, 1).catch((cause) => cause)

    expect(error).toBeInstanceOf(UploadRejected)
    expect((error as UploadRejected).failure).toEqual({ reason: 'UPLOAD_NOT_FOUND', params: {} })
  })
})
