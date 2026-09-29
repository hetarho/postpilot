import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import {
  appFailureFromConnect,
  appFailureFromProto,
  ModelRefSchema,
  type ProtoVoiceCheck,
  VoiceService,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { VoiceCheck } from '../model/check'
import { toComparisons } from './fingerprint-comparison'
import { requireCheckStatus, requirePromptPart } from './voice-enums'
import { voiceChecksQueryKey } from './voice-queries'

type ModelRefInput = { providerId: string; modelId: string }

export function toVoiceCheck(check: ProtoVoiceCheck): VoiceCheck {
  return {
    id: check.id,
    prompt: check.prompt && {
      key: check.prompt.key,
      part: requirePromptPart(check.prompt.part),
      photo: check.prompt.photo,
      text: check.prompt.text,
    },
    answer: check.answer,
    answerDeleted: check.answerDeleted,
    status: requireCheckStatus(check.status),
    piece: check.piece,
    failure: check.failure ? appFailureFromProto(check.failure) : undefined,
    comparison: toComparisons(check.comparison),
    stale: check.stale,
    createdAt: check.createdAt,
  }
}

/** The voice's 검증 results newest first, and its queued or running check job so a reload
 *  resumes polling (VOICE-31, VOICE-43). */
export function useVoiceChecks(
  ownerId: string,
  voiceId: string,
): {
  checks: VoiceCheck[]
  activeJobId: string
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceChecksQueryKey(transport, ownerId, voiceId),
    queryFn: () => createClient(VoiceService, transport).listVoiceChecks({ voiceId }),
    enabled: ownerId !== '' && voiceId !== '',
  })
  const checks = useMemo(() => query.data?.checks.map(toVoiceCheck) ?? [], [query.data])
  return {
    checks,
    activeJobId: query.data?.activeJobId ?? '',
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}

/** The query key a finished check job refreshes. */
export function useVoiceChecksQueryKey(ownerId: string, voiceId: string) {
  const transport = useTransport()
  return useMemo(
    () => voiceChecksQueryKey(transport, ownerId, voiceId),
    [ownerId, transport, voiceId],
  )
}

/** 검증하기 on one prompt with the account's write selection (VOICE-43). */
export function useStartVoiceCheck(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.startVoiceCheck, {
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: voiceChecksQueryKey(transport, ownerId, voiceId) }),
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    isPending: mutation.isPending,
    isError: mutation.isError,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    reset: mutation.reset,
    start: async (promptKey: string, model: ModelRefInput) => {
      const response = await mutation.mutateAsync({
        voiceId,
        promptKey,
        model: create(ModelRefSchema, model),
      })
      return { jobId: response.jobId }
    },
  }
}

/** A failed 검증's retry: a new check on the same prompt (VOICE-44). */
export function useRetryVoiceCheck(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.retryVoiceCheck, {
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: voiceChecksQueryKey(transport, ownerId, voiceId) }),
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    isPending: mutation.isPending,
    isError: mutation.isError,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    retry: async (checkId: string, model: ModelRefInput) => {
      const response = await mutation.mutateAsync({ checkId, model: create(ModelRefSchema, model) })
      return { jobId: response.jobId }
    },
  }
}
