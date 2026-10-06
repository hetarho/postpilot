import { ANALYSIS_PREPARATION_LIMITS as limits } from '../config/limits'
import {
  CLIP_BROWSER_ANALYSIS_PROFILE,
  type ClipAnalysisPreparation,
  type ClipAnalysisPreparationInput,
} from '@/entities/clip-project'
import type { BrowserMediaSourceAccess } from '@/shared/lib'
import type {
  AnalysisEncoder,
  AnalysisPreparationProgress,
  AnalysisPreparationRequest,
} from './types'
import {
  validateAnalysisArtifact,
  validateMissingSlots,
  validateOriginalMeasurements,
} from './coverage'

export interface AnalysisPreparationPorts {
  access(
    sourceId: string,
    fingerprint: string,
    signal: AbortSignal,
  ): Promise<BrowserMediaSourceAccess>
  encoder(signal: AbortSignal, progress: (fraction: number) => void): AnalysisEncoder
  begin(input: ClipAnalysisPreparationInput, signal: AbortSignal): Promise<ClipAnalysisPreparation>
  reserve(
    input: { preparationId: string; slot: string; bytes: number; sha256: string },
    signal: AbortSignal,
  ): Promise<{ url: string; headers: Record<string, string>; expiresAt: string; slot: string }>
  complete(id: string, signal: AbortSignal): Promise<ClipAnalysisPreparation>
  cancel(id: string): Promise<unknown>
  cancelParent(id: string): Promise<unknown>
  upload?: typeof fetch
  now?: () => number
}

/** Page-owned immutable attempt: activate the approved durable parent before
 * encoding. Never replay its paid start, and never route failures to native. */
export async function prepareBrowserAnalysis(
  input: AnalysisPreparationRequest,
  ports: AnalysisPreparationPorts,
  signal: AbortSignal,
  startParent: (preparationId: string) => Promise<{ jobId: string }>,
  progress: (value: AnalysisPreparationProgress) => void = () => {},
) {
  let preparation: ClipAnalysisPreparation | undefined, parent: { jobId: string } | undefined
  let current: AnalysisPreparationProgress = {
    stage: 'measuring',
    done: 0,
    total: input.batch.sources.length,
  }
  const update = (value: AnalysisPreparationProgress) => {
    current = value
    progress(value)
  }
  const encoder = ports.encoder(signal, (fraction) => {
    if (!signal.aborted) progress({ ...current, fraction })
  })
  const now = ports.now ?? Date.now
  try {
    signal.throwIfAborted()
    const originals = []
    for (const [index, source] of input.batch.sources.entries()) {
      update({
        stage: 'measuring',
        done: index,
        total: input.batch.sources.length,
        sourceId: source.id,
      })
      const access = await ports.access(source.id, source.metadata.fingerprint, signal)
      const measured = await encoder.measure({
        sourceId: source.id,
        fingerprint: source.metadata.fingerprint,
        access,
      })
      if (measured.sourceId !== source.id || measured.fingerprint !== source.metadata.fingerprint)
        throw new Error('CLIP_ANALYSIS_COPY_OWNERSHIP')
      originals.push(measured)
      validateOriginalMeasurements(originals)
    }
    preparation = await ports.begin(
      {
        projectId: input.projectId,
        batchId: input.batch.id,
        expectedRevision: input.revision,
        quoteId: input.quote.quoteId,
        profileVersion: CLIP_BROWSER_ANALYSIS_PROFILE,
        originals,
      },
      signal,
    )
    if (
      preparation.projectId !== input.projectId ||
      preparation.batchId !== input.batch.id ||
      preparation.revision !== input.revision ||
      preparation.state !== 'preparing' ||
      Date.parse(preparation.expiresAt) <= now()
    )
      throw new Error('CLIP_ANALYSIS_COPY_OWNERSHIP')
    validateMissingSlots(preparation, originals)
    update({ stage: 'starting', done: 0, total: preparation.copies.length })
    parent = await startParent(preparation.id)
    if (!parent.jobId) throw new Error('Missing durable clip job')
    signal.throwIfAborted()
    for (const [index, copy] of preparation.copies.entries()) {
      if (Date.parse(preparation.expiresAt) <= now())
        throw new Error('CLIP_ANALYSIS_PREPARATION_EXPIRED')
      const access = await ports.access(copy.sourceId, copy.fingerprint, signal)
      update({
        stage: 'encoding',
        done: index,
        total: preparation.copies.length,
        sourceId: copy.sourceId,
        slot: copy.slot,
      })
      const artifact = await encoder.encode(
        { sourceId: copy.sourceId, fingerprint: copy.fingerprint, access },
        copy,
      )
      signal.throwIfAborted()
      validateAnalysisArtifact(copy, artifact)
      const upload = await ports.reserve(
        {
          preparationId: preparation.id,
          slot: copy.slot,
          bytes: artifact.buffer.byteLength,
          sha256: artifact.sha256,
        },
        signal,
      )
      if (
        upload.slot !== copy.slot ||
        Date.parse(upload.expiresAt) <= now() ||
        upload.headers['If-None-Match'] !== '*'
      )
        throw new Error('CLIP_ANALYSIS_COPY_OWNERSHIP')
      update({ stage: 'uploading', done: index, total: preparation.copies.length, slot: copy.slot })
      const response = await (ports.upload ?? fetch)(upload.url, {
        method: 'PUT',
        headers: upload.headers,
        body: artifact.buffer,
        signal: AbortSignal.any([signal, AbortSignal.timeout(limits.timeoutMs)]),
        credentials: 'omit',
        redirect: 'error',
      })
      if (!response.ok) throw new Error('CLIP_ANALYSIS_UPLOAD_FAILED')
      signal.throwIfAborted()
    }
    update({
      stage: 'verifying',
      done: preparation.copies.length,
      total: preparation.copies.length,
    })
    const completed = await ports.complete(preparation.id, signal)
    if (
      completed.id !== preparation.id ||
      completed.projectId !== input.projectId ||
      completed.batchId !== input.batch.id ||
      completed.jobId !== parent.jobId ||
      !['verifying', 'accepted', 'consumed'].includes(completed.state)
    )
      throw new Error('CLIP_ANALYSIS_COPY_OWNERSHIP')
    return parent
  } catch (error) {
    // Abort local resources immediately. Durable parent cancellation precedes
    // fencing the session, and neither cleanup request uses the aborted signal.
    encoder.close()
    if (parent) await ports.cancelParent(parent.jobId).catch(() => undefined)
    if (preparation) await ports.cancel(preparation.id).catch(() => undefined)
    throw error
  } finally {
    encoder.close()
  }
}
