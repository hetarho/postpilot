import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import {
  ModelRefSchema,
  VoiceFeedbackReason,
  VoiceLearningService,
} from '@/shared/api'

export function useVoiceLearningActions() {
  const finalizeMutation = useMutation(VoiceLearningService.method.finalizeAndLearn)
  const retryMutation = useMutation(VoiceLearningService.method.retryVoiceLearning)
  const feedbackMutation = useMutation(VoiceLearningService.method.giveSentenceFeedback)
  const model = (ref: ModelRef) => create(ModelRefSchema, ref)
  return {
    finalize: (postSlug: string, analyzeModel: ModelRef) =>
      finalizeMutation.mutateAsync({ postSlug, analyzeModel: model(analyzeModel) }),
    retry: (eventId: string, analyzeModel: ModelRef) =>
      retryMutation.mutateAsync({ eventId, analyzeModel: model(analyzeModel) }),
    satisfy: (postSlug: string) =>
      feedbackMutation.mutateAsync({
        postSlug,
        satisfaction: true,
        reason: VoiceFeedbackReason.UNSPECIFIED,
      }),
    pending: finalizeMutation.isPending || retryMutation.isPending,
    feedbackPending: feedbackMutation.isPending,
    errorMessage: finalizeMutation.error
      ? ConnectError.from(finalizeMutation.error).rawMessage
      : retryMutation.error
        ? ConnectError.from(retryMutation.error).rawMessage
        : feedbackMutation.error
          ? ConnectError.from(feedbackMutation.error).rawMessage
          : '',
  }
}
