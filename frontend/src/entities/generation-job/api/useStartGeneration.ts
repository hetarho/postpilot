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
    /** `reobserveFiles` is still sent for an older server, but the server ignores it: a writing
     *  run observes only the attachments lacking an observation, and re-observing is
     *  `useStartObservation`'s (GEN-89). */
    start: (
      postSlug: string,
      observeModel: ModelRef | undefined,
      writeModel: ModelRef,
      targetLength?: number,
      reobserveFiles?: readonly string[],
      /** 이 스토리로 글 쓰기 / 다시 쓰기: write along the stored storyline, which observes exactly
       *  what it holds, so no re-observation answer goes with it (GEN-70). */
      fromStoryline = false,
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        writeModel: create(ModelRefSchema, writeModel),
        targetLength,
        reobserve:
          reobserveFiles && !fromStoryline
            ? create(ReobserveSelectionSchema, { files: [...reobserveFiles] })
            : undefined,
        fromStoryline,
      }),
  }
}
