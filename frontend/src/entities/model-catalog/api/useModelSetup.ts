import { create } from '@bufbuild/protobuf'
import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ModelRefSchema, ProviderService } from '@/shared/api'
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
  const mutation = useMutation(ProviderService.method.saveComparisonPair)
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
