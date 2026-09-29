import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { stageToProto } from '@/entities/model-catalog/@x/model-experiment'
import { ModelExperimentService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { isExperimentActive } from '../model/types'
import type {
  ExperimentSourceName,
  ExperimentStageName,
  LeaderboardScopeName,
  LeaderboardWindowName,
} from '../model/types'
import {
  experimentSourceToProto,
  leaderboardScopeToProto,
  leaderboardWindowToProto,
  toExperiment,
  toLeaderboardEntry,
} from './experiment-mappers'

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

/** The account's comparisons newest first, narrowed to one stage and, for a write history, one
 *  source: 글쓰기 lists post-sourced comparisons, 말투 반영 voice-sourced ones (MODEL-67). */
export function useExperiments(stage?: ExperimentStageName, source?: ExperimentSourceName) {
  const query = useQuery(
    ModelExperimentService.method.listExperiments,
    { stage: stage ? stageToProto(stage) : undefined, source: experimentSourceToProto(source) },
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

export function useLeaderboard(
  stage: ExperimentStageName,
  window: LeaderboardWindowName,
  scope: LeaderboardScopeName,
) {
  const query = useQuery(ModelExperimentService.method.getLeaderboard, {
    stage: stageToProto(stage),
    window: leaderboardWindowToProto(window),
    scope: leaderboardScopeToProto(scope),
  })
  const entries = useMemo(() => query.data?.entries.map(toLeaderboardEntry) ?? [], [query.data])
  return { ...query, entries }
}
