import { createClient, type Transport } from '@connectrpc/connect'
import { ClipService } from '@/shared/api'
import { toClipSourceBatch } from './clip-project'

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
