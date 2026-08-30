import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useIsMutating, useQueryClient } from '@tanstack/react-query'
import {
  ComparisonPairSchema,
  type GetComparisonPairsResponse,
  GetComparisonPairsResponseSchema,
  ModelRefSchema,
  ProviderService,
} from '@/shared/api'
import type { ModelRef, StageName } from '../model/types'
import {
  getComparisonPairsQueryKey,
  getSelectionsQueryKey,
  stageToProto,
  toComparisonPair,
  toRecommendationSet,
} from './catalog-mappers'

export function useModelSetup() {
  const pairs = useQuery(ProviderService.method.getComparisonPairs, {})
  const recommendations = useQuery(ProviderService.method.listRecommendationSets, {})
  return {
    pairs: pairs.data?.pairs.flatMap((pair) => toComparisonPair(pair) ?? []) ?? [],
    recommendations: recommendations.data?.sets.map(toRecommendationSet) ?? [],
    isPending: pairs.isPending || recommendations.isPending,
    isError: pairs.isError || recommendations.isError,
  }
}

export function useApplyRecommendation() {
  const mutation = useMutation(ProviderService.method.applyRecommendationSet)
  const transport = useTransport()
  const queryClient = useQueryClient()
  return {
    ...mutation,
    apply: async (id: string) => {
      const response = await mutation.mutateAsync({ id })
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getSelectionsQueryKey(transport) }),
        queryClient.invalidateQueries({ queryKey: getComparisonPairsQueryKey(transport) }),
      ])
      return response
    },
  }
}

export function useSaveComparisonPair() {
  const transportForCache = useTransport()
  const cache = useQueryClient()
  const mutation = useMutation(ProviderService.method.saveComparisonPair, {
    mutationKey: SAVE_COMPARISON_PAIR_MUTATION_KEY,
    // Written synchronously, exactly as `useSaveSelection` does: `useComparisonPairSavePending`
    // goes false the moment the mutation resolves, so a pair that only arrives with the
    // invalidate's refetch would leave a window where the start reads the PREVIOUS candidates —
    // the very stale read this pending flag was added to prevent.
    onSuccess: (data) => {
      const saved = data.pair
      if (!saved) return
      cache.setQueryData<GetComparisonPairsResponse>(
        getComparisonPairsQueryKey(transportForCache),
        (old) => {
          const next = old
            ? clone(GetComparisonPairsResponseSchema, old)
            : create(GetComparisonPairsResponseSchema, {})
          next.pairs = [
            ...next.pairs.filter((existing) => existing.stage !== saved.stage),
            create(ComparisonPairSchema, saved),
          ]
          return next
        },
      )
    },
  })
  const transport = useTransport()
  const queryClient = useQueryClient()
  return {
    ...mutation,
    save: async (stage: StageName, candidateA: ModelRef, candidateB: ModelRef) => {
      const response = await mutation.mutateAsync({
        stage: stageToProto(stage),
        candidateA: create(ModelRefSchema, candidateA),
        candidateB: create(ModelRefSchema, candidateB),
      })
      await queryClient.invalidateQueries({ queryKey: getComparisonPairsQueryKey(transport) })
      return response
    },
  }
}

/** A model-lab start must not read the previous saved pair while a replacement is in flight. */
export function useComparisonPairSavePending(): boolean {
  return useIsMutating({ mutationKey: SAVE_COMPARISON_PAIR_MUTATION_KEY }) > 0
}

const SAVE_COMPARISON_PAIR_MUTATION_KEY = ['model-comparison-pair', 'save'] as const
