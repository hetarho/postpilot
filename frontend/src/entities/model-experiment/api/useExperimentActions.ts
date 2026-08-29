import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ModelExperimentService } from '@/shared/api'
import { experimentQueryKey, experimentsQueryKey, leaderboardQueryKey } from './experiment-mappers'

export function useExperimentActions(id: string, onChanged?: () => Promise<unknown>) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const choose = useMutation(ModelExperimentService.method.chooseWinner)
  const useSingle = useMutation(ModelExperimentService.method.useSingleCandidate)
  const dismiss = useMutation(ModelExperimentService.method.dismissExperiment)
  const retry = useMutation(ModelExperimentService.method.retryCandidate)
  const apply = useMutation(ModelExperimentService.method.applyWinnerOutput)
  const adopt = useMutation(ModelExperimentService.method.adoptWinnerModel)
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: experimentQueryKey(transport, id) }),
      ...[undefined, 1, 2, 3].map((stage) =>
        queryClient.invalidateQueries({ queryKey: experimentsQueryKey(transport, stage) }),
      ),
      ...[1, 2, 3].map((stage) =>
        queryClient.invalidateQueries({ queryKey: leaderboardQueryKey(transport, stage) }),
      ),
    ])
    await onChanged?.()
  }
  return {
    choose: async (candidateId: string) => {
      const value = await choose.mutateAsync({ experimentId: id, candidateId })
      await refresh()
      return value
    },
    useSingle: async (candidateId: string) => {
      const value = await useSingle.mutateAsync({ experimentId: id, candidateId })
      await refresh()
      return value
    },
    dismiss: async () => {
      const value = await dismiss.mutateAsync({ experimentId: id })
      await refresh()
      return value
    },
    retry: async () => {
      const value = await retry.mutateAsync({ experimentId: id })
      await refresh()
      return value
    },
    apply: async (confirmStyleguideOverwrite = false) => {
      const value = await apply.mutateAsync({ experimentId: id, confirmStyleguideOverwrite })
      await refresh()
      return value
    },
    adopt: async () => {
      const value = await adopt.mutateAsync({ experimentId: id })
      await refresh()
      return value
    },
    isPending:
      choose.isPending ||
      useSingle.isPending ||
      dismiss.isPending ||
      retry.isPending ||
      apply.isPending ||
      adopt.isPending,
    error:
      choose.error ?? useSingle.error ?? dismiss.error ?? retry.error ?? apply.error ?? adopt.error,
  }
}
