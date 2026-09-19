import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import {
  appFailureFromConnect,
  GenerationService,
  ModelRefSchema,
  ReobserveSelectionSchema,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { ModelRef } from '../model/types'

/** Starting a run is the job noun's own create (ARCH-14): the screens that choose the models and
 *  the photos render around it, and the proto schemas stop here (ARCH-17). */
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
