/** A photo attached to a post, as the app talks about it. */
export interface PostImage {
  id: string
  /** The name the model and the exporters refer to this photo by; unique within a post. */
  filename: string
  width: number
  height: number
  bytes: number
  /** Where to load the pixels from. Normally the short-lived presigned GET minted per
   *  `GetPost` (never persisted — spec/policy/uploads.md). For a photo this client just
   *  uploaded it is a local object URL of the converted copy, until the next `GetPost`
   *  replaces it — the confirm answer carries no URL, and the bytes are already here. */
  viewUrl: string
}

/** Why the server refused an upload step. Final: a retry would get the same answer. */
export type UploadRejection = 'duplicate-filename' | 'invalid'

export class UploadRejected extends Error {
  constructor(readonly reason: UploadRejection) {
    super(reason)
    this.name = 'UploadRejected'
  }
}

/** The server found no object behind the upload id at confirm time: the PUT did not
 *  land. Retryable, but from `CreateUpload`, not by confirming again. */
export class UploadObjectMissing extends Error {
  constructor() {
    super('object missing')
    this.name = 'UploadObjectMissing'
  }
}
