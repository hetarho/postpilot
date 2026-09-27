import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import { appFailureFromConnect, GenerationService, ModelRefSchema } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { ModelRef } from '../model/types'

/** The storyline space's AI request (GEN-69): a job that rewrites the stored storyline from the
 *  owner's request. It observes nothing, so it takes the write model alone. */
export function useStartStorylineRevision() {
  const mutation = useMutation(GenerationService.method.startStorylineRevision)
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    start: (postSlug: string, request: string, writeModel: ModelRef) =>
      mutation.mutateAsync({ postSlug, request, writeModel: create(ModelRefSchema, writeModel) }),
  }
}
