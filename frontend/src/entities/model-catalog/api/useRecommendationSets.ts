import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ProviderService, appFailureFromConnect } from '@/shared/api'
import type { RecommendationSet } from '../model/types'
import {
  listRecommendationSetsQueryKey,
  stageToProto,
  toRecommendationSet,
} from './catalog-mappers'

/** Every recommendation set, in the operator's order (MODEL-71). The operator's tab and
 *  /ai-models read this one query, so one write refreshes both. */
export function useRecommendationSets(): {
  sets: RecommendationSet[]
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(ProviderService.method.listRecommendationSets, {})
  return { sets: data?.sets.map(toRecommendationSet) ?? [], isPending, isError }
}

function useInvalidateRecommendationSets() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  return () =>
    queryClient.invalidateQueries({ queryKey: listRecommendationSetsQueryKey(transport) })
}

/** Creates (`id: ''`) or replaces one set whole. The server validates the whole draft and names
 *  every offending field at once (MODEL-70), so the failure is kept for the form to spread over
 *  its fields. Analyze sends its active model alone. */
export function useSaveRecommendationSet() {
  const invalidate = useInvalidateRecommendationSets()
  const mutation = useMutation(ProviderService.method.saveRecommendationSet, {
    onSettled: () => void invalidate(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    save: async (draft: RecommendationSet) => {
      const response = await mutation.mutateAsync({
        id: draft.id,
        label: draft.label,
        selections: draft.selections.map((selection) => ({
          stage: stageToProto(selection.stage),
          active: selection.active,
          ...(selection.stage === 'analyze'
            ? {}
            : { candidateA: selection.candidateA, candidateB: selection.candidateB }),
        })),
      })
      return response.set ? toRecommendationSet(response.set) : undefined
    },
  }
}

export function useDeleteRecommendationSet() {
  const invalidate = useInvalidateRecommendationSets()
  const mutation = useMutation(ProviderService.method.deleteRecommendationSet, {
    onSettled: () => void invalidate(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    remove: (id: string) => mutation.mutateAsync({ id }),
  }
}

/** Moves one set a place up (`earlier`) or down in the order every account sees. */
export function useMoveRecommendationSet() {
  const invalidate = useInvalidateRecommendationSets()
  const mutation = useMutation(ProviderService.method.moveRecommendationSet, {
    onSettled: () => void invalidate(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    move: (id: string, earlier: boolean) => mutation.mutate({ id, earlier }),
  }
}
