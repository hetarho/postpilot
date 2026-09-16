import { createClient, type Transport } from '@connectrpc/connect'
import { ClipService } from '@/shared/api'
import { toClipProject, toClipSourceBatch } from './clip-project'

/** Metadata may outlive a page; playback capabilities stay with its runtime session. */
export async function getClipSources(
  transport: Transport,
  projectId: string,
  signal?: AbortSignal,
) {
  const response = await createClient(ClipService, transport).getClipSources(
    { projectId },
    { signal },
  )
  return response.batches.map(toClipSourceBatch)
}

/** The ONE owner action that changes source audio. The expected revision is the
 *  plan the owner was looking at, or 0 before a plan exists, and the fingerprint
 *  pins the exact file the choice is about (CLIP-100). */
export async function setClipSourceOriginalSound(
  transport: Transport,
  input: {
    projectId: string
    batchId: string
    sourceId: string
    expectedFingerprint: string
    retainOriginalAudio: boolean
    expectedRevision: number
  },
  signal?: AbortSignal,
) {
  const response = await createClient(ClipService, transport).setClipSourceOriginalSound(input, {
    signal,
  })
  if (!response.batch || !response.project) throw new Error('Invalid source sound response')
  return { batch: toClipSourceBatch(response.batch), project: toClipProject(response.project) }
}

/** The order the owner arranged this batch's footage in (CLIP-136). The whole
 *  batch travels, because a partial order would leave the rest where nobody put
 *  it, and the server refuses one anyway. */
export async function reorderClipSources(
  transport: Transport,
  input: { projectId: string; batchId: string; sourceIds: string[] },
  signal?: AbortSignal,
) {
  const response = await createClient(ClipService, transport).reorderClipSources(input, { signal })
  if (!response.batch) throw new Error('Invalid source order response')
  return toClipSourceBatch(response.batch)
}

export async function getClipSourcePlayback(
  transport: Transport,
  projectId: string,
  sourceId: string,
  expectedFingerprint: string,
  signal?: AbortSignal,
) {
  const response = await createClient(ClipService, transport).getClipSourcePlayback(
    { projectId, sourceId, expectedFingerprint },
    { signal },
  )
  if (!response.url || !Number.isFinite(Date.parse(response.expiresAt)))
    throw new Error('Invalid source playback capability')
  return { url: response.url, expiresAt: response.expiresAt }
}
