import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { ModelExperimentService, ModelRefSchema } from '@/shared/api'

export function useStartModelExperiment() {
  const observe = useMutation(ModelExperimentService.method.startObserveExperiment)
  const analyze = useMutation(ModelExperimentService.method.startAnalyzeExperiment)
  return {
    isPending: observe.isPending || analyze.isPending,
    error: observe.error ?? analyze.error,
    startObserve: (postSlug: string, modelA: ModelRef, modelB: ModelRef) =>
      observe.mutateAsync({
        postSlug,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
      }),
    startAnalyze: (modelA: ModelRef, modelB: ModelRef) =>
      analyze.mutateAsync({
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
      }),
  }
}
