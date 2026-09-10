import { createClient, type Transport } from '@connectrpc/connect'
import { toClipSourceBatch } from '@/entities/clip-project'
import { ClipService } from '@/shared/api'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { readSourceManifest } from '../model/manifest'
import type { SourcePipeline } from '../model/session'

export function createClipSourcePipeline(transport: Transport): SourcePipeline {
  const client = createClient(ClipService, transport)
  return {
    read: readSourceManifest,
    async reserve(projectId, manifest, signal) {
      // Deliberately project each metadata field. No File, Blob or preview URL crosses Connect.
      const response = await client.createClipSourceBatch(
        {
          projectId,
          sources: manifest.map((m) => ({
            filename: m.filename,
            contentType: m.contentType,
            bytes: BigInt(m.bytes),
            durationMs: m.durationMs,
            width: m.width,
            height: m.height,
            fingerprint: m.fingerprint,
          })),
        },
        { signal },
      )
      if (!response.batch) throw new Error('Missing source reservation')
      return {
        batch: toClipSourceBatch(response.batch),
        uploads: response.uploads.map((u) => ({
          sourceId: u.sourceId,
          putUrl: u.putUrl,
          headers: u.headers,
        })),
      }
    },
    put: putBlobWithProgress,
    async confirm(batchId, sourceId, signal) {
      const response = await client.confirmClipSource({ batchId, sourceId }, { signal })
      if (!response.batch) throw new Error('Missing confirmed sources')
      return toClipSourceBatch(response.batch)
    },
    async discard(batchId) {
      await client.discardClipSourceBatch({ batchId })
    },
    createURL: (file) => URL.createObjectURL(file),
    revokeURL: (url) => URL.revokeObjectURL(url),
  }
}
