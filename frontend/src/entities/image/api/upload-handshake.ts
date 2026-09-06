import { type Transport, createClient } from '@connectrpc/connect'
import { AttachmentKind, appFailureFromConnect, PostService } from '@/shared/api'
import { type PostVideo, toPostVideo } from '@/entities/video/@x/image'
import {
  type PostImage,
  UploadObjectMissing,
  UploadRejected,
  UploadRpcFailure,
} from '../model/types'
import { toPostImage } from './image-mappers'

/** Which kind of attachment a handshake is for. It decides the ceiling the reservation counts
 *  against, the extension of the object key and the Content-Type the PUT is signed for — all
 *  of it server-side; this only says which. */
export type UploadKind = 'photo' | 'video'

/** What a confirm produced. Exactly one is set, decided by the UPLOAD's own kind rather than
 *  by the request: a client that reserved a photo can never be handed a clip back. */
export type ConfirmedAttachment =
  { kind: 'photo'; image: PostImage } | { kind: 'video'; video: PostVideo }

/** What the browser measured about the file. A photo reports the converted copy's dimensions;
 *  a clip adds the duration its container declared, which the server bounds. */
export interface ConfirmMeasurements {
  width: number
  height: number
  durationMs?: number
}

export interface PresignedUpload {
  uploadId: string
  /** On the storage host — the API is never in the path of the bytes ([I6]). */
  putUrl: string
  /** Must be sent back exactly: it is part of the presigned signature
   *  (spec/legacy/policy/uploads.md). */
  contentType: string
}

/** The two RPCs around a direct PUT (spec/legacy/policy/uploads.md — the upload handshake).
 *  Throws `UploadRejected` for a final answer, `UploadObjectMissing` when the confirm
 *  found nothing to confirm; anything else is a transport failure and retryable as is. */
export function createUploadHandshake(transport: Transport): {
  createUpload: (slug: string, filename: string, kind?: UploadKind) => Promise<PresignedUpload>
  confirmUpload: (
    uploadId: string,
    measurements: ConfirmMeasurements,
  ) => Promise<ConfirmedAttachment>
} {
  const client = createClient(PostService, transport)

  return {
    async createUpload(slug, filename, kind = 'photo') {
      try {
        const response = await client.createUpload({
          postSlug: slug,
          filename,
          // UNSPECIFIED reads as a photo on the server, so the default keeps every existing
          // call byte-identical on the wire.
          // The generator strips the enum's own prefix, so these are the proto's
          // ATTACHMENT_KIND_VIDEO / ATTACHMENT_KIND_PHOTO.
          kind: kind === 'video' ? AttachmentKind.VIDEO : AttachmentKind.PHOTO,
        })
        return {
          uploadId: response.uploadId,
          putUrl: response.putUrl,
          contentType: response.contentType,
        }
      } catch (error) {
        throw classify(error)
      }
    },

    async confirmUpload(uploadId, { width, height, durationMs }) {
      try {
        const response = await client.confirmUpload({
          uploadId,
          width,
          height,
          durationMs: BigInt(durationMs ?? 0),
        })
        // Which half came back is the server's answer, not ours to assume.
        if (response.video) return { kind: 'video', video: toPostVideo(response.video) }
        if (response.image) return { kind: 'photo', image: toPostImage(response.image) }
        throw new Error('ConfirmUpload returned no attachment')
      } catch (error) {
        throw classify(error)
      }
    },
  }
}

function classify(error: unknown): unknown {
  const failure = appFailureFromConnect(error)
  switch (failure.reason) {
    case 'POST_FILENAME_TAKEN':
    case 'UPLOAD_INVALID':
    case 'UPLOAD_NOT_FOUND':
      return new UploadRejected(failure)
    case 'UPLOAD_OBJECT_MISSING':
      return new UploadObjectMissing(failure)
    default:
      return new UploadRpcFailure(failure)
  }
}
