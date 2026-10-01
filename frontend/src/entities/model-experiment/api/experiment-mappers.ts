import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { toModelRef } from '@/entities/model-catalog/@x/model-experiment'
import { toComparisons } from '@/entities/voice/@x/model-experiment'
import {
  CandidateStatus,
  contentLanguageFromProto,
  CostSource,
  DisplaySide,
  ExperimentOrigin,
  ExperimentOutcome,
  ExperimentSource,
  ExperimentStatus,
  LeaderboardScope,
  LeaderboardWindow,
  ModelExperimentService,
  Stage,
  VerdictBadge,
  appFailureFromProto,
  type ProtoExperimentCandidate,
  type ProtoLeaderboardEntry,
  type ProtoModelExperiment,
} from '@/shared/api'
import type { CandidateBadges, VerdictBadgeName } from '../model/badges'
import type {
  CandidateStatusName,
  CostSourceName,
  ExperimentCandidate,
  ExperimentOriginName,
  ExperimentSourceName,
  ExperimentStageName,
  ExperimentStatusName,
  LeaderboardEntry,
  LeaderboardScopeName,
  LeaderboardWindowName,
  ModelExperiment,
} from '../model/types'

export function toExperiment(value: ProtoModelExperiment): ModelExperiment {
  return {
    id: value.id,
    stage: stageName(value.stage),
    origin: originName(value.origin),
    reviewMode: value.reviewMode === 'candidate_ranking' ? 'candidate_ranking' : 'pairwise',
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
    completedAt: value.completedAt,
    revealed: value.revealed,
    targetLanguage: contentLanguageFromProto(value.targetLanguage),
    source: value.source === ExperimentSource.VOICE ? 'voice' : 'post',
    voicePromptKey: value.voicePromptKey,
    voicePromptText: value.voicePromptText,
    voiceAnswer: value.voiceAnswer,
  }
}

/** A history filter on the wire; none is every source. */
export function experimentSourceToProto(
  source: ExperimentSourceName | undefined,
): ExperimentSource {
  if (source === 'voice') return ExperimentSource.VOICE
  if (source === 'post') return ExperimentSource.POST
  return ExperimentSource.UNSPECIFIED
}

function toCandidate(value: ProtoExperimentCandidate): ExperimentCandidate {
  const output = value.output
  return {
    id: value.id,
    displaySide: displaySideName(value.displaySide),
    status: candidateStatusName(value.status),
    badges: value.badges.map(badgeFromProto).filter((badge) => badge !== undefined),
    otherNote: value.otherNote,
    rank: value.rank,
    output:
      output.case === 'postContent'
        ? { kind: 'write', content: output.value }
        : output.case === 'observationSet'
          ? { kind: 'observe', observations: output.value.observations }
          : output.case === 'voicePiece'
            ? {
                kind: 'voice',
                text: output.value.text,
                comparison: toComparisons(output.value.comparison),
              }
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

function displaySideName(value: DisplaySide): ExperimentCandidate['displaySide'] {
  switch (value) {
    case DisplaySide.LEFT:
      return 'left'
    case DisplaySide.RIGHT:
      return 'right'
    case DisplaySide.C:
      return 'c'
    case DisplaySide.D:
      return 'd'
    case DisplaySide.E:
      return 'e'
    case DisplaySide.UNSPECIFIED:
      return 'left'
    default: {
      const impossible: never = value
      throw new Error(`Unknown display side: ${impossible}`)
    }
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
    // A badge this build does not know is dropped rather than rendered as a blank chip.
    badgeTallies: value.badgeTallies.flatMap((tally) => {
      const badge = badgeFromProto(tally.badge)
      return badge ? [{ badge, count: tally.count }] : []
    }),
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

/** Matches every cached ListExperiments, whatever its stage filter — for a change that can
 *  reach any of them at once, such as a post deletion detaching its experiments. */
export function experimentListQueriesKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ModelExperimentService.method.listExperiments,
    transport,
    cardinality: 'finite',
  })
}

/** Matches every cached leaderboard, whatever its stage, window or scope. A verdict changes
 *  all of them at once — it enters its own window and every wider one — so the invalidation
 *  is stated here rather than enumerated by the caller. */
export function leaderboardQueriesKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ModelExperimentService.method.getLeaderboard,
    transport,
    cardinality: 'finite',
  })
}

export function leaderboardWindowToProto(window: LeaderboardWindowName): LeaderboardWindow {
  if (window === 'day') return LeaderboardWindow.DAY
  if (window === 'month') return LeaderboardWindow.MONTH
  return LeaderboardWindow.WEEK
}

export function leaderboardScopeToProto(scope: LeaderboardScopeName): LeaderboardScope {
  return scope === 'all' ? LeaderboardScope.ALL : LeaderboardScope.ME
}

/** One table, read both ways. A badge the other end knows and this one does not would drop
 *  silently on a verdict, so the pairing is stated once and pinned by a test. */
const BADGE_TO_PROTO: Record<VerdictBadgeName, VerdictBadge> = {
  fast: VerdictBadge.FAST,
  natural: VerdictBadge.NATURAL,
  on_brief: VerdictBadge.ON_BRIEF,
  structured: VerdictBadge.STRUCTURED,
  accurate: VerdictBadge.ACCURATE,
  in_voice: VerdictBadge.IN_VOICE,
  concise: VerdictBadge.CONCISE,
  slow: VerdictBadge.SLOW,
  ai_like: VerdictBadge.AI_LIKE,
  off_brief: VerdictBadge.OFF_BRIEF,
  verbose: VerdictBadge.VERBOSE,
  inaccurate: VerdictBadge.INACCURATE,
  off_voice: VerdictBadge.OFF_VOICE,
  repetitive: VerdictBadge.REPETITIVE,
  broken_format: VerdictBadge.BROKEN_FORMAT,
  other: VerdictBadge.OTHER,
}

export function badgeToProto(badge: VerdictBadgeName): VerdictBadge {
  return BADGE_TO_PROTO[badge]
}

export function badgeFromProto(value: VerdictBadge): VerdictBadgeName | undefined {
  for (const [name, wire] of Object.entries(BADGE_TO_PROTO) as [VerdictBadgeName, VerdictBadge][]) {
    if (wire === value) return name
  }
  return undefined
}

/** The payload a verdict carries. Built here rather than in the sheet, so no screen names a
 *  proto symbol (ARCH-17). */
export function badgesToProto(badges: CandidateBadges[]) {
  return badges.map((candidate) => ({
    candidateId: candidate.candidateId,
    badges: candidate.badges.map(badgeToProto),
    otherNote: candidate.otherNote,
  }))
}

function statusName(value: ExperimentStatus): ExperimentStatusName {
  return (
    (
      {
        [ExperimentStatus.QUEUED]: 'queued',
        [ExperimentStatus.RUNNING]: 'running',
        [ExperimentStatus.REVIEW]: 'review',
        [ExperimentStatus.PARTIAL]: 'partial',
        [ExperimentStatus.COMPLETED]: 'completed',
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
        [CostSource.UNSPECIFIED]: 'withheld',
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
/** UNSPECIFIED reads as the editor, matching the server: it is the behaviour every client
 *  predating the field was written against, so an unstated origin never silently turns a
 *  committing verdict into a pick. */
function originName(value: ExperimentOrigin): ExperimentOriginName {
  return value === ExperimentOrigin.LAB ? 'lab' : 'editor'
}
/** The lab compares observe and write alone (MODEL-30). A value this build does not know
 *  reads as observe, the lab's default stage, rather than as a stage it does not compare. */
function stageName(value: Stage): ExperimentStageName {
  return value === Stage.WRITE ? 'write' : 'observe'
}
