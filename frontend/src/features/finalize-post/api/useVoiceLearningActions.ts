import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import {
  appFailureFromConnect,
  ModelRefSchema,
  VoiceFeedbackReason,
  VoiceLearningService,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useVoiceLearningActions() {
  const learnMutation = useMutation(VoiceLearningService.method.learnFromFinalizedPost)
  const retryMutation = useMutation(VoiceLearningService.method.retryVoiceLearning)
  const feedbackMutation = useMutation(VoiceLearningService.method.giveSentenceFeedback)
  const model = (ref: ModelRef) => create(ModelRefSchema, ref)
  return {
    learn: (postSlug: string, analyzeModel: ModelRef) =>
      learnMutation.mutateAsync({ postSlug, analyzeModel: model(analyzeModel) }),
    retry: (eventId: string, analyzeModel: ModelRef) =>
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

function formatMutationFailure(error: Error | null): string {
  return error ? formatAppFailure(appFailureFromConnect(error)) : ''
}
