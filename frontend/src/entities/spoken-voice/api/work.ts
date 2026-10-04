import { useEffect, useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { SpokenVoiceGenerationService, type ProtoSpokenOperation } from '@/shared/api'
import { SPOKEN_CANCELLATION_POLICY, SPOKEN_POLL_MS } from '../config/limits'
import {
  spokenOperationActive,
  type SpokenOperation,
  type SpokenOperationState,
  type SpokenWorkInput,
  type SpokenWorkKind,
  type SpokenWorkQuote,
} from '../model/types'
import { spokenScope } from './hooks'
const STATES: SpokenOperationState[] = [
  'reserved',
  'queued',
  'claimed',
  'received',
  'published',
  'failed',
  'unresolved',
  'cancelled',
]
const KINDS: SpokenWorkKind[] = ['voice_design', 'voice_confirm', 'voice_reuse_probe']
export function toSpokenOperation(p: ProtoSpokenOperation | undefined): SpokenOperation {
  if (
    !p ||
    !STATES.includes(p.state as SpokenOperationState) ||
    !KINDS.includes(p.kind as SpokenWorkKind)
  )
    throw new Error('Invalid spoken operation')
  return {
    id: p.id,
    kind: p.kind as SpokenWorkKind,
    state: p.state as SpokenOperationState,
    jobId: p.jobId,
    draftId: p.draftId,
    voiceId: p.voiceId,
    candidateId: p.candidateId,
    resultId: p.resultId,
    failureReason: p.failureReason,
  }
}
export function useSpokenOperation(ownerId: string, id: string) {
  const transport = useTransport(),
    cache = useQueryClient()
  const query = useQuery({
    queryKey: [...spokenScope(transport, ownerId), 'operation', id],
    enabled: !!ownerId && !!id,
    queryFn: async () =>
      toSpokenOperation(
        (await createClient(SpokenVoiceGenerationService, transport).getSpokenOperation({ id }))
          .operation,
      ),
    refetchInterval: (q) =>
      q.state.data && spokenOperationActive(q.state.data.state) ? SPOKEN_POLL_MS : false,
  })
  const operation = query.data
  useEffect(() => {
    if (operation && !spokenOperationActive(operation.state)) {
      void cache.invalidateQueries({ queryKey: [...spokenScope(transport, ownerId), 'draft'] })
      void cache.invalidateQueries({ queryKey: [...spokenScope(transport, ownerId), 'voices'] })
      void cache.invalidateQueries({ queryKey: [...spokenScope(transport, ownerId), 'drafts'] })
    }
  }, [operation, cache, transport, ownerId])
  return { ...query, operation }
}
export function useSpokenWorkActions(ownerId: string) {
  const transport = useTransport(),
    cache = useQueryClient(),
    client = useMemo(() => createClient(SpokenVoiceGenerationService, transport), [transport])
  const mutation = useMutation({
    mutationFn: async (fn: () => Promise<unknown>) => {
      if (!ownerId) throw new Error('Authentication required')
      return fn()
    },
    retry: false,
  })
  async function perform<T>(fn: () => Promise<T>): Promise<T> {
    let output: T | undefined
    await mutation.mutateAsync(async () => {
      output = await fn()
    })
    if (output === undefined) throw new Error('Missing spoken work response')
    return output
  }
  const quote = (input: SpokenWorkInput) =>
    perform(async () => {
      const payload = {
        draftId: input.draftId,
        expectedRevision: input.revision,
        candidateId: input.candidateId ?? '',
      }
      const q =
        input.kind === 'voice_design'
          ? await client.quoteVoiceCandidates(payload)
          : await client.quoteVoiceConfirmation(payload)
      return {
        id: q.quoteId,
        maximumCredits: q.maximumCredits,
        expiresAt: q.expiresAt,
        approvalRequired: q.approvalRequired,
        existingVoiceId: q.existingVoiceId,
      } satisfies SpokenWorkQuote
    })
  const start = (input: SpokenWorkInput, key: string, q: SpokenWorkQuote) =>
    perform(async () => {
      const payload = {
        input: {
          draftId: input.draftId,
          expectedRevision: input.revision,
          candidateId: input.candidateId ?? '',
        },
        idempotencyKey: key,
        approval: q.approvalRequired
          ? {
              quoteId: q.id,
              approvedMaxCredits: q.maximumCredits,
              cancellationPolicyVersion: SPOKEN_CANCELLATION_POLICY,
            }
          : undefined,
      }
      const r =
        input.kind === 'voice_design'
          ? await client.startVoiceCandidates(payload)
          : await client.startVoiceConfirmation(payload)
      const op = toSpokenOperation(r.operation)
      if (op.id) cache.setQueryData([...spokenScope(transport, ownerId), 'operation', op.id], op)
      return op
    })
  const change = (id: string, recover: boolean) =>
    perform(async () => {
      const r = recover
        ? await client.retrySpokenPublication({ id })
        : await client.cancelSpokenOperation({ id })
      const o = toSpokenOperation(r.operation)
      cache.setQueryData([...spokenScope(transport, ownerId), 'operation', id], o)
      return o
    })
  return {
    quote,
    start,
    cancel: (id: string) => change(id, false),
    recover: (id: string) => change(id, true),
    pending: mutation.isPending,
    error: mutation.error,
  }
}
