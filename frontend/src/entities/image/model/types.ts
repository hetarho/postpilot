/** A photo attached to a post, as the app talks about it. */
export interface PostImage {
  id: string
  /** The name the model and the exporters refer to this photo by; unique within a post. */
  filename: string
  width: number
  height: number
  bytes: number
  /** Where to load the pixels from. Normally the short-lived presigned GET minted per
   *  `GetPost` (never persisted — spec/legacy/policy/uploads.md). For a photo this client just
   *  uploaded it is a local object URL of the converted copy, until the next `GetPost`
   *  replaces it — the confirm answer carries no URL, and the bytes are already here. */
  viewUrl: string
}

/** Why the server refused an upload step. Final: a retry would get the same answer. */
export type UploadRejection = 'duplicate-filename' | 'invalid'

export class UploadRejected extends Error {
  readonly reason: UploadRejection

  constructor(readonly failure: AppFailure) {
    const reason = failure.reason === 'POST_FILENAME_TAKEN' ? 'duplicate-filename' : 'invalid'
    super(reason)
    this.name = 'UploadRejected'
    this.reason = reason
  }
}

/** The server found no object behind the upload id at confirm time: the PUT did not
 *  land. Retryable, but from `CreateUpload`, not by confirming again. */
export class UploadObjectMissing extends Error {
  constructor(readonly failure: AppFailure = { reason: 'UPLOAD_OBJECT_MISSING', params: {} }) {
    super(failure.reason)
    this.name = 'UploadObjectMissing'
  }
}

/** A retryable CreateUpload/ConfirmUpload transport failure with its public structured reason. */
export class UploadRpcFailure extends Error {
  constructor(readonly failure: AppFailure) {
    super(failure.reason)
    this.name = 'UploadRpcFailure'
  }
}
import type { AppFailure } from '@/shared/api'
