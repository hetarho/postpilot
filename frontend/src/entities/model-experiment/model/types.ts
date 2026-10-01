import type { ModelRef, StageName } from '@/entities/model-catalog/@x/model-experiment'
import type { FingerprintComparisonItem } from '@/entities/voice/@x/model-experiment'
import type { VerdictBadgeName } from './badges'
import type { AppFailure, ContentLanguage, Observation, PostContent } from '@/shared/api'

export type ExperimentStatusName =
  'queued' | 'running' | 'review' | 'partial' | 'completed' | 'decided' | 'dismissed' | 'failed'
export type CandidateStatusName = 'pending' | 'running' | 'succeeded' | 'failed'
export type DisplaySideName = 'left' | 'right' | 'c' | 'd' | 'e'
/** `withheld` is a reader the server sends no supplier cost to — anyone but the operator
 *  (QUOTA-66). The screen says nothing about cost then, not "cost unavailable". */
export type CostSourceName = 'reported' | 'estimated' | 'unavailable' | 'mixed' | 'withheld'
/** Where a comparison was started, frozen by the server at start. It decides which verdict
 *  the review offers, and it is never the address the review was opened from. */
export type ExperimentOriginName = 'editor' | 'lab'
/** How far back a leaderboard reads, measured from the moment of the request. There is no
 *  all-time value: model quality moves with every release. */
export type LeaderboardWindowName = 'day' | 'week' | 'month'
/** Whose verdicts a leaderboard replays: the account's own, or everyone's as model-level
 *  figures that name no account. */
export type LeaderboardScopeName = 'me' | 'all'
/** The stages the model lab compares (MODEL-30). Analyze keeps its one active selection and is
 *  never compared, so no comparison, history or board is ever an analyze one. */
export type ExperimentStageName = Exclude<StageName, 'analyze'>

export interface CandidateUsage {
  promptTokens: bigint
  completionTokens: bigint
  costMicrousd: bigint
  costSource: CostSourceName
  latencyMs: bigint
}

export type CandidateOutput =
  | { kind: 'write'; content: PostContent }
  | { kind: 'observe'; observations: Observation[] }
  /** A 말투 반영 비교 piece, measured against the voice's current analysis (MODEL-67). */
  | { kind: 'voice'; text: string; comparison: FingerprintComparisonItem[] }

/** What a write comparison was drawn from: a post, or one voice's answered prompt. */
export type ExperimentSourceName = 'post' | 'voice'

/** The model lab's tabs on the compare and history pages: the two stages it compares and
 *  말투 반영, a write comparison drawn from a voice (MODEL-44, MODEL-67). */
export type ModelLabTabName = ExperimentStageName | 'voice'

export interface ExperimentCandidate {
  id: string
  displaySide: DisplaySideName
  status: CandidateStatusName
  /** What the verdict said about this candidate. Revealed with its identity, never before. */
  badges: VerdictBadgeName[]
  otherNote: string
  rank?: number
  output?: CandidateOutput
  failure: AppFailure | undefined
  model?: ModelRef
  modelLabel: string
  usage?: CandidateUsage
}

export interface ModelExperiment {
  id: string
  stage: ExperimentStageName
  /** `editor`: this comparison wrote a post that has no content until one side is applied,
   *  so its verdict applies the winner. `lab`: the verdict is a ranking pick that applies
   *  nothing, and every application is a separate follow-up. */
  origin: ExperimentOriginName
  reviewMode?: 'pairwise' | 'candidate_ranking'
  status: ExperimentStatusName
  postSlug: string
  /** The frozen voice for write work; observe compares the image snapshot only. */
  voiceId: string
  /** The 템플릿 the frozen write input carried, by name. Empty when the post had none; it keeps
   *  the name the snapshot froze even after that template is renamed or deleted. */
  templateName: string
  jobId: string
  candidates: ExperimentCandidate[]
  winnerCandidateId: string
  outcome: 'winner' | 'skipped' | 'unpaired' | ''
  applyFailure: AppFailure | undefined
  appliedAt: string
  adoptionRequested: boolean
  adoptionFailure: AppFailure | undefined
  adoptedAt: string
  createdAt: string
  finishedAt: string
  decidedAt: string
  completedAt?: string
  appliedCandidateId?: string
  adoptedCandidateId?: string
  revealed: boolean
  targetLanguage: ContentLanguage | undefined
  source: ExperimentSourceName
  /** A voice-sourced comparison's prompt, and the owner's answer while the snapshot keeps it. */
  voicePromptKey: string
  voicePromptText: string
  voiceAnswer: string
}

/** How often one model earned one badge inside this board's own scope, stage and window. */
export interface BadgeTally {
  badge: VerdictBadgeName
  count: number
}

export interface LeaderboardEntry {
  rank: number
  model: ModelRef
  modelLabel: string
  rating: number
  matches: number
  wins: number
  losses: number
  winRate: number
  successfulCalls: number
  averageLatencyMs: bigint
  promptTokens: bigint
  completionTokens: bigint
  totalCostMicrousd: bigint
  costQuality: CostSourceName
  provisional: boolean
  active: boolean
  recommended: boolean
  disappeared: boolean
  /** Most often first, then in catalog order. It explains a rank, never produces one. */
  badgeTallies: BadgeTally[]
}

export function isExperimentActive(status: ExperimentStatusName): boolean {
  return status === 'queued' || status === 'running'
}

export function needsExperimentReview(status: ExperimentStatusName): boolean {
  return status === 'review' || status === 'partial' || status === 'failed'
}
