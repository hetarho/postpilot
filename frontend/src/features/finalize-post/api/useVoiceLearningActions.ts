import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { ModelRefSchema, VoiceFeedbackReason, VoiceLearningService } from '@/shared/api'

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
    errorMessage: learnMutation.error
      ? ConnectError.from(learnMutation.error).rawMessage
      : retryMutation.error
        ? ConnectError.from(retryMutation.error).rawMessage
        : feedbackMutation.error
          ? ConnectError.from(feedbackMutation.error).rawMessage
          : '',
  }
}
