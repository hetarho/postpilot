// What the writing-test entity may import from the image entity (FSD cross-import, `@x`): a test
// photo goes through the post photo pipeline, so its handshake answers in the upload batch's
// shapes and fails with the same errors the batch classifies (`classifyUploadFailure`).
export { classifyUploadFailure } from '../api/upload-handshake'
export type {
  ConfirmedAttachment,
  ConfirmMeasurements,
  PresignedUpload,
} from '../api/upload-handshake'
