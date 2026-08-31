import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { appFailureFromConnect, ModelExperimentService, ModelRefSchema } from '@/shared/api'

export function useStartModelExperiment() {
  const observe = useMutation(ModelExperimentService.method.startObserveExperiment)
  const analyze = useMutation(ModelExperimentService.method.startAnalyzeExperiment)
  return {
    isPending: observe.isPending || analyze.isPending,
    failure:
      observe.error || analyze.error
        ? appFailureFromConnect(observe.error ?? analyze.error)
        : undefined,
    startObserve: (postSlug: string, modelA: ModelRef, modelB: ModelRef) =>
      observe.mutateAsync({
        postSlug,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
      }),
    // An analyze experiment compares one voice's corpus, so the voice is named explicitly — the
    // server never falls back to the default (spec/policy/model-experiments.md).
    startAnalyze: (voiceId: string, modelA: ModelRef, modelB: ModelRef) =>
      analyze.mutateAsync({
        voiceId,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
      }),
  }
}
