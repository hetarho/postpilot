import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { appFailureFromProto, type ProtoVoiceCheck, VoiceService } from '@/shared/api'
import type { VoiceCheck } from '../model/check'
import { toComparisons } from './fingerprint-comparison'
import { requireCheckStatus, requirePromptPart } from './voice-enums'
import { voiceChecksQueryKey } from './voice-queries'

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
