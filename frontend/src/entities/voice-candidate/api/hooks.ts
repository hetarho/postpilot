import { createClient, type Transport } from '@connectrpc/connect'
import { createConnectQueryKey, useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import { toVoice, voicesQueryKey } from '@/entities/voice/@x/voice-candidate'
import { appFailureFromConnect, WritingVoiceCandidateService } from '@/shared/api'
import {
  completeCandidateBatch,
  WRITING_VOICE_CANDIDATE_COUNT,
  type WritingVoiceCandidateBatch,
  type WritingVoiceCandidateCount,
} from '../model/types'

interface CandidateModelRef {
  providerId: string
  modelId: string
}
export function latestWritingVoiceCandidatesQueryKey(transport: Transport, ownerId: string) {
  return [
    ...createConnectQueryKey({
      schema: WritingVoiceCandidateService.method.getLatestWritingVoiceCandidates,
      input: {},
      transport,
      cardinality: 'finite',
    }),
    ownerId,
  ] as const
}
function useCandidateClient() {
  const transport = useTransport()
  return useMemo(() => createClient(WritingVoiceCandidateService, transport), [transport])
}
export function useLatestWritingVoiceCandidates(ownerId: string) {
  const client = useCandidateClient()
  const transport = useTransport()
  const queryKey = useMemo(
    () => latestWritingVoiceCandidatesQueryKey(transport, ownerId),
    [transport, ownerId],
  )
  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }) => {
      const response = await client.getLatestWritingVoiceCandidates({}, { signal })
      if (response.resultJobId && !completeCandidateBatch(response.candidates))
        throw new Error('writing candidate batch unavailable')
      return response
    },
    enabled: !!ownerId,
  })
  const batch = useMemo<WritingVoiceCandidateBatch | undefined>(
    () =>
      query.data
        ? {
            jobId: query.data.jobId,
            resultJobId: query.data.resultJobId,
            candidates: query.data.candidates.map(({ id, name, description, sample }) => ({
              id,
              name,
              description,
              sample,
            })),
          }
        : undefined,
    [query.data],
  )
  return {
    batch,
    queryKey,
    isPending: query.isPending,
    isError: query.isError,
    failure: query.error ? appFailureFromConnect(query.error) : undefined,
    refetch: () => void query.refetch(),
  }
}
export function useWritingVoiceCandidates(ownerId: string, jobId: string) {
  const client = useCandidateClient()
  const transport = useTransport()
  const query = useQuery({
    queryKey: [
      ...createConnectQueryKey({
        schema: WritingVoiceCandidateService.method.getWritingVoiceCandidates,
        input: { jobId },
        transport,
        cardinality: 'finite',
      }),
      ownerId,
    ],
    queryFn: async ({ signal }) => {
      const response = await client.getWritingVoiceCandidates({ jobId }, { signal })
      if (response.jobId !== jobId || !completeCandidateBatch(response.candidates))
        throw new Error('writing candidate batch unavailable')
      return response
    },
    enabled: !!ownerId && !!jobId,
  })
  return {
    candidates:
      query.data?.candidates.map(({ id, name, description, sample }) => ({
        id,
        name,
        description,
        sample,
      })) ?? [],
    isPending: query.isPending,
    isError: query.isError,
    failure: query.error ? appFailureFromConnect(query.error) : undefined,
  }
}
export function useEstimateWritingVoiceCandidates(
  ownerId: string,
  model: CandidateModelRef | undefined,
  candidateCount: WritingVoiceCandidateCount = WRITING_VOICE_CANDIDATE_COUNT,
) {
  const client = useCandidateClient()
  const transport = useTransport()
  const query = useQuery({
    queryKey: [
      ...createConnectQueryKey({
        schema: WritingVoiceCandidateService.method.estimateWritingVoiceCandidates,
        input: { writeModel: model, candidateCount },
        transport,
        cardinality: 'finite',
      }),
      ownerId,
    ],
    queryFn: ({ signal }) =>
      client.estimateWritingVoiceCandidates({ writeModel: model, candidateCount }, { signal }),
    enabled: !!ownerId && !!model,
    staleTime: 0,
  })
  return {
    estimate: query.data ? { credits: query.data.credits, free: query.data.free } : undefined,
    isPending: query.isPending,
    isFetching: query.isFetching,
    isError: query.isError,
    failure: query.error ? appFailureFromConnect(query.error) : undefined,
    refetch: () => void query.refetch(),
  }
}
export function useStartWritingVoiceCandidates(ownerId: string) {
  const client = useCandidateClient()
  const transport = useTransport()
  const cache = useQueryClient()
  const mutation = useMutation({
    mutationFn: async (
      model: CandidateModelRef & { candidateCount?: WritingVoiceCandidateCount },
    ) => {
      const response = await client.startWritingVoiceCandidates({
        writeModel: model,
        candidateCount: model.candidateCount ?? WRITING_VOICE_CANDIDATE_COUNT,
      })
      if (!response.jobId) throw new Error('writing candidate job unavailable')
      return { jobId: response.jobId }
    },
    onSuccess: () =>
      void cache.invalidateQueries({
        queryKey: latestWritingVoiceCandidatesQueryKey(transport, ownerId),
      }),
  })
  return { start: mutation.mutateAsync }
}
export function useCancelWritingVoiceCandidates(ownerId: string) {
  const client = useCandidateClient()
  const transport = useTransport()
  const cache = useQueryClient()
  const mutation = useMutation({
    mutationFn: async (jobId: string) => {
      await client.cancelWritingVoiceCandidates({ jobId })
    },
    onSuccess: () =>
      void cache.invalidateQueries({
        queryKey: latestWritingVoiceCandidatesQueryKey(transport, ownerId),
      }),
  })
  return { cancel: mutation.mutateAsync }
}
export function useAdoptWritingVoiceCandidate(ownerId: string) {
  const client = useCandidateClient()
  const transport = useTransport()
  const cache = useQueryClient()
  const mutation = useMutation({
    mutationFn: async ({ jobId, candidateId }: { jobId: string; candidateId: string }) => {
      const response = await client.adoptWritingVoiceCandidate({
        jobId,
        candidateId,
        makeDefault: true,
      })
      const voice = toVoice(response.voice)
      if (!voice.id || !voice.made || voice.deleted)
        throw new Error('saved writing voice unavailable')
      return voice
    },
    onSuccess: () => {
      void cache.invalidateQueries({
        queryKey: latestWritingVoiceCandidatesQueryKey(transport, ownerId),
      })
      void cache.invalidateQueries({ queryKey: voicesQueryKey(transport, ownerId) })
    },
  })
  return { adopt: mutation.mutateAsync }
}
