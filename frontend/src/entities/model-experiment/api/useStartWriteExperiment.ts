import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog/@x/model-experiment'
import {
  appFailureFromConnect,
  ExperimentOrigin,
  ModelExperimentService,
  ModelRefSchema,
  ReobserveSelectionSchema,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { ExperimentOriginName } from '../model/types'

export function useStartWriteExperiment() {
  const mutation = useMutation(ModelExperimentService.method.startWriteExperiment)
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    /** `reobserveFiles` carries the same presence contract as `useStartGeneration`'s.
     *
     *  `origin` is required rather than defaulted, because the two surfaces that start a
     *  write comparison get different verdicts from it: the editor's applies its winner to
     *  the post, the lab's applies nothing. A caller that forgets it would silently get the
     *  editor's. */
    start: (
      postSlug: string,
      origin: ExperimentOriginName,
      observeModel: ModelRef | undefined,
      modelA: ModelRef,
      modelB: ModelRef,
      targetLength?: number,
      reobserveFiles?: readonly string[],
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
        origin: origin === 'lab' ? ExperimentOrigin.LAB : ExperimentOrigin.EDITOR,
        targetLength,
        reobserve: reobserveFiles
          ? create(ReobserveSelectionSchema, { files: [...reobserveFiles] })
          : undefined,
      }),
  }
}
