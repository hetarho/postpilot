import type { ClipSourceCalls } from '@/entities/clip-project'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { readSourceManifest } from '../model/manifest'
import type { SourcePipeline } from '../model/session'

/** The upload session's collaborators: the rpcs come from the clip-project entity (ARCH-17),
 *  and this feature adds what only a browser can do — reading a file, putting bytes, minting a
 *  preview URL. */
export function createClipSourcePipeline(calls: ClipSourceCalls): SourcePipeline {
  return {
    read: readSourceManifest,
    retained: (projectId, signal) => calls.retained(projectId, signal),
    playback: (projectId, id, fingerprint, signal) =>
      calls.playback(projectId, id, fingerprint, signal),
    // Deliberately project each metadata field. No File, Blob or preview URL crosses Connect.
    reserve: (projectId, manifest, signal) => calls.reserve(projectId, manifest, signal),
    put: putBlobWithProgress,
    confirm: (batchId, sourceId, signal) => calls.confirm(batchId, sourceId, signal),
    discard: (batchId) => calls.discard(batchId),
    createURL: (file) => URL.createObjectURL(file),
    revokeURL: (url) => URL.revokeObjectURL(url),
  }
}
