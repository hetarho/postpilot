import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, ModelExperimentService } from '@/shared/api'
import type { CandidateBadges, VerdictBadgeName } from '../model/badges'
import {
  badgesToProto,
  badgeToProto,
  experimentListQueriesKey,
  experimentQueryKey,
  leaderboardQueriesKey,
} from './experiment-mappers'

export function useExperimentActions(id: string, onChanged?: () => Promise<unknown>) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const choose = useMutation(ModelExperimentService.method.chooseWinner)
  const complete = useMutation(ModelExperimentService.method.completeExperimentReview)
  const decideWrite = useMutation(ModelExperimentService.method.decideWriteExperiment)
  const useSingle = useMutation(ModelExperimentService.method.useSingleCandidate)
  const dismiss = useMutation(ModelExperimentService.method.dismissExperiment)
  const retry = useMutation(ModelExperimentService.method.retryCandidate)
  const apply = useMutation(ModelExperimentService.method.applyWinnerOutput)
  const adopt = useMutation(ModelExperimentService.method.adoptWinnerModel)
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: experimentQueryKey(transport, id) }),
      // Every history at once, whatever its stage filter: the one this experiment is listed in
      // and the unfiltered one both show its new status.
      queryClient.invalidateQueries({ queryKey: experimentListQueriesKey(transport) }),
      // Every board at once: a verdict lands in its own window and in every wider one, on
      // the account's board and on the shared one.
      queryClient.invalidateQueries({ queryKey: leaderboardQueriesKey(transport) }),
    ])
    await onChanged?.()
  }
  return {
    complete: async (
      ranks: Array<{
        candidateId: string
        rank: number
        badges: VerdictBadgeName[]
        otherNote: string
      }>,
      skip = false,
    ) => {
      const value = await complete.mutateAsync({
        experimentId: id,
        skip,
        ranks: ranks.map((item) => ({ ...item, badges: item.badges.map(badgeToProto) })),
      })
      await refresh()
      return value
    },
    choose: async (candidateId: string, badges: CandidateBadges[] = []) => {
      const value = await choose.mutateAsync({
        experimentId: id,
        candidateId,
        badges: badgesToProto(badges),
      })
      await refresh()
      return value
    },
    decideWrite: async (
      candidateId: string,
      adoptWinnerModel: boolean,
      badges: CandidateBadges[] = [],
    ) => {
      const value = await decideWrite.mutateAsync({
        experimentId: id,
        candidateId,
        adoptWinnerModel,
        badges: badgesToProto(badges),
      })
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
    apply: async () => {
      const value = await apply.mutateAsync({ experimentId: id })
      await refresh()
      return value
    },
    adopt: async () => {
      const value = await adopt.mutateAsync({ experimentId: id })
      await refresh()
      return value
    },
    isPending:
      complete.isPending ||
      choose.isPending ||
      decideWrite.isPending ||
      useSingle.isPending ||
      dismiss.isPending ||
      retry.isPending ||
      apply.isPending ||
      adopt.isPending,
    failure: mutationFailure(
      complete.error ??
        choose.error ??
        decideWrite.error ??
        useSingle.error ??
        dismiss.error ??
        retry.error ??
        apply.error ??
        adopt.error,
    ),
  }
}

function mutationFailure(error: Error | null) {
  return error ? appFailureFromConnect(error) : undefined
}
