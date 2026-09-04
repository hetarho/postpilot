import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { toModelRef, type StageName } from '@/entities/model-catalog/@x/model-experiment'
import {
  CandidateStatus,
  contentLanguageFromProto,
  CostSource,
  DisplaySide,
  ExperimentOutcome,
  ExperimentStatus,
  ModelExperimentService,
  appFailureFromProto,
  type ProtoExperimentCandidate,
  type ProtoLeaderboardEntry,
  type ProtoModelExperiment,
} from '@/shared/api'
import type {
  CandidateStatusName,
  CostSourceName,
  ExperimentCandidate,
  ExperimentStatusName,
  LeaderboardEntry,
  ModelExperiment,
} from '../model/types'

export function toExperiment(value: ProtoModelExperiment): ModelExperiment {
  return {
    id: value.id,
    stage: stageName(value.stage),
    status: statusName(value.status),
    postSlug: value.postSlug,
    voiceId: value.voiceId,
    templateName: value.templateName,
    jobId: value.jobId,
    candidates: value.candidates.map(toCandidate),
    winnerCandidateId: value.winnerCandidateId,
    outcome: outcomeName(value.outcome),
    applyFailure: value.applyFailure ? appFailureFromProto(value.applyFailure) : undefined,
    appliedAt: value.appliedAt,
    adoptionRequested: value.adoptionRequested,
    adoptionFailure: value.adoptionFailure ? appFailureFromProto(value.adoptionFailure) : undefined,
    adoptedAt: value.adoptedAt,
    createdAt: value.createdAt,
    finishedAt: value.finishedAt,
    decidedAt: value.decidedAt,
    revealed: value.revealed,
    targetLanguage: contentLanguageFromProto(value.targetLanguage),
  }
}

function toCandidate(value: ProtoExperimentCandidate): ExperimentCandidate {
  const output = value.output
  return {
    id: value.id,
    displaySide: value.displaySide === DisplaySide.LEFT ? 'left' : 'right',
    status: candidateStatusName(value.status),
    output:
      output.case === 'postContent'
        ? { kind: 'write', content: output.value }
        : output.case === 'observationSet'
          ? { kind: 'observe', observations: output.value.observations }
          : output.case === 'styleguide'
            ? { kind: 'analyze', styleguide: output.value }
            : undefined,
    failure:
      value.failure || value.status === CandidateStatus.FAILED
        ? appFailureFromProto(value.failure)
        : undefined,
    model: value.model ? toModelRef(value.model) : undefined,
    modelLabel: value.modelLabel,
    usage: value.usage
      ? {
          promptTokens: value.usage.promptTokens,
          completionTokens: value.usage.completionTokens,
          costMicrousd: value.usage.costMicrousd,
          costSource: costSourceName(value.usage.costSource),
          latencyMs: value.usage.latencyMs,
        }
      : undefined,
  }
}

export function toLeaderboardEntry(value: ProtoLeaderboardEntry): LeaderboardEntry {
  return {
    rank: value.rank,
    model: toModelRef(value.model),
    modelLabel: value.modelLabel,
    rating: value.rating,
    matches: value.matches,
    wins: value.wins,
    losses: value.losses,
    winRate: value.winRate,
    successfulCalls: value.successfulCalls,
    averageLatencyMs: value.averageLatencyMs,
    promptTokens: value.promptTokens,
    completionTokens: value.completionTokens,
    totalCostMicrousd: value.totalCostMicrousd,
    costQuality: costSourceName(value.costQuality),
    provisional: value.provisional,
    active: value.active,
    recommended: value.recommended,
    disappeared: value.disappeared,
  }
}

export function experimentQueryKey(transport: Transport, id: string) {
  return createConnectQueryKey({
    schema: ModelExperimentService.method.getExperiment,
    input: { id },
    transport,
    cardinality: 'finite',
  })
}

export function experimentsQueryKey(transport: Transport, stage?: number) {
  return createConnectQueryKey({
    schema: ModelExperimentService.method.listExperiments,
    input: { stage },
    transport,
    cardinality: 'finite',
  })
}

/** Matches every cached ListExperiments, whatever its stage filter — for a change that can
 *  reach any of them at once, such as a post deletion detaching its experiments. */
export function experimentListQueriesKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ModelExperimentService.method.listExperiments,
    transport,
    cardinality: 'finite',
  })
}

export function leaderboardQueryKey(transport: Transport, stage: number) {
  return createConnectQueryKey({
    schema: ModelExperimentService.method.getLeaderboard,
    input: { stage },
    transport,
    cardinality: 'finite',
  })
}

function statusName(value: ExperimentStatus): ExperimentStatusName {
  return (
    (
      {
        [ExperimentStatus.QUEUED]: 'queued',
        [ExperimentStatus.RUNNING]: 'running',
        [ExperimentStatus.REVIEW]: 'review',
        [ExperimentStatus.PARTIAL]: 'partial',
        [ExperimentStatus.DECIDED]: 'decided',
        [ExperimentStatus.DISMISSED]: 'dismissed',
        [ExperimentStatus.FAILED]: 'failed',
      } as Partial<Record<ExperimentStatus, ExperimentStatusName>>
    )[value] ?? 'failed'
  )
}
function candidateStatusName(value: CandidateStatus): CandidateStatusName {
  return (
    (
      {
        [CandidateStatus.PENDING]: 'pending',
        [CandidateStatus.RUNNING]: 'running',
        [CandidateStatus.SUCCEEDED]: 'succeeded',
        [CandidateStatus.FAILED]: 'failed',
      } as Partial<Record<CandidateStatus, CandidateStatusName>>
    )[value] ?? 'failed'
  )
}
function costSourceName(value: CostSource): CostSourceName {
  return (
    (
      {
        [CostSource.REPORTED]: 'reported',
        [CostSource.ESTIMATED]: 'estimated',
        [CostSource.UNAVAILABLE]: 'unavailable',
        [CostSource.MIXED]: 'mixed',
      } as Partial<Record<CostSource, CostSourceName>>
    )[value] ?? 'unavailable'
  )
}
function outcomeName(value: ExperimentOutcome): ModelExperiment['outcome'] {
  if (value === ExperimentOutcome.WINNER) return 'winner'
  if (value === ExperimentOutcome.SKIPPED) return 'skipped'
  if (value === ExperimentOutcome.UNPAIRED) return 'unpaired'
  return ''
}
function stageName(value: number): StageName {
  if (value === 1) return 'observe'
  if (value === 2) return 'write'
  return 'analyze'
}
