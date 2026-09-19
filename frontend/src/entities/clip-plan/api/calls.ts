import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { ClipPlanService } from '@/shared/api'
import {
  toClipProject,
  toClipRevisionQuote,
  type ClipProject,
  type ClipQuote,
} from '@/entities/clip-project/@x/clip-plan'
import { clipPlanToProto } from './edit-plan'
import type { ClipEditPlan } from '../model/edit-plan'
import type { ClipRevisionTarget } from '../model/revision'

/** The plan's own rpcs (ARCH-17): saving a correction, and asking for one to be rewritten. */
export interface ClipPlanCalls {
  /** Saves the draft against the revision it was edited from; the answer is the whole project,
   *  because a save moves the project's revision and its notices with it. */
  save(input: {
    projectId: string
    expectedRevision: number
    plan: ClipEditPlan
  }): Promise<ClipProject>
  /** Starts a paid rewrite of the saved plan. */
  revise(input: {
    projectId: string
    request: string
    target: ClipRevisionTarget
    observeModel: { providerId: string; modelId: string }
    writeModel: { providerId: string; modelId: string }
    quote: ClipQuote
  }): Promise<{ jobId: string }>
}

export function clipPlanCalls(transport: Transport): ClipPlanCalls {
  const client = createClient(ClipPlanService, transport)
  return {
    async save({ projectId, expectedRevision, plan }) {
      const result = await client.saveClipEditPlan({
        projectId,
        expectedRevision,
        plan: clipPlanToProto(plan),
      })
      if (!result.project?.editing) throw new Error('Missing saved correction')
      return toClipProject(result.project)
    },
    async revise({ quote, ...input }) {
      const response = await client.startClipRevision({
        ...input,
        quoteId: quote.quoteId,
        approvedMaxCredits: quote.maxCredits,
        cancellationPolicyVersion: quote.cancellationPolicy?.version,
      })
      if (!response.jobId) throw new Error('Missing durable clip job')
      return { jobId: response.jobId }
    },
  }
}

export function useClipPlanCalls(): ClipPlanCalls {
  const transport = useTransport()
  return useMemo(() => clipPlanCalls(transport), [transport])
}

/** The price of a rewrite. Like a generation quote it binds the exact words it was taken
 *  against (CLIP-131), so the binding is the identity and nothing is served from a cache. */
export function useClipRevisionQuote(
  ownerId: string,
  input: {
    projectId: string
    request: string
    target: ClipRevisionTarget
    observeModel?: { providerId: string; modelId: string } | null
    writeModel?: { providerId: string; modelId: string } | null
  },
  binding: string,
  enabled: boolean,
) {
  const transport = useTransport()
  return useQuery({
    queryKey: ['clip-revision-quote', transport, ownerId, binding],
    enabled,
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    queryFn: async ({ signal }) =>
      toClipRevisionQuote(
        await createClient(ClipPlanService, transport).quoteClipRevision(
          {
            projectId: input.projectId,
            request: input.request,
            target: input.target,
            observeModel: input.observeModel ?? undefined,
            writeModel: input.writeModel ?? undefined,
          },
          { signal },
        ),
        binding,
      ),
  })
}
