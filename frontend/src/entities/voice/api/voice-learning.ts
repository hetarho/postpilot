import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  appFailureFromConnect,
  ModelRefSchema,
  VoiceFeedbackReason,
  VoiceLearningService,
  type VoiceRuleStatus,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import {
  voiceConfirmationsQueryKey,
  voiceProfileQueryKey,
  voiceVersionsQueryKey,
} from './voice-queries'

/** What a finalized post teaches the voice. The evidence is the post's, but everything it writes
 *  is the voice's profile, so the calls live with the voice entity (ARCH-14) and the panels that
 *  render the outcome stay in `features/finalize-post`. */
export function useVoiceLearningActions() {
  const learnMutation = useMutation(VoiceLearningService.method.learnFromFinalizedPost)
  const retryMutation = useMutation(VoiceLearningService.method.retryVoiceLearning)
  const feedbackMutation = useMutation(VoiceLearningService.method.giveSentenceFeedback)
  const model = (ref: { providerId: string; modelId: string }) => create(ModelRefSchema, ref)
  return {
    learn: (postSlug: string, analyzeModel: { providerId: string; modelId: string }) =>
      learnMutation.mutateAsync({ postSlug, analyzeModel: model(analyzeModel) }),
    retry: (eventId: string, analyzeModel: { providerId: string; modelId: string }) =>
      retryMutation.mutateAsync({ eventId, analyzeModel: model(analyzeModel) }),
    satisfy: (postSlug: string) =>
      feedbackMutation.mutateAsync({
        postSlug,
        satisfaction: true,
        reason: VoiceFeedbackReason.UNSPECIFIED,
      }),
    pending: learnMutation.isPending || retryMutation.isPending,
    feedbackPending: feedbackMutation.isPending,
    errorMessage: formatMutationFailure(
      learnMutation.error ?? retryMutation.error ?? feedbackMutation.error,
    ),
  }
}

/** One sentence the owner marked as off-voice. */
export function useSentenceFeedback() {
  const mutation = useMutation(VoiceLearningService.method.giveSentenceFeedback)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    give: (postSlug: string, sentence: string, reason: VoiceFeedbackReason) =>
      mutation.mutateAsync({
        postSlug,
        sentenceRef: sentence,
        authoredText: sentence,
        reason,
      }),
  }
}

/** A rule's status and its pending conflicts. Every outcome republishes the profile, so the
 *  profile, its versions and the confirmation list are stale together. */
export function useVoiceRuleActions(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const statusMutation = useMutation(VoiceLearningService.method.setVoiceRuleStatus)
  const resolveMutation = useMutation(VoiceLearningService.method.resolveRuleConfirmation)
  const refresh = () => {
    for (const queryKey of [
      voiceProfileQueryKey(transport, ownerId, voiceId),
      voiceVersionsQueryKey(transport, ownerId, voiceId),
      voiceConfirmationsQueryKey(transport, ownerId, voiceId),
    ]) {
      void queryClient.invalidateQueries({ queryKey })
    }
  }
  return {
    error: statusMutation.error ?? resolveMutation.error,
    pending: statusMutation.isPending || resolveMutation.isPending,
    setStatus: (ruleId: string, status: VoiceRuleStatus) =>
      statusMutation.mutateAsync({ ruleId, status }).then(refresh),
    resolve: (confirmationId: string, replace: boolean) =>
      resolveMutation.mutateAsync({ confirmationId, replace }).then(refresh),
  }
}

function formatMutationFailure(error: Error | null): string {
  return error ? formatAppFailure(appFailureFromConnect(error)) : ''
}
