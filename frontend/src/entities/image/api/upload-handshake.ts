import { Code, ConnectError, type Transport, createClient } from '@connectrpc/connect'
import { PostService } from '@/shared/api'
import { type PostImage, UploadObjectMissing, UploadRejected } from '../model/types'
import { toPostImage } from './image-mappers'

export interface PresignedUpload {
  uploadId: string
  /** On the storage host — the API is never in the path of the bytes ([I6]). */
  putUrl: string
  /** Must be sent back exactly: it is part of the presigned signature
   *  (spec/policy/uploads.md). */
  contentType: string
}

/** The two RPCs around a direct PUT (spec/policy/uploads.md — the upload handshake).
 *  Throws `UploadRejected` for a final answer, `UploadObjectMissing` when the confirm
 *  found nothing to confirm; anything else is a transport failure and retryable as is. */
export function createUploadHandshake(transport: Transport): {
  createUpload: (slug: string, filename: string) => Promise<PresignedUpload>
  confirmUpload: (uploadId: string, width: number, height: number) => Promise<PostImage>
} {
  const client = createClient(PostService, transport)

  return {
    async createUpload(slug, filename) {
      try {
        const response = await client.createUpload({ postSlug: slug, filename })
        return {
          uploadId: response.uploadId,
          putUrl: response.putUrl,
          contentType: response.contentType,
        }
      } catch (error) {
        throw classify(error)
      }
    },

    async confirmUpload(uploadId, width, height) {
      try {
        const response = await client.confirmUpload({ uploadId, width, height })
        if (!response.image) throw new Error('ConfirmUpload returned no image')
        return toPostImage(response.image)
      } catch (error) {
        throw classify(error)
      }
    },
  }
}

function classify(error: unknown): unknown {
  switch (ConnectError.from(error).code) {
    case Code.AlreadyExists:
      return new UploadRejected('duplicate-filename')
    case Code.InvalidArgument:
      return new UploadRejected('invalid')
    case Code.FailedPrecondition:
      return new UploadObjectMissing()
    default:
      return error
  }
}
