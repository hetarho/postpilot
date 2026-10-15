import type { ClipSourceBatch, ClipSourceCalls, ClipSourceMetadata } from '@/entities/clip-project'
import {
  CLIP_SOURCE_MAX_BATCH_BYTES,
  CLIP_SOURCE_MAX_COUNT,
  CLIP_SOURCE_MAX_DURATION_MS,
} from '@/entities/clip-design'
import { createClipSourcePipeline } from './pipeline'
import { CLIP_IMPORT_COMPLETION_TIMEOUT_MS } from '../config/import'

export interface ClipImportSource {
  filename: string
  load(signal: AbortSignal): Promise<File>
}
export interface ClipImportProblem {
  filename: string
  reason: 'limit' | 'read' | 'duplicate' | 'transfer' | 'unconfirmed'
}
export interface ClipSourceImportResult {
  batch?: ClipSourceBatch
  uploaded: string[]
  problems: ClipImportProblem[]
  uncertain: boolean
}

/** Read an owned post video's ordinary view URL; only a File enters the existing pipeline. */
export async function fetchClipSourceFile(
  input: { url: string; filename: string; contentType: string },
  signal: AbortSignal,
): Promise<File> {
  const response = await fetch(input.url, { signal, credentials: 'omit', cache: 'no-store' })
  if (!response.ok) throw new Error('Clip source download failed')
  const blob = await response.blob()
  signal.throwIfAborted()
  return new File([blob], input.filename, { type: input.contentType })
}

/** Unlike an interactive replacement, importing a post keeps every verified sibling when one
 * transfer fails. No successful original is discarded or uploaded a second time. */
export async function transferClipSourceFiles(
  projectId: string,
  sources: readonly ClipImportSource[],
  calls: ClipSourceCalls,
  signal: AbortSignal,
): Promise<ClipSourceImportResult> {
  const pipeline = createClipSourcePipeline(calls)
  const accepted: Array<{ file?: File; metadata: ClipSourceMetadata }> = []
  const problems: ClipImportProblem[] = []
  let bytes = 0,
    duration = 0
  for (const source of sources) {
    if (signal.aborted) {
      problems.push({ filename: source.filename, reason: 'transfer' })
      continue
    }
    if (accepted.length >= CLIP_SOURCE_MAX_COUNT) {
      problems.push({ filename: source.filename, reason: 'limit' })
      continue
    }
    try {
      const file = await source.load(signal)
      const metadata = (await pipeline.read([file], signal))[0]
      if (!metadata || metadata.filename !== source.filename)
        throw new Error('Unexpected imported file')
      if (accepted.some((item) => item.metadata.fingerprint === metadata.fingerprint)) {
        problems.push({ filename: source.filename, reason: 'duplicate' })
        continue
      }
      if (
        bytes + metadata.bytes > CLIP_SOURCE_MAX_BATCH_BYTES ||
        duration + metadata.durationMs > CLIP_SOURCE_MAX_DURATION_MS
      ) {
        problems.push({ filename: source.filename, reason: 'limit' })
        continue
      }
      accepted.push({ file, metadata })
      bytes += metadata.bytes
      duration += metadata.durationMs
    } catch {
      problems.push({ filename: source.filename, reason: 'read' })
    }
  }
  if (!accepted.length) return { uploaded: [], problems, uncertain: false }
  signal.throwIfAborted()
  const reservation = await pipeline.reserve(
    projectId,
    accepted.map((item) => item.metadata),
    signal,
  )
  if (
    reservation.batch.projectId !== projectId ||
    reservation.batch.sources.length !== accepted.length ||
    reservation.uploads.length !== accepted.length ||
    new Set(reservation.batch.sources.map((source) => source.metadata.fingerprint)).size !==
      accepted.length ||
    reservation.batch.sources.some(
      (source) =>
        !accepted.some((item) => item.metadata.fingerprint === source.metadata.fingerprint),
    )
  )
    throw new Error('Invalid imported source reservation')
  const confirmed = new Set<string>()
  for (const source of reservation.batch.sources) {
    const entry = accepted.find(
      (item) => item.metadata.fingerprint === source.metadata.fingerprint,
    )!
    const upload = reservation.uploads.find((item) => item.sourceId === source.id)
    if (!upload || !entry.file || signal.aborted) {
      entry.file = undefined
      continue
    }
    try {
      await pipeline.put(upload.putUrl, upload.headers, entry.file, () => {}, signal)
    } catch {
      /* A lost PUT response can still have left a complete object. Confirm it independently. */
    }
    try {
      const batch = await calls.confirm(reservation.batch.id, source.id, signal, {
        preserveConfirmedImport: true,
      })
      if (batch.sources.some((item) => item.id === source.id && item.state === 'ready'))
        confirmed.add(entry.metadata.filename)
    } catch {
      /* Reconcile a failed or lost acknowledgement against the completion receipt. */
    }
    entry.file = undefined
  }
  let result: ClipSourceBatch | undefined
  let settled = false
  // Completion is idempotent and credit-free. A bounded detached completion preserves
  // already confirmed files when the page left while a transfer was in flight.
  for (let attempt = 0; attempt < 2 && !settled; attempt++) {
    try {
      result = await calls.completeImport(
        projectId,
        reservation.batch.id,
        AbortSignal.timeout(CLIP_IMPORT_COMPLETION_TIMEOUT_MS),
      )
      if (result) {
        const original = new Map(reservation.batch.sources.map((source) => [source.id, source]))
        const ids = new Set(result.sources.map((source) => source.id))
        if (
          result.projectId !== projectId ||
          result.state !== 'ready' ||
          !result.current ||
          !result.sources.length ||
          ids.size !== result.sources.length ||
          result.sources.some(
            (source) =>
              source.state !== 'ready' ||
              source.metadata.fingerprint !== original.get(source.id)?.metadata.fingerprint ||
              source.actualBytes !== source.metadata.bytes,
          ) ||
          [...confirmed].some(
            (name) => !result!.sources.some((source) => source.metadata.filename === name),
          )
        )
          throw new Error('Invalid confirmed source receipt')
      } else if (confirmed.size) throw new Error('Completion lost verified sources')
      settled = true
    } catch {
      /* Keep the server's originals; a missing response is never permission to discard. */
    }
  }
  const uploaded = settled
    ? (result?.sources.map((source) => source.metadata.filename) ?? [])
    : [...confirmed]
  const arrived = new Set(uploaded)
  for (const entry of accepted)
    if (!arrived.has(entry.metadata.filename))
      problems.push({
        filename: entry.metadata.filename,
        reason: settled ? 'transfer' : 'unconfirmed',
      })
  return { batch: result, uploaded, problems, uncertain: !settled }
}
