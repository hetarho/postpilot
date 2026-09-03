import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import {
  appFailureFromConnect,
  GenerationService,
  ModelRefSchema,
  ReobserveSelectionSchema,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useStartGeneration() {
  const mutation = useMutation(GenerationService.method.startGeneration)

  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    /** `reobserveFiles` carries PRESENCE: `undefined` is a start with no re-observation
     *  decision, which observes every attached photo, while an EMPTY array is the decision to
     *  observe nothing and reuse every stored observation. Collapsing the two would turn a
     *  reuse-everything confirmation back into a full re-observation. */
    start: (
      postSlug: string,
      observeModel: ModelRef | undefined,
      writeModel: ModelRef,
      targetLength?: number,
      reobserveFiles?: readonly string[],
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        writeModel: create(ModelRefSchema, writeModel),
        targetLength,
        reobserve: reobserveFiles
          ? create(ReobserveSelectionSchema, { files: [...reobserveFiles] })
          : undefined,
      }),
  }
}
