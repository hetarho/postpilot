import { useMemo } from 'react'
import { type Transport, createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { WritingTestService } from '@/shared/api'
import {
  type ConfirmedAttachment,
  type ConfirmMeasurements,
  type PresignedUpload,
  classifyUploadFailure,
} from '@/entities/image/@x/writing-test'

/** The two RPCs around a direct PUT of a test photo. It answers in the shapes of the post
 *  photo handshake (`createUploadHandshake`), so the same upload pipeline drives it: the first
 *  argument of `createUpload` is the post slug there and has no meaning for a test photo. */
export function createTestPhotoHandshake(transport: Transport): {
  createUpload: (_key: string, filename: string) => Promise<PresignedUpload>
  confirmUpload: (
    uploadId: string,
    measurements: ConfirmMeasurements,
  ) => Promise<ConfirmedAttachment>
} {
  const client = createClient(WritingTestService, transport)
  return {
    async createUpload(_key, filename) {
      try {
        const response = await client.createTestPhotoUpload({ filename })
        return {
          uploadId: response.uploadId,
          putUrl: response.putUrl,
          contentType: response.contentType,
        }
      } catch (error) {
        throw classifyUploadFailure(error)
      }
    },
    async confirmUpload(uploadId, { width, height }) {
      try {
        const { photo } = await client.confirmTestPhotoUpload({ uploadId, width, height })
        if (!photo) throw new Error('ConfirmTestPhotoUpload returned no photo')
        return {
          kind: 'photo',
          image: {
            id: photo.id,
            filename: photo.filename,
            width: photo.width,
            height: photo.height,
            // The confirm answer states no size; the batch swaps in its local preview.
            bytes: 0,
            viewUrl: photo.viewUrl,
          },
        }
      } catch (error) {
        throw classifyUploadFailure(error)
      }
    },
  }
}

/** The test photo handshake bound to the transport the app is mounted on. */
export function useTestPhotoHandshake(): ReturnType<typeof createTestPhotoHandshake> {
  const transport = useTransport()
  return useMemo(() => createTestPhotoHandshake(transport), [transport])
}
