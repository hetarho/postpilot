import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { stageToProto, type StageName } from '@/entities/model-catalog/@x/model-experiment'
import { ModelExperimentService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { isExperimentActive } from '../model/types'
import { toExperiment, toLeaderboardEntry } from './experiment-mappers'

export function useExperiment(id: string) {
  const query = useQuery(
    ModelExperimentService.method.getExperiment,
    { id },
    {
      enabled: Boolean(id),
      refetchInterval: (state) => {
        const value = state.state.data?.experiment
        return value && isExperimentActive(toExperiment(value).status) ? POLL_INTERVAL_MS : false
      },
    },
  )
  return {
    ...query,
    experiment: query.data?.experiment ? toExperiment(query.data.experiment) : undefined,
  }
}

export function useExperiments(stage?: StageName) {
  const query = useQuery(
    ModelExperimentService.method.listExperiments,
    { stage: stage ? stageToProto(stage) : undefined },
    {
      refetchInterval: (state) =>
        state.state.data?.experiments.some((value) =>
          isExperimentActive(toExperiment(value).status),
        )
          ? POLL_INTERVAL_MS
          : false,
    },
  )
  const experiments = useMemo(() => query.data?.experiments.map(toExperiment) ?? [], [query.data])
  return { ...query, experiments }
}

export function useLeaderboard(stage: StageName) {
  const query = useQuery(ModelExperimentService.method.getLeaderboard, {
    stage: stageToProto(stage),
  })
  const entries = useMemo(() => query.data?.entries.map(toLeaderboardEntry) ?? [], [query.data])
  return { ...query, entries }
}
