import type { ModelRef, StageName } from '@/entities/model-catalog/@x/model-experiment'
import type { VerdictBadgeName } from './badges'
import type { AppFailure, ContentLanguage, Observation, PostContent } from '@/shared/api'

export type ExperimentStatusName =
  'queued' | 'running' | 'review' | 'partial' | 'decided' | 'dismissed' | 'failed'
export type CandidateStatusName = 'pending' | 'running' | 'succeeded' | 'failed'
export type DisplaySideName = 'left' | 'right'
export type CostSourceName = 'reported' | 'estimated' | 'unavailable' | 'mixed'
/** Where a comparison was started, frozen by the server at start. It decides which verdict
 *  the review offers, and it is never the address the review was opened from. */
export type ExperimentOriginName = 'editor' | 'lab'
/** How far back a leaderboard reads, measured from the moment of the request. There is no
 *  all-time value: model quality moves with every release. */
export type LeaderboardWindowName = 'day' | 'week' | 'month'
/** Whose verdicts a leaderboard replays: the account's own, or everyone's as model-level
 *  figures that name no account. */
export type LeaderboardScopeName = 'me' | 'all'

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
  | { kind: 'analyze'; styleguide: string }

export interface ExperimentCandidate {
  id: string
  displaySide: DisplaySideName
  status: CandidateStatusName
  /** What the verdict said about this candidate. Revealed with its identity, never before. */
  badges: VerdictBadgeName[]
  otherNote: string
  output?: CandidateOutput
  failure: AppFailure | undefined
  model?: ModelRef
  modelLabel: string
  usage?: CandidateUsage
}

export interface ModelExperiment {
  id: string
  stage: StageName
  /** `editor`: this comparison wrote a post that has no content until one side is applied,
   *  so its verdict applies the winner. `lab`: the verdict is a ranking pick that applies
   *  nothing, and every application is a separate follow-up. */
  origin: ExperimentOriginName
  status: ExperimentStatusName
  postSlug: string
  /** The frozen voice for analyze/write work; observe compares the image snapshot only. */
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
  revealed: boolean
  targetLanguage: ContentLanguage | undefined
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
