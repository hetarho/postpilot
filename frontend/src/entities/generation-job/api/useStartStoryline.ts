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

/** 스토리라인 먼저 and 다시 만들기 (GEN-68): a storyline job, started like a generation with the
 *  same models, observing only the attachments lacking an observation (GEN-89), and writing the
 *  post's storyline instead of its content. */
export function useStartStoryline() {
  const mutation = useMutation(GenerationService.method.startStoryline)

  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    /** `reobserveFiles` is ignored by the server, as `useStartGeneration`'s is. */
    start: (
      postSlug: string,
      observeModel: ModelRef | undefined,
      writeModel: ModelRef,
      reobserveFiles?: readonly string[],
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        writeModel: create(ModelRefSchema, writeModel),
        reobserve: reobserveFiles
          ? create(ReobserveSelectionSchema, { files: [...reobserveFiles] })
          : undefined,
      }),
  }
}
