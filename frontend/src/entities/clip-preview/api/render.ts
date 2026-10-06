import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { ClipRenderKind, ClipRenderService } from '@/shared/api'
import { toClipProject, type ClipProject } from '@/entities/clip-project/@x/clip-preview'

/** Which machine makes the file. The kind is a domain word here; the proto enum stays inside
 *  this adapter (ARCH-3, ARCH-17). */
export type ClipRenderMachine = 'server' | 'browser'

/** What the browser measured about the file it produced, exactly as the server checks it. */
export interface ClipRenderVerdict {
  passed: boolean
  measurements: {
    compositionVersion?: string
    snapshotFingerprint?: string
    backgroundVersion?: string
    backgroundSnapshotFingerprint?: string
    backgroundDigest?: string
    backgroundComplete?: boolean
    backgroundSampleCount?: number
    backgroundNotices?: { code: string; action: string; elementId?: string; cutId?: string }[]
    width: number
    height: number
    frameRateNumerator: number
    frameRateDenominator: number
    videoFrames: number
    videoCodec: string
    videoProfile: string
    hasAudio: boolean
    audioCodec: string
    audioRate: number
    loudnessLufs?: number
    truePeakDbtp?: number
    silent: boolean
    speechFingerprint?: string
  }
}

/** The render family's rpcs (ARCH-17): admitting a render, cancelling one the browser owns, and
 *  the three steps that store what the browser encoded. */
export interface ClipRenderCalls {
  admitComposition?(input: {
    projectId: string
    expectedRevision: number
    batchId: string
    compositionVersion: string
  }): Promise<{
    renderId: string
    compositionVersion: string
    snapshotFingerprint: string
    componentVersion: string
    fontVersion: string
    assetVersion: string
  }>
  admit(input: {
    projectId: string
    expectedRevision: number
    batchId: string
    machine: ClipRenderMachine
    reuseExisting?: boolean
  }): Promise<{ renderId: string; jobId: string; reusedResult: boolean }>
  cancelBrowserRender(renderId: string): Promise<boolean>
  prepareUpload(
    renderId: string,
    bytes: number,
    signal: AbortSignal,
  ): Promise<{ putUrl: string; headers: Record<string, string> }>
  reportVerdict(
    renderId: string,
    verdict: ClipRenderVerdict,
    signal: AbortSignal,
  ): Promise<{ passed: boolean; notices: { code: string; action: string }[] }>
  completeUpload(renderId: string, signal: AbortSignal): Promise<ClipProject>
}

export function clipRenderCalls(transport: Transport): ClipRenderCalls {
  const client = createClient(ClipRenderService, transport)
  return {
    admitComposition: (input) => client.startClipBrowserCompositionRender(input),
    async admit({ machine, ...input }) {
      // Settle admission even if the page leaves, so a late identity can be cancelled.
      const response = await client.startClipRender({
        ...input,
        renderKind: machine === 'browser' ? ClipRenderKind.BROWSER : ClipRenderKind.SERVER,
        reuseExisting: input.reuseExisting ?? false,
      })
      return {
        renderId: response.renderId,
        jobId: response.jobId,
        reusedResult: response.reusedResult,
      }
    },
    cancelBrowserRender: async (renderId) =>
      (await client.cancelClipBrowserRender({ renderId })).cancelled,
    prepareUpload: (renderId, bytes, signal) =>
      client.prepareClipRenderUpload({ renderId, bytes: BigInt(bytes) }, { signal }),
    reportVerdict: (renderId, verdict, signal) =>
      client.reportClipRenderVerdict({ renderId, ...verdict }, { signal }),
    async completeUpload(renderId, signal) {
      const response = await client.completeClipRenderUpload({ renderId }, { signal })
      if (!response.project) throw new Error('Missing stored render')
      return toClipProject(response.project)
    },
  }
}

export function useClipRenderCalls(): ClipRenderCalls {
  const transport = useTransport()
  return useMemo(() => clipRenderCalls(transport), [transport])
}
