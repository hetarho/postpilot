import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import {
  createConnectQueryKey,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
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

export function ownedExperimentQueryKey(transport: Transport, ownerId: string, id: string) {
  return [
    ...createConnectQueryKey({
      schema: ModelExperimentService.method.getExperiment,
      input: { id },
      transport,
      cardinality: 'finite',
    }),
    ownerId,
  ] as const
}
export function ownedExperimentsQueryKey(
  transport: Transport,
  ownerId: string,
  stage?: ExperimentStageName,
  source?: ExperimentSourceName,
) {
  return [
    ...createConnectQueryKey({
      schema: ModelExperimentService.method.listExperiments,
      input: {
        stage: stage ? stageToProto(stage) : undefined,
        source: experimentSourceToProto(source),
      },
      transport,
      cardinality: 'finite',
    }),
    ownerId,
  ] as const
}
export function useExperiment(id: string, ownerId?: string) {
  const transport = useTransport()
  const client = useMemo(() => createClient(ModelExperimentService, transport), [transport])
  const legacy = useConnectQuery(
    ModelExperimentService.method.getExperiment,
    { id },
    {
      enabled: Boolean(id) && ownerId === undefined,
      refetchInterval: (state) => {
        const value = state.state.data?.experiment
        return value && isExperimentActive(toExperiment(value).status) ? POLL_INTERVAL_MS : false
      },
    },
  )
  const owned = useQuery({
    queryKey: ownedExperimentQueryKey(transport, ownerId ?? '', id),
    queryFn: ({ signal }) => client.getExperiment({ id }, { signal }),
    enabled: !!id && ownerId !== undefined && ownerId !== '',
    retry: false,
    refetchInterval: (state) =>
      state.state.data?.experiment &&
      isExperimentActive(toExperiment(state.state.data.experiment).status)
        ? POLL_INTERVAL_MS
        : false,
  })
  const query = ownerId === undefined ? legacy : owned
  return {
    ...query,
    experiment: query.data?.experiment ? toExperiment(query.data.experiment) : undefined,
  }
}

/** The account's comparisons newest first, narrowed to one stage and, for a write history, one
 *  source: 글쓰기 lists post-sourced comparisons, 말투 반영 voice-sourced ones (MODEL-67). */
export function useExperiments(
  stage?: ExperimentStageName,
  source?: ExperimentSourceName,
  ownerId?: string,
) {
  const transport = useTransport()
  const client = useMemo(() => createClient(ModelExperimentService, transport), [transport])
  const legacy = useConnectQuery(
    ModelExperimentService.method.listExperiments,
    { stage: stage ? stageToProto(stage) : undefined, source: experimentSourceToProto(source) },
    {
      enabled: ownerId === undefined,
      refetchInterval: (state) =>
        state.state.data?.experiments.some((value) =>
          isExperimentActive(toExperiment(value).status),
        )
          ? POLL_INTERVAL_MS
          : false,
    },
  )
  const owned = useQuery({
    queryKey: ownedExperimentsQueryKey(transport, ownerId ?? '', stage, source),
    queryFn: ({ signal }) =>
      client.listExperiments(
        { stage: stage ? stageToProto(stage) : undefined, source: experimentSourceToProto(source) },
        { signal },
      ),
    enabled: ownerId !== undefined && ownerId !== '',
    retry: false,
    refetchInterval: (state) =>
      state.state.data?.experiments.some((value) => isExperimentActive(toExperiment(value).status))
        ? POLL_INTERVAL_MS
        : false,
  })
  const query = ownerId === undefined ? legacy : owned
  const experiments = useMemo(() => query.data?.experiments.map(toExperiment) ?? [], [query.data])
  return { ...query, experiments }
}

export function useLeaderboard(
  stage: ExperimentStageName,
  window: LeaderboardWindowName,
  scope: LeaderboardScopeName,
) {
  const query = useConnectQuery(ModelExperimentService.method.getLeaderboard, {
    stage: stageToProto(stage),
    window: leaderboardWindowToProto(window),
    scope: leaderboardScopeToProto(scope),
  })
  const entries = useMemo(() => query.data?.entries.map(toLeaderboardEntry) ?? [], [query.data])
  return { ...query, entries }
}
