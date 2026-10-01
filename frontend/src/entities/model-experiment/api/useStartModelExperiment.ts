import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog/@x/model-experiment'
import { appFailureFromConnect, ModelExperimentService, ModelRefSchema } from '@/shared/api'

export function useStartModelExperiment() {
  const observe = useMutation(ModelExperimentService.method.startObserveExperiment)
  return {
    isPending: observe.isPending,
    failure: observe.error ? appFailureFromConnect(observe.error) : undefined,
    startObserve: (
      postSlug: string,
      modelA: ModelRef,
      modelB: ModelRef,
      extras: readonly ModelRef[] = [],
    ) =>
      observe.mutateAsync({
        postSlug,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
        candidates: [modelA, modelB, ...extras].map((ref) => create(ModelRefSchema, ref)),
      }),
  }
}
