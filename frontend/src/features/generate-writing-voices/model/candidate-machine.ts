import { completeCandidateBatch, type WritingVoiceCandidateBatch } from '@/entities/voice-candidate'
import type { AppFailure } from '@/shared/api'

export type CandidatePhase =
  'idle' | 'confirming' | 'starting' | 'running' | 'ready' | 'failed' | 'cancelling' | 'adopting'
export interface CandidateState extends WritingVoiceCandidateBatch {
  ownerId: string
  phase: CandidatePhase
  hydrated: boolean
  selectedId: string
  operation: number
  frozenModel?: { providerId: string; modelId: string }
  failure?: AppFailure
  cancelDialog: boolean
  settledJobId: string
  settledStatus: string
}
export const initialCandidateState = (ownerId: string): CandidateState => ({
  ownerId,
  phase: 'idle',
  hydrated: false,
  jobId: '',
  resultJobId: '',
  candidates: [],
  selectedId: '',
  operation: 0,
  cancelDialog: false,
  settledJobId: '',
  settledStatus: '',
})
export type CandidateEvent = { ownerId: string } & (
  | { type: 'latest'; batch: WritingVoiceCandidateBatch }
  | { type: 'confirm'; model: { providerId: string; modelId: string } }
  | { type: 'dismiss-confirm' }
  | { type: 'start' }
  | { type: 'started'; operation: number; jobId: string }
  | { type: 'failure'; operation: number; failure: AppFailure }
  | { type: 'terminal'; jobId: string; status: string; failure?: AppFailure }
  | { type: 'result-failed'; jobId: string; failure?: AppFailure }
  | { type: 'select'; candidateId: string }
  | { type: 'adopt' }
  | { type: 'adopted'; operation: number }
  | { type: 'open-cancel' | 'dismiss-cancel' | 'cancel' }
  | { type: 'cancelled'; operation: number }
)
export function candidateBusy(state: CandidateState) {
  return ['starting', 'running', 'cancelling', 'adopting'].includes(state.phase)
}
export function candidateTransition(state: CandidateState, event: CandidateEvent): CandidateState {
  if (!state.ownerId || event.ownerId !== state.ownerId) return state
  if (event.type === 'latest') {
    if (['starting', 'adopting'].includes(state.phase)) return state
    if (candidateBusy(state) && state.jobId && event.batch.jobId !== state.jobId) return state
    const valid = !!event.batch.resultJobId && completeCandidateBatch(event.batch.candidates)
    const batchChanged = valid && event.batch.resultJobId !== state.resultJobId
    const next = {
      ...state,
      hydrated: true,
      ...(valid
        ? {
            resultJobId: event.batch.resultJobId,
            candidates: event.batch.candidates,
            selectedId: batchChanged ? '' : state.selectedId,
          }
        : {}),
    }
    if (state.phase === 'confirming') return next
    if (state.phase === 'cancelling')
      return valid && event.batch.resultJobId === state.jobId && state.settledStatus === 'done'
        ? { ...next, phase: 'ready', cancelDialog: false }
        : next
    if (event.batch.jobId === state.settledJobId && state.settledStatus === 'failed')
      return { ...next, phase: 'failed' }
    if (event.batch.jobId === state.settledJobId && state.settledStatus === 'cancelled')
      return { ...next, phase: next.candidates.length ? 'ready' : 'idle' }
    if (event.batch.jobId && event.batch.jobId !== event.batch.resultJobId)
      return {
        ...next,
        jobId: event.batch.jobId,
        phase: state.phase === 'failed' && state.jobId === event.batch.jobId ? 'failed' : 'running',
      }
    return { ...next, jobId: event.batch.jobId, phase: valid ? 'ready' : 'idle' }
  }
  if (event.type === 'terminal') {
    if (event.jobId !== state.jobId || !['running', 'cancelling'].includes(state.phase))
      return state
    const settled = { settledJobId: event.jobId, settledStatus: event.status }
    if (event.status === 'failed')
      return { ...state, ...settled, phase: 'failed', cancelDialog: false, failure: event.failure }
    if (event.status === 'cancelled')
      return {
        ...state,
        ...settled,
        phase: state.candidates.length ? 'ready' : 'idle',
        cancelDialog: false,
        failure: undefined,
      }
    if (
      event.status === 'done' &&
      state.resultJobId === event.jobId &&
      completeCandidateBatch(state.candidates)
    )
      return { ...state, ...settled, phase: 'ready', cancelDialog: false, failure: undefined }
    return event.status === 'done' ? { ...state, ...settled } : state
  }
  if (event.type === 'result-failed') {
    return event.jobId === state.jobId && ['running', 'cancelling'].includes(state.phase)
      ? { ...state, phase: 'failed', failure: event.failure, cancelDialog: false }
      : state
  }
  if (event.type === 'failure') {
    if (
      event.operation !== state.operation ||
      !['starting', 'adopting', 'cancelling'].includes(state.phase)
    )
      return state
    return {
      ...state,
      phase: state.phase === 'cancelling' ? 'running' : 'failed',
      cancelDialog: false,
      failure: event.failure,
    }
  }
  switch (event.type) {
    case 'confirm':
      return !candidateBusy(state) &&
        state.phase !== 'confirming' &&
        event.model.modelId &&
        event.model.providerId
        ? { ...state, phase: 'confirming', frozenModel: { ...event.model }, failure: undefined }
        : state
    case 'dismiss-confirm':
      return state.phase === 'confirming'
        ? { ...state, phase: state.candidates.length ? 'ready' : 'idle', frozenModel: undefined }
        : state
    case 'start':
      return state.phase === 'confirming' && state.frozenModel
        ? { ...state, phase: 'starting', operation: state.operation + 1 }
        : state
    case 'started':
      return state.phase === 'starting' && event.operation === state.operation && event.jobId
        ? { ...state, phase: 'running', jobId: event.jobId, cancelDialog: false }
        : state
    case 'select':
      return ['ready', 'failed'].includes(state.phase) &&
        state.candidates.some((candidate) => candidate.id === event.candidateId)
        ? { ...state, selectedId: event.candidateId }
        : state
    case 'adopt':
      return ['ready', 'failed'].includes(state.phase) &&
        !!state.resultJobId &&
        completeCandidateBatch(state.candidates) &&
        state.candidates.some((candidate) => candidate.id === state.selectedId)
        ? { ...state, phase: 'adopting', operation: state.operation + 1, failure: undefined }
        : state
    case 'adopted':
      return state.phase === 'adopting' && event.operation === state.operation
        ? { ...state, phase: 'ready' }
        : state
    case 'open-cancel':
      return state.phase === 'running' ? { ...state, cancelDialog: true } : state
    case 'dismiss-cancel':
      return state.phase === 'running' ? { ...state, cancelDialog: false } : state
    case 'cancel':
      return state.phase === 'running' && state.cancelDialog
        ? { ...state, phase: 'cancelling', operation: state.operation + 1 }
        : state
    case 'cancelled':
      return state.phase === 'cancelling' && event.operation === state.operation
        ? {
            ...state,
            cancelDialog: false,
            phase:
              state.resultJobId === state.jobId && completeCandidateBatch(state.candidates)
                ? 'ready'
                : 'cancelling',
          }
        : state
  }
}
