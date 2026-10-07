import {
  assign,
  fromPromise,
  getInitialSnapshot,
  getNextSnapshot,
  setup,
  type SnapshotFrom,
} from 'xstate'
import type { Voice } from '@/entities/voice'
import {
  completeCandidateBatch,
  WRITING_VOICE_CANDIDATE_COUNT,
  WRITING_VOICE_CANDIDATE_COUNTS,
  type WritingVoiceCandidateBatch,
  type WritingVoiceCandidateCount,
} from '@/entities/voice-candidate'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

export type CandidatePhase =
  'idle' | 'confirming' | 'starting' | 'running' | 'ready' | 'failed' | 'cancelling' | 'adopting'
export interface CandidateState extends WritingVoiceCandidateBatch {
  ownerId: string
  phase: CandidatePhase
  hydrated: boolean
  selectedId: string
  operation: number
  frozenModel?: { providerId: string; modelId: string }
  count: WritingVoiceCandidateCount
  frozenCount: WritingVoiceCandidateCount
  countJobId: string
  failure?: AppFailure
  cancelDialog: boolean
  settledJobId: string
  settledStatus: string
}
const initialCandidateData = (ownerId: string): CandidateContext => ({
  ownerId,
  hydrated: false,
  jobId: '',
  resultJobId: '',
  candidates: [],
  selectedId: '',
  operation: 0,
  settledJobId: '',
  settledStatus: '',
  count: WRITING_VOICE_CANDIDATE_COUNT,
  frozenCount: WRITING_VOICE_CANDIDATE_COUNT,
  countJobId: '',
})
export type CandidateEvent = { ownerId: string } & (
  | { type: 'latest'; batch: WritingVoiceCandidateBatch }
  | { type: 'confirm'; model: { providerId: string; modelId: string } }
  | { type: 'choose-count'; count: WritingVoiceCandidateCount }
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

type CandidateContext = Omit<CandidateState, 'phase' | 'cancelDialog'>
export interface CandidateWork {
  ownerId: string
  operation: number
  kind: 'start' | 'cancel' | 'adopt'
  model?: { providerId: string; modelId: string }
  jobId: string
  resultJobId: string
  selectedId: string
  count?: WritingVoiceCandidateCount
}
export interface CandidateResult {
  ownerId: string
  operation: number
  jobId?: string
  voice?: Voice
}
function resultOf(event: unknown) {
  return (event as { output?: CandidateResult }).output
}
function currentResult(context: CandidateContext, event: unknown) {
  const result = resultOf(event)
  return !!result && result.ownerId === context.ownerId && result.operation === context.operation
}
const workInput =
  (kind: CandidateWork['kind']) =>
  ({ context }: { context: CandidateContext }): CandidateWork => ({
    ownerId: context.ownerId,
    operation: context.operation,
    kind,
    model: context.frozenModel,
    jobId: context.jobId,
    resultJobId: context.resultJobId,
    selectedId: context.selectedId,
    count: context.frozenCount,
  })
function owns(context: CandidateContext, event: CandidateEvent) {
  return !!context.ownerId && context.ownerId === event.ownerId
}
function latestPhase(
  context: CandidateContext,
  event: CandidateEvent,
  phase: CandidatePhase,
): CandidatePhase | undefined {
  if (!owns(context, event) || event.type !== 'latest') return undefined
  if (
    ['running', 'cancelling'].includes(phase) &&
    context.jobId &&
    event.batch.jobId !== context.jobId
  )
    return undefined
  const valid = !!event.batch.resultJobId && validBatch(context, event.batch)
  if (event.batch.resultJobId && event.batch.resultJobId === context.countJobId && !valid)
    return 'failed'
  if (phase === 'confirming') return 'confirming'
  if (phase === 'cancelling')
    return valid && event.batch.resultJobId === context.jobId && context.settledStatus === 'done'
      ? 'ready'
      : 'cancelling'
  if (event.batch.jobId === context.settledJobId && context.settledStatus === 'failed')
    return 'failed'
  if (event.batch.jobId === context.settledJobId && context.settledStatus === 'cancelled')
    return (valid ? event.batch.candidates : context.candidates).length ? 'ready' : 'idle'
  if (event.batch.jobId && event.batch.jobId !== event.batch.resultJobId)
    return phase === 'failed' && context.jobId === event.batch.jobId ? 'failed' : 'running'
  return valid ? 'ready' : 'idle'
}
function validBatch(context: CandidateContext, batch: WritingVoiceCandidateBatch) {
  return completeCandidateBatch(
    batch.candidates,
    batch.resultJobId === context.countJobId ? context.frozenCount : undefined,
  )
}
const latestTransitions = (from: CandidatePhase) =>
  (['idle', 'confirming', 'running', 'ready', 'failed', 'cancelling'] as const).map((to) => ({
    guard: { type: 'latestPhase' as const, params: { from, to } },
    target: from === to ? undefined : to,
    actions: { type: 'latest' as const, params: { from } },
  }))
const terminalTransitions = [
  { guard: 'terminalFailed', target: 'failed', actions: ['settled', 'terminalFailure'] },
  { guard: 'terminalCancelledReady', target: 'ready', actions: ['settled', 'clearFailure'] },
  { guard: 'terminalCancelled', target: 'idle', actions: ['settled', 'clearFailure'] },
  { guard: 'terminalDoneReady', target: 'ready', actions: ['settled', 'clearFailure'] },
  { guard: 'terminalDone', actions: 'settled' },
] as const
const selectable = {
  'choose-count': { guard: 'validCount', actions: 'chooseCount' },
  confirm: { guard: 'model', target: 'confirming', actions: 'confirm' },
  select: { guard: 'candidate', actions: 'select' },
  adopt: { guard: 'adoptable', target: 'adopting', actions: ['begin', 'clearFailure'] },
} as const
function terminalMatches(context: CandidateContext, event: CandidateEvent) {
  return owns(context, event) && event.type === 'terminal' && event.jobId === context.jobId
}
export const candidateMachine = setup({
  types: {
    context: {} as CandidateContext,
    events: {} as CandidateEvent,
    input: {} as { ownerId: string },
  },
  actors: {
    perform: fromPromise<CandidateResult, CandidateWork>(async () => {
      throw new Error('Generated style actor unavailable')
    }),
  },
  guards: {
    validCount: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'choose-count' &&
      WRITING_VOICE_CANDIDATE_COUNTS.includes(event.count),
    startedResult: ({ context, event }) =>
      currentResult(context, event) && !!resultOf(event)?.jobId,
    adoptedResult: ({ context, event }) =>
      currentResult(context, event) &&
      !!resultOf(event)?.voice?.made &&
      !resultOf(event)?.voice?.deleted,
    cancelledResult: ({ context, event }) => currentResult(context, event),
    cancelledReadyResult: ({ context, event }) =>
      currentResult(context, event) &&
      context.resultJobId === context.jobId &&
      completeCandidateBatch(context.candidates),
    owned: ({ context, event }) => owns(context, event),
    latestPhase: ({ context, event }, params: { from: CandidatePhase; to: CandidatePhase }) =>
      latestPhase(context, event, params.from) === params.to,
    model: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'confirm' &&
      !!event.model.providerId &&
      !!event.model.modelId,
    hasCandidates: ({ context, event }) => owns(context, event) && context.candidates.length > 0,
    frozen: ({ context, event }) => owns(context, event) && !!context.frozenModel,
    started: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'started' &&
      event.operation === context.operation &&
      !!event.jobId,
    operation: ({ context, event }) =>
      owns(context, event) && 'operation' in event && event.operation === context.operation,
    candidate: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'select' &&
      context.candidates.some((candidate) => candidate.id === event.candidateId),
    adoptable: ({ context, event }) =>
      owns(context, event) &&
      !!context.resultJobId &&
      completeCandidateBatch(context.candidates) &&
      context.candidates.some((candidate) => candidate.id === context.selectedId),
    terminalFailed: ({ context, event }) =>
      terminalMatches(context, event) && event.type === 'terminal' && event.status === 'failed',
    terminalCancelledReady: ({ context, event }) =>
      terminalMatches(context, event) &&
      event.type === 'terminal' &&
      event.status === 'cancelled' &&
      context.candidates.length > 0,
    terminalCancelled: ({ context, event }) =>
      terminalMatches(context, event) && event.type === 'terminal' && event.status === 'cancelled',
    terminalDoneReady: ({ context, event }) =>
      terminalMatches(context, event) &&
      event.type === 'terminal' &&
      event.status === 'done' &&
      context.resultJobId === event.jobId &&
      completeCandidateBatch(context.candidates),
    terminalDone: ({ context, event }) =>
      terminalMatches(context, event) && event.type === 'terminal' && event.status === 'done',
    resultFailed: ({ context, event }) =>
      owns(context, event) && event.type === 'result-failed' && context.jobId === event.jobId,
    cancelledReady: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'cancelled' &&
      event.operation === context.operation &&
      context.resultJobId === context.jobId &&
      completeCandidateBatch(context.candidates),
  },
  actions: {
    chooseCount: assign(({ event }) =>
      event.type === 'choose-count' ? { count: event.count } : {},
    ),
    latest: assign(({ context, event }, params: { from: CandidatePhase }) => {
      if (event.type !== 'latest') return {}
      const valid = !!event.batch.resultJobId && validBatch(context, event.batch)
      const retainJob =
        params.from === 'confirming' ||
        params.from === 'cancelling' ||
        (event.batch.jobId === context.settledJobId &&
          ['failed', 'cancelled'].includes(context.settledStatus))
      return {
        hydrated: true,
        ...(event.batch.resultJobId && event.batch.resultJobId === context.countJobId && !valid
          ? { failure: { reason: 'WRITING_VOICE_CANDIDATE_OUTPUT_INVALID' as const, params: {} } }
          : {}),
        ...(valid
          ? {
              resultJobId: event.batch.resultJobId,
              candidates: event.batch.candidates,
              selectedId: event.batch.resultJobId !== context.resultJobId ? '' : context.selectedId,
            }
          : {}),
        ...(!retainJob ? { jobId: event.batch.jobId } : {}),
      }
    }),
    confirm: assign(({ context, event }) =>
      event.type === 'confirm'
        ? { frozenModel: { ...event.model }, frozenCount: context.count, failure: undefined }
        : {},
    ),
    dismiss: assign({ frozenModel: undefined }),
    begin: assign(({ context }) => ({ operation: context.operation + 1 })),
    started: assign(({ event }) =>
      event.type === 'started' ? { jobId: event.jobId, countJobId: event.jobId } : {},
    ),
    startedResult: assign(({ event }) => ({
      jobId: resultOf(event)!.jobId!,
      countJobId: resultOf(event)!.jobId!,
    })),
    select: assign(({ event }) =>
      event.type === 'select' ? { selectedId: event.candidateId } : {},
    ),
    failure: assign(({ event }) =>
      event.type === 'failure' || event.type === 'result-failed' ? { failure: event.failure } : {},
    ),
    terminalFailure: assign(({ event }) =>
      event.type === 'terminal' ? { failure: event.failure } : {},
    ),
    settled: assign(({ event }) =>
      event.type === 'terminal' ? { settledJobId: event.jobId, settledStatus: event.status } : {},
    ),
    clearFailure: assign({ failure: undefined }),
    invocationFailed: assign(({ event }) => ({
      failure: appFailureFromConnect((event as unknown as { error: unknown }).error),
    })),
    notifyAdopted: () => {},
    refreshCancelled: () => {},
  },
}).createMachine({
  id: 'writingCandidates',
  initial: 'idle',
  context: ({ input }) => initialCandidateData(input.ownerId),
  states: {
    idle: {
      on: {
        latest: latestTransitions('idle'),
        confirm: selectable.confirm,
        'choose-count': selectable['choose-count'],
      },
    },
    confirming: {
      on: {
        latest: latestTransitions('confirming'),
        'dismiss-confirm': [
          { guard: 'hasCandidates', target: 'ready', actions: 'dismiss' },
          { guard: 'owned', target: 'idle', actions: 'dismiss' },
        ],
        start: { guard: 'frozen', target: 'starting', actions: 'begin' },
      },
    },
    starting: {
      invoke: {
        src: 'perform',
        input: workInput('start'),
        onDone: { guard: 'startedResult', target: 'running', actions: 'startedResult' },
        onError: { target: 'failed', actions: 'invocationFailed' },
      },
      on: {
        started: { guard: 'started', target: 'running', actions: 'started' },
        failure: { guard: 'operation', target: 'failed', actions: 'failure' },
      },
    },
    ready: { on: { ...selectable, latest: latestTransitions('ready') } },
    failed: { on: { ...selectable, latest: latestTransitions('failed') } },
    adopting: {
      invoke: {
        src: 'perform',
        input: workInput('adopt'),
        onDone: { guard: 'adoptedResult', target: 'ready', actions: 'notifyAdopted' },
        onError: { target: 'failed', actions: 'invocationFailed' },
      },
      on: {
        adopted: { guard: 'operation', target: 'ready' },
        failure: { guard: 'operation', target: 'failed', actions: 'failure' },
      },
    },
    running: {
      initial: 'active',
      on: {
        latest: latestTransitions('running'),
        terminal: terminalTransitions,
        'result-failed': { guard: 'resultFailed', target: 'failed', actions: 'failure' },
      },
      states: {
        active: { on: { 'open-cancel': { guard: 'owned', target: 'confirming' } } },
        confirming: {
          on: {
            'dismiss-cancel': { guard: 'owned', target: 'active' },
            cancel: { guard: 'owned', target: '#writingCandidates.cancelling', actions: 'begin' },
          },
        },
      },
    },
    cancelling: {
      invoke: {
        src: 'perform',
        input: workInput('cancel'),
        onDone: [
          { guard: 'cancelledReadyResult', target: 'ready', actions: 'refreshCancelled' },
          { guard: 'cancelledResult', actions: 'refreshCancelled' },
        ],
        onError: { target: 'running', actions: 'invocationFailed' },
      },
      on: {
        latest: latestTransitions('cancelling'),
        terminal: terminalTransitions,
        'result-failed': { guard: 'resultFailed', target: 'failed', actions: 'failure' },
        failure: { guard: 'operation', target: 'running', actions: 'failure' },
        cancelled: [{ guard: 'cancelledReady', target: 'ready' }, { guard: 'operation' }],
      },
    },
  },
})
const projections = new WeakMap<object, CandidateState>()
export function candidateStateOf(snapshot: SnapshotFrom<typeof candidateMachine>): CandidateState {
  let state = projections.get(snapshot)
  if (!state) {
    state = {
      ...snapshot.context,
      phase: typeof snapshot.value === 'string' ? (snapshot.value as CandidatePhase) : 'running',
      cancelDialog: snapshot.matches({ running: 'confirming' }),
    }
    projections.set(snapshot, state)
  }
  return state
}
export function initialCandidateState(ownerId: string): CandidateState {
  return candidateStateOf(getInitialSnapshot(candidateMachine, { ownerId }))
}
export function candidateTransition(state: CandidateState, event: CandidateEvent): CandidateState {
  const { phase, cancelDialog, ...context } = state
  const value = phase === 'running' ? { running: cancelDialog ? 'confirming' : 'active' } : phase
  const snapshot = candidateMachine.resolveState({ value, context })
  const next = getNextSnapshot(candidateMachine, snapshot, event)
  return next.context === snapshot.context &&
    JSON.stringify(next.value) === JSON.stringify(snapshot.value)
    ? state
    : candidateStateOf(next)
}
