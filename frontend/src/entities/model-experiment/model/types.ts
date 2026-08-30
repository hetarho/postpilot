import type { ModelRef, StageName } from '@/entities/model-catalog/@x/model-experiment'
import type { Observation, PostContent } from '@/shared/api'

export type ExperimentStatusName =
  'queued' | 'running' | 'review' | 'partial' | 'decided' | 'dismissed' | 'failed'
export type CandidateStatusName = 'pending' | 'running' | 'succeeded' | 'failed'
export type DisplaySideName = 'left' | 'right'
export type CostSourceName = 'reported' | 'estimated' | 'unavailable' | 'mixed'

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
  output?: CandidateOutput
  error: string
  model?: ModelRef
  modelLabel: string
  usage?: CandidateUsage
}

export interface ModelExperiment {
  id: string
  stage: StageName
  status: ExperimentStatusName
  postSlug: string
  /** The frozen voice for analyze/write work; observe compares the image snapshot only. */
  voiceId: string
  /** The 용도 the frozen write input carried, by name. Empty when the post had none; it keeps
   *  the name the snapshot froze even after that purpose is renamed or deleted. */
  purposeName: string
  jobId: string
  candidates: ExperimentCandidate[]
  winnerCandidateId: string
  outcome: 'winner' | 'skipped' | 'unpaired' | ''
  applyError: string
  appliedAt: string
  adoptionRequested: boolean
  adoptionError: string
  adoptedAt: string
  createdAt: string
  finishedAt: string
  decidedAt: string
  revealed: boolean
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
}

export function isExperimentActive(status: ExperimentStatusName): boolean {
  return status === 'queued' || status === 'running'
}

export function needsExperimentReview(status: ExperimentStatusName): boolean {
  return status === 'review' || status === 'partial' || status === 'failed'
}
