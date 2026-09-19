import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ClipService } from '@/shared/api'
import { clipProjectsKey, toClipProject } from './clip-project'
import type { ClipProject } from '../model/types'
import {
  getClipSourcePlayback,
  getClipSources,
  reorderClipSources,
  setClipSourceOriginalSound,
} from './sources'
import { toClipSourceBatch } from './clip-project'

/** The project's own rpcs as a consumer asks for them (ARCH-17): the transport, the descriptor
 *  and the proto↔domain mapping stay here, and a feature reaches them through a hook. */
export interface ClipProjectCalls {
  /** The server's full projection of one project — editing, notices, finalization and all. */
  fetch(projectId: string, signal?: AbortSignal): Promise<ClipProject>
  /** Starting the AI run the owner approved a quote for. */
  startGeneration(input: {
    projectId: string
    batchId: string
    observeModel: { providerId: string; modelId: string }
    writeModel: { providerId: string; modelId: string }
    quoteId: string
    approvedMaxCredits: number
    cancellationPolicyVersion?: number
  }): Promise<{ jobId: string }>
}

export function clipProjectCalls(transport: Transport): ClipProjectCalls {
  const client = createClient(ClipService, transport)
  return {
    async fetch(projectId, signal) {
      const response = await client.getClipProject({ id: projectId }, { signal })
      if (!response.project) throw new Error('Missing owned clip')
      return toClipProject(response.project)
    },
    async startGeneration(input) {
      const response = await client.startClipGeneration(input)
      if (!response.jobId) throw new Error('Missing durable clip job')
      return { jobId: response.jobId }
    },
  }
}

export function useClipProjectCalls(): ClipProjectCalls {
  const transport = useTransport()
  return useMemo(() => clipProjectCalls(transport), [transport])
}

/** The cache scope every clip query hangs under. A feature that has to name it (a job poll that
 *  invalidates the project it ran on) asks the entity rather than the transport. */
export function useClipProjectsKey(ownerId: string) {
  const transport = useTransport()
  return useMemo(() => clipProjectsKey(transport, ownerId), [transport, ownerId])
}

/** One entry for "this project changed, read it again" (ARCH-14). */
export function useRefreshClipProjects(ownerId: string) {
  const key = useClipProjectsKey(ownerId)
  const cache = useQueryClient()
  return useMemo(
    () => ({
      all: () => cache.invalidateQueries({ queryKey: key }),
      detail: (projectId: string) =>
        cache.invalidateQueries(
          { queryKey: [...key, 'detail', projectId] },
          { throwOnError: true },
        ),
    }),
    [cache, key],
  )
}

/** The source batch's rpcs: reserving an upload, confirming a part, discarding the batch, and
 *  the two reads a session needs to play footage it did not upload itself. */
export interface ClipSourceCalls {
  retained(projectId: string, signal?: AbortSignal): ReturnType<typeof getClipSources>
  playback(
    projectId: string,
    sourceId: string,
    fingerprint: string,
    signal?: AbortSignal,
  ): ReturnType<typeof getClipSourcePlayback>
  reserve(
    projectId: string,
    sources: readonly {
      filename: string
      contentType: string
      bytes: number
      durationMs: number
      width: number
      height: number
      fingerprint: string
    }[],
    signal?: AbortSignal,
  ): Promise<{
    batch: ReturnType<typeof toClipSourceBatch>
    uploads: { sourceId: string; putUrl: string; headers: Record<string, string> }[]
  }>
  confirm(
    batchId: string,
    sourceId: string,
    signal?: AbortSignal,
  ): Promise<ReturnType<typeof toClipSourceBatch>>
  discard(batchId: string): Promise<void>
  sound: (
    input: Parameters<typeof setClipSourceOriginalSound>[1],
    signal?: AbortSignal,
  ) => ReturnType<typeof setClipSourceOriginalSound>
  reorder: (
    input: Parameters<typeof reorderClipSources>[1],
    signal?: AbortSignal,
  ) => ReturnType<typeof reorderClipSources>
}

export function clipSourceCalls(transport: Transport): ClipSourceCalls {
  const client = createClient(ClipService, transport)
  return {
    retained: (projectId, signal) => getClipSources(transport, projectId, signal),
    playback: (projectId, sourceId, fingerprint, signal) =>
      getClipSourcePlayback(transport, projectId, sourceId, fingerprint, signal),
    async reserve(projectId, sources, signal) {
      // Deliberately project each metadata field. No File, Blob or preview URL crosses Connect.
      const response = await client.createClipSourceBatch(
        {
          projectId,
          sources: sources.map((m) => ({
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
    async confirm(batchId, sourceId, signal) {
      const response = await client.confirmClipSource({ batchId, sourceId }, { signal })
      if (!response.batch) throw new Error('Missing confirmed sources')
      return toClipSourceBatch(response.batch)
    },
    async discard(batchId) {
      await client.discardClipSourceBatch({ batchId })
    },
    sound: (input, signal) => setClipSourceOriginalSound(transport, input, signal),
    reorder: (input, signal) => reorderClipSources(transport, input, signal),
  }
}

export function useClipSourceCalls(): ClipSourceCalls {
  const transport = useTransport()
  return useMemo(() => clipSourceCalls(transport), [transport])
}
