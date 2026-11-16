import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import { GenerationService, ModelRefSchema } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import type { ModelRef } from '../model/types'

/** ②'s AI revision: one durable `revise` job over the post's canonical content. It changes the post
 *  and nothing else — no voice learns from it — so no other cache goes stale when it starts. */
export function useStartRevision() {
  const mutation = useMutation(GenerationService.method.startRevision)

  return {
    ...mutation,
    errorMessage: connectFailureMessage(mutation.error),
    start: (postSlug: string, instruction: string, writeModel: ModelRef) =>
      mutation.mutateAsync({
        postSlug,
        instruction,
        writeModel: create(ModelRefSchema, writeModel),
      }),
  }
}
