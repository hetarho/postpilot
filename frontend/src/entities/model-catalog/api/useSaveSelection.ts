import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useIsMutating, useQueryClient } from '@tanstack/react-query'
import {
  appFailureFromConnect,
  type GetSelectionsResponse,
  GetSelectionsResponseSchema,
  ModelRefSchema,
  ProviderService,
  SelectionSchema,
} from '@/shared/api'
import type { ModelRef, StageName } from '../model/types'
import { getSelectionsQueryKey, stageToProto } from './catalog-mappers'

/** Records a stage's choice (PRD F-4: the last selection is remembered per account). */
export function useSaveSelection() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ProviderService.method.saveSelection, {
    mutationKey: SAVE_SELECTION_MUTATION_KEY,
    onSuccess: (data) => {
      const saved = data.selection
      if (!saved) return
      // The dropdown reflects the choice at once and every other reader of the
      // selections sees the same answer, without a round trip.
      queryClient.setQueryData<GetSelectionsResponse>(getSelectionsQueryKey(transport), (old) => {
        const next = old
          ? clone(GetSelectionsResponseSchema, old)
          : create(GetSelectionsResponseSchema, {})
        next.selections = [
          ...next.selections.filter((existing) => existing.stage !== saved.stage),
          create(SelectionSchema, saved),
        ]
        return next
      })
    },
  })

  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    save: (stage: StageName, ref: ModelRef) =>
      mutation.mutate({
        stage: stageToProto(stage),
        ref: create(ModelRefSchema, ref),
      }),
  }
}

/** True while either stage dropdown is persisting a new choice. Generation reads the
 * shared selection cache, so it must not start against the previous value in this gap. */
export function useSelectionSavePending(): boolean {
  return useIsMutating({ mutationKey: SAVE_SELECTION_MUTATION_KEY }) > 0
}

const SAVE_SELECTION_MUTATION_KEY = ['model-selection', 'save'] as const
