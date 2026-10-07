import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { createClient, type Transport } from '@connectrpc/connect'
import { ClipSourceService, type ClipAnalysisPreparationResponse } from '@/shared/api'
import {
  CLIP_BROWSER_ANALYSIS_PROFILE,
  type ClipAnalysisPreparation,
  type ClipAnalysisPreparationInput,
} from '../model/analysis-preparation'

const preparationStates = new Set([
  'preparing',
  'verifying',
  'accepted',
  'consumed',
  'failed',
  'cancelled',
  'expired',
])
const copyStates = new Set(['expected', 'reserved', 'verified', 'cleanup', 'deleted'])
const sha256 = /^[a-f0-9]{64}$/

/** The browser's claims and the server's copy verdict remain separate. No
 * signed access or decoded media becomes durable domain data. */
export function toClipAnalysisPreparation(
  response: ClipAnalysisPreparationResponse,
): ClipAnalysisPreparation {
  const profile = response.profile
  if (
    !response.preparationId ||
    !response.projectId ||
    !response.batchId ||
    !preparationStates.has(response.state) ||
    !Number.isFinite(Date.parse(response.expiresAt)) ||
    response.originalMeasurementProvenance !== 'browser_client' ||
    !profile ||
    profile.version !== CLIP_BROWSER_ANALYSIS_PROFILE ||
    profile.intervalMs !== 60_000 ||
    profile.longEdge !== 720 ||
    profile.framesPerSecond !== 15 ||
    profile.maxCopyBytes !== BigInt(8 * 1024 * 1024) ||
    profile.videoCodec !== 'h264' ||
    profile.pixelFormat !== 'yuv420p' ||
    profile.audioCodec !== 'aac' ||
    profile.audioRate !== 48_000 ||
    profile.audioChannels !== 1 ||
    profile.audioBitrate !== 64_000 ||
    response.copies.length > 49 ||
    response.progress < 0 ||
    response.progress > 1000
  )
    throw new Error('Invalid analysis preparation response')
  const slots = new Set<string>()
  const copies = response.copies.map((copy) => {
    if (
      !copy.slot ||
      slots.has(copy.slot) ||
      !copy.sourceId ||
      !sha256.test(copy.fingerprint) ||
      !copyStates.has(copy.state) ||
      copy.ordinal < 0 ||
      copy.offsetMs !== copy.ordinal * profile.intervalMs ||
      copy.durationMs <= 0 ||
      copy.durationMs > profile.intervalMs ||
      copy.width < 2 ||
      copy.height < 2 ||
      Math.max(copy.width, copy.height) > profile.longEdge ||
      copy.width % 2 !== 0 ||
      copy.height % 2 !== 0 ||
      copy.bytes < 0n ||
      copy.bytes > profile.maxCopyBytes ||
      (copy.sha256 !== '' && !sha256.test(copy.sha256))
    )
      throw new Error('Invalid analysis copy slot')
    slots.add(copy.slot)
    return {
      slot: copy.slot,
      sourceId: copy.sourceId,
      fingerprint: copy.fingerprint,
      ordinal: copy.ordinal,
      offsetMs: copy.offsetMs,
      durationMs: copy.durationMs,
      width: copy.width,
      height: copy.height,
      hasAudio: copy.hasAudio,
      sha256: copy.sha256,
      state: copy.state as ClipAnalysisPreparation['copies'][number]['state'],
      bytes: Number(copy.bytes),
    }
  })
  return {
    id: response.preparationId,
    projectId: response.projectId,
    batchId: response.batchId,
    revision: response.expectedRevision,
    state: response.state as ClipAnalysisPreparation['state'],
    expiresAt: response.expiresAt,
    originalMeasurementProvenance: 'browser_client',
    profile: {
      version: profile.version,
      intervalMs: profile.intervalMs,
      longEdge: profile.longEdge,
      framesPerSecond: profile.framesPerSecond,
      maxCopyBytes: Number(profile.maxCopyBytes),
      videoCodec: profile.videoCodec,
      pixelFormat: profile.pixelFormat,
      audioCodec: profile.audioCodec,
      audioRate: profile.audioRate,
      audioChannels: profile.audioChannels,
      audioBitrate: profile.audioBitrate,
      qualified: profile.qualified,
    },
    copies,
    progress: response.progress,
    failure: response.failure,
    jobId: response.jobId,
  }
}

export async function beginClipAnalysisPreparation(
  transport: Transport,
  input: ClipAnalysisPreparationInput,
  signal?: AbortSignal,
) {
  if (input.originals.some((original) => original.provenance !== 'browser_client'))
    throw new Error('Invalid original measurement provenance')
  return toClipAnalysisPreparation(
    await createClient(ClipSourceService, transport).beginClipAnalysisPreparation(
      {
        projectId: input.projectId,
        batchId: input.batchId,
        expectedRevision: input.expectedRevision,
        quoteId: input.quoteId,
        profileVersion: input.profileVersion,
        originals: input.originals.map((original) => ({
          sourceId: original.sourceId,
          fingerprint: original.fingerprint,
          durationMs: original.durationMs,
          width: original.width,
          height: original.height,
          frameRateNumerator: original.frameRateNumerator,
          frameRateDenominator: original.frameRateDenominator,
          cadenceVerified: original.cadenceVerified,
          decodedFrames: original.decodedFrames,
          hasAudio: original.hasAudio,
          audioRate: original.audioRate,
          audioChannels: original.audioChannels,
        })),
      },
      { signal },
    ),
  )
}
export async function reserveClipAnalysisCopy(
  transport: Transport,
  input: { preparationId: string; slot: string; bytes: number; sha256: string },
  signal?: AbortSignal,
) {
  if (
    !Number.isSafeInteger(input.bytes) ||
    input.bytes < 1 ||
    input.bytes > 8 * 1024 * 1024 ||
    !sha256.test(input.sha256)
  )
    throw new Error('Invalid analysis copy reservation')
  const response = await createClient(ClipSourceService, transport).reserveClipAnalysisCopy(
    { ...input, bytes: BigInt(input.bytes) },
    { signal },
  )
  if (
    response.slot !== input.slot ||
    !response.putUrl ||
    !Number.isFinite(Date.parse(response.expiresAt)) ||
    response.headers['If-None-Match'] !== '*'
  )
    throw new Error('Invalid private analysis copy upload')
  return {
    slot: response.slot,
    url: response.putUrl,
    headers: response.headers,
    expiresAt: response.expiresAt,
  }
}
export async function completeClipAnalysisPreparation(
  transport: Transport,
  preparationId: string,
  signal?: AbortSignal,
) {
  return toClipAnalysisPreparation(
    await createClient(ClipSourceService, transport).completeClipAnalysisPreparation(
      { preparationId },
      { signal },
    ),
  )
}
export async function cancelClipAnalysisPreparation(
  transport: Transport,
  preparationId: string,
  signal?: AbortSignal,
) {
  return toClipAnalysisPreparation(
    await createClient(ClipSourceService, transport).cancelClipAnalysisPreparation(
      { preparationId },
      { signal },
    ),
  )
}

export function clipAnalysisPreparationCalls(transport: Transport) {
  return {
    begin: (input: ClipAnalysisPreparationInput, signal: AbortSignal) =>
      beginClipAnalysisPreparation(transport, input, signal),
    reserve: (
      input: { preparationId: string; slot: string; bytes: number; sha256: string },
      signal: AbortSignal,
    ) => reserveClipAnalysisCopy(transport, input, signal),
    complete: (id: string, signal: AbortSignal) =>
      completeClipAnalysisPreparation(transport, id, signal),
    cancel: (id: string, signal?: AbortSignal) =>
      cancelClipAnalysisPreparation(transport, id, signal),
  }
}

export function useClipAnalysisPreparationCalls() {
  const transport = useTransport()
  return useMemo(() => clipAnalysisPreparationCalls(transport), [transport])
}
