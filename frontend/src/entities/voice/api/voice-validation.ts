import { create } from '@bufbuild/protobuf'
import { createClient } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, ModelRefSchema, VoiceValidationService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  voiceComparisonQueryKey,
  voiceProfileQueryKey,
  voiceValidationQueryKey,
  voiceVersionsQueryKey,
} from './voice-queries'

/** A validation and a rule comparison are both "run this voice's profile against its own sources
 *  and show me what changes". They read and write nothing but the voice, so the descriptors live
 *  here and the screens that start, poll and decide them take these hooks (ARCH-17). */

const running = (status: string | undefined) => ['queued', 'running'].includes(status ?? '')

export function useStartVoiceProfileValidation() {
  const mutation = useMutation(VoiceValidationService.method.startVoiceProfileValidation)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    start: (input: {
      voiceId: string
      analyzeModel: { providerId: string; modelId: string }
      writeModel: { providerId: string; modelId: string }
      judgeEnabled: boolean
    }) =>
      mutation.mutateAsync({
        voiceId: input.voiceId,
        analyzeModel: create(ModelRefSchema, input.analyzeModel),
        writeModel: create(ModelRefSchema, input.writeModel),
        judgeEnabled: input.judgeEnabled,
      }),
  }
}

export function useStartVoiceRuleComparison() {
  const mutation = useMutation(VoiceValidationService.method.startVoiceRuleComparison)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    start: (input: {
      ruleId: string
      sourceId: string
      writeModel: { providerId: string; modelId: string }
    }) =>
      mutation.mutateAsync({
        ruleId: input.ruleId,
        sourceId: input.sourceId,
        writeModel: create(ModelRefSchema, input.writeModel),
      }),
  }
}

/** One validation, polled while it runs, with the retry that restarts it. The query key is the
 *  screen's own invalidation target — the job it names finishes into this entry. */
export function useVoiceProfileValidation(ownerId: string, voiceId: string, validationId: string) {
  const transport = useTransport()
  const queryKey = voiceValidationQueryKey(transport, ownerId, voiceId, validationId)
  const query = useQuery({
    queryKey,
    queryFn: () =>
      createClient(VoiceValidationService, transport).getVoiceProfileValidation({ validationId }),
    refetchInterval: (state) =>
      running(state.state.data?.validation?.status) ? POLL_INTERVAL_MS : false,
  })
  const retryMutation = useMutation(VoiceValidationService.method.retryVoiceProfileValidation)
  return {
    queryKey,
    validation: query.data?.validation,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => query.refetch(),
    retryFailure: retryMutation.error ? appFailureFromConnect(retryMutation.error) : undefined,
    retryPending: retryMutation.isPending,
    retry: async () => {
      await retryMutation.mutateAsync({ validationId })
      await query.refetch()
    },
  }
}

/** One rule comparison, polled while it runs, with the decide and retry verbs the screen offers. */
export function useVoiceRuleComparison(ownerId: string, voiceId: string, comparisonId: string) {
  const transport = useTransport()
  const queryKey = voiceComparisonQueryKey(transport, ownerId, voiceId, comparisonId)
  const query = useQuery({
    queryKey,
    queryFn: () =>
      createClient(VoiceValidationService, transport).getVoiceRuleComparison({ comparisonId }),
    refetchInterval: (state) =>
      running(state.state.data?.comparison?.status) ? POLL_INTERVAL_MS : false,
  })
  const queryClient = useQueryClient()
  const decideMutation = useMutation(VoiceValidationService.method.decideVoiceRuleComparison)
  const retryMutation = useMutation(VoiceValidationService.method.retryVoiceRuleComparison)
  return {
    queryKey,
    comparison: query.data?.comparison,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => query.refetch(),
    decidePending: decideMutation.isPending,
    decideFailure: decideMutation.error ? appFailureFromConnect(decideMutation.error) : undefined,
    decide: async (chosenSide: string) => {
      await decideMutation.mutateAsync({ comparisonId, chosenSide })
      // The decision publishes to the comparison's own voice, so only that voice's profile and
      // version list are stale.
      for (const stale of [
        queryKey,
        voiceProfileQueryKey(transport, ownerId, voiceId),
        voiceVersionsQueryKey(transport, ownerId, voiceId),
      ]) {
        await queryClient.invalidateQueries({ queryKey: stale })
      }
    },
    retryPending: retryMutation.isPending,
    retryFailure: retryMutation.error ? appFailureFromConnect(retryMutation.error) : undefined,
    retry: async () => {
      await retryMutation.mutateAsync({ comparisonId })
      await query.refetch()
    },
  }
}
