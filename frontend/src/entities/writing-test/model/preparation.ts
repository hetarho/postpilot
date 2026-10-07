import type { AppFailure } from '@/shared/api'
import type { TestCount, TestEntrant, TestModelRef, TestSettingKind } from './types'

export interface CandidatePreparationScope {
  kind: TestSettingKind
  count: TestCount
}
export interface PreparedCandidate {
  id: string
  name: string
  description: string
  body: string
  titleArea: string
  revision: number
  source: Extract<TestEntrant, { type: 'authoring' }>
}
export interface PreparedCandidates extends CandidatePreparationScope {
  sessionId: string
  revision: number
  status: 'idle' | 'running' | 'ready' | 'failed' | 'cancelled'
  activeJobId: string
  candidates: PreparedCandidate[]
  failure?: AppFailure
}
export interface CandidatePreparationEstimateInput extends CandidatePreparationScope {
  writeModel: TestModelRef
  sessionId?: string
}
export interface CandidatePreparationStartInput extends CandidatePreparationScope {
  sessionId: string
  expectedRevision: number
  requestKey: string
  prompt: string
  writeModel: TestModelRef
}
export interface CandidatePreparationReadInput extends CandidatePreparationScope {
  sessionId: string
}
export interface CandidatePreparationClient {
  estimate(
    input: CandidatePreparationEstimateInput,
    signal?: AbortSignal,
  ): Promise<{ free: boolean; credits: number }>
  create(
    input: CandidatePreparationScope & { requestKey: string },
    signal?: AbortSignal,
  ): Promise<PreparedCandidates>
  start(input: CandidatePreparationStartInput, signal?: AbortSignal): Promise<PreparedCandidates>
  get(input: CandidatePreparationReadInput, signal?: AbortSignal): Promise<PreparedCandidates>
  cancel(
    input: CandidatePreparationReadInput & { jobId: string },
    signal?: AbortSignal,
  ): Promise<PreparedCandidates>
}
