import type { ClipSourceMetadata } from '@/entities/clip-project'
import {
  CLIP_SOURCE_CONTAINERS,
  CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES,
  CLIP_SOURCE_MAX_BATCH_BYTES,
  CLIP_SOURCE_MAX_COUNT,
  CLIP_SOURCE_MAX_DURATION_MS,
  CLIP_SOURCE_MAX_FILE_BYTES,
  CLIP_SOURCE_MAX_FILENAME_CHARS,
} from '@/shared/config'
import { readVideoMetadata, type VideoMetadata } from '@/shared/lib/video'

export type ClipSelectionReason =
  | 'count'
  | 'fileBytes'
  | 'batchBytes'
  | 'duration'
  | 'container'
  | 'metadata'
  | 'duplicate'
  | 'fingerprint'
  | 'filename'
export class ClipSelectionError extends Error {
  constructor(readonly reason: ClipSelectionReason) {
    super(reason)
    this.name = 'ClipSelectionError'
  }
}
export function checkSourceFiles(files: readonly File[]) {
  if (!files.length || files.length > CLIP_SOURCE_MAX_COUNT) throw new ClipSelectionError('count')
  let bytes = 0
  for (const file of files) {
    const extension = file.name.split('.').at(-1)?.toLowerCase() ?? ''
    if (
      !file.name.trim() ||
      Array.from(file.name).length > CLIP_SOURCE_MAX_FILENAME_CHARS ||
      /[\0\r\n]/u.test(file.name)
    )
      throw new ClipSelectionError('filename')
    if (!CLIP_SOURCE_CONTAINERS[extension]?.includes(file.type))
      throw new ClipSelectionError('container')
    if (file.size <= 0 || file.size > CLIP_SOURCE_MAX_FILE_BYTES)
      throw new ClipSelectionError('fileBytes')
    bytes += file.size
  }
  if (bytes > CLIP_SOURCE_MAX_BATCH_BYTES) throw new ClipSelectionError('batchBytes')
}
/** v1: version byte, uint64 size, uint32 MIME length + UTF-8 MIME, uint64 duration,
 * then first and last min(size, 64 KiB) slices (both included for a short file).
 * Integers are big-endian. Filenames and modification dates are not identity. */
export async function sourceFingerprint(file: File, metadata: VideoMetadata): Promise<string> {
  const mime = new TextEncoder().encode(file.type)
  const prefix = new Uint8Array(1 + 8 + 4 + mime.length + 8)
  const view = new DataView(prefix.buffer)
  prefix[0] = 1
  view.setBigUint64(1, BigInt(file.size))
  view.setUint32(9, mime.length)
  prefix.set(mime, 13)
  view.setBigUint64(13 + mime.length, BigInt(metadata.durationMs))
  try {
    const [first, last] = await Promise.all([
      file.slice(0, CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES).arrayBuffer(),
      file.slice(Math.max(0, file.size - CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES)).arrayBuffer(),
    ])
    const input = new Uint8Array(prefix.length + first.byteLength + last.byteLength)
    input.set(prefix)
    input.set(new Uint8Array(first), prefix.length)
    input.set(new Uint8Array(last), prefix.length + first.byteLength)
    const digest = await crypto.subtle.digest('SHA-256', input)
    return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')
  } catch {
    throw new ClipSelectionError('fingerprint')
  }
}
export async function readSourceManifest(
  files: readonly File[],
  signal: AbortSignal,
): Promise<ClipSourceMetadata[]> {
  checkSourceFiles(files)
  const out: ClipSourceMetadata[] = []
  let duration = 0
  for (const file of files) {
    signal.throwIfAborted()
    let metadata: VideoMetadata
    try {
      metadata = await readVideoMetadata(file, signal)
    } catch (error) {
      signal.throwIfAborted()
      throw error instanceof ClipSelectionError ? error : new ClipSelectionError('metadata')
    }
    if (
      ![metadata.durationMs, metadata.width, metadata.height].every(
        (v) => Number.isSafeInteger(v) && v > 0,
      )
    )
      throw new ClipSelectionError('metadata')
    duration += metadata.durationMs
    if (duration > CLIP_SOURCE_MAX_DURATION_MS) throw new ClipSelectionError('duration')
    const fingerprint = await sourceFingerprint(file, metadata)
    signal.throwIfAborted()
    if (out.some((v) => v.fingerprint === fingerprint)) throw new ClipSelectionError('duplicate')
    out.push({
      ...metadata,
      fingerprint,
      filename: file.name,
      contentType: file.type,
      bytes: file.size,
    })
  }
  return out
}
