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

/** 관찰하기 and 다시 관찰하기 (GEN-89): an observation job, which observes the post's attachments
 *  and writes their observations and nothing else. Its job reports the `observe` stage, so the
 *  existing stage strings label it. */
export function useStartObservation() {
  const mutation = useMutation(GenerationService.method.startObservation)

  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    /** `reobserveFiles` carries PRESENCE (GEN-9): an EMPTY array is 관찰하기, which observes only
     *  the attachments lacking an observation, the picker's answer is 다시 관찰하기, and
     *  `undefined` observes every attachment. */
    start: (
      postSlug: string,
      observeModel: ModelRef | undefined,
      reobserveFiles?: readonly string[],
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        reobserve: reobserveFiles
          ? create(ReobserveSelectionSchema, { files: [...reobserveFiles] })
          : undefined,
      }),
  }
}
