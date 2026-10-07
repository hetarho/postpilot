import { assign, setup, type SnapshotFrom } from 'xstate'
import type { TestPublicationAction } from '@/entities/writing-test'
import {
  writingTestOperationBusy,
  type WritingTestOperationContext,
  type WritingTestPhase,
} from './writing-test-machine'

export type WritingTestView =
  | 'factor'
  | 'candidates'
  | 'material'
  | 'estimate'
  | 'work'
  | 'compare'
  | 'champion'
  | 'publication'
  | 'recovery'
export interface TestPublicationChoices {
  action: TestPublicationAction
  name: string
  scope: string
  scopeIds: string[]
  makeDefault: boolean
}
export interface TestPresentationRecovery {
  step?: 'factor' | 'candidates' | 'material'
  reading?: Record<string, number>
  visibleCandidateId?: string
  choices?: TestPublicationChoices
}
interface PresentationInput {
  scopeKey: string
  operation: WritingTestOperationContext
  phase: WritingTestPhase
  recovery?: TestPresentationRecovery
  prepareCandidates?: () => void
}
export interface WritingTestPresentationContext {
  scopeKey: string
  operation: WritingTestOperationContext
  phase: WritingTestPhase
  step: 'factor' | 'candidates' | 'material'
  reading: Record<string, number>
  visibleCandidateId: string
  choices: TestPublicationChoices
  prepareCandidates?: () => void
}
export type WritingTestPresentationEvent = { scopeKey: string } & (
  | { type: 'OBSERVE'; operation: WritingTestOperationContext; phase: WritingTestPhase }
  | { type: 'NEXT' | 'BACK' | 'OPEN_PUBLICATION' | 'PREPARE_CANDIDATES' }
  | { type: 'READING'; candidateId: string; position: number }
  | { type: 'SHOW_CANDIDATE'; candidateId: string }
  | { type: 'PUBLICATION_CHOICES'; choices: TestPublicationChoices }
)
function accepted(
  context: WritingTestPresentationContext,
  event: WritingTestPresentationEvent,
): boolean {
  if (event.scopeKey !== context.scopeKey || context.operation.suspended) return false
  if (event.type !== 'OBSERVE') return true
  const next = event.operation
  return (
    next.scopeKey === context.scopeKey &&
    next.operation >= context.operation.operation &&
    (!context.operation.test ||
      !next.test ||
      (next.test.id === context.operation.test.id &&
        next.test.revision >= context.operation.test.revision))
  )
}
function recoveryPhase(phase: WritingTestPhase): boolean {
  return [
    'partial',
    'quoteExpired',
    'cancelled',
    'expired',
    'conflict',
    'uncertain',
    'uncertainPublication',
    'failed',
  ].includes(phase)
}
/** Owns only the user's purposeful step, reading positions and unsent publication choices. */
export const testFlowMachine = setup({
  types: {
    context: {} as WritingTestPresentationContext,
    input: {} as PresentationInput,
    events: {} as WritingTestPresentationEvent,
  },
  guards: {
    accepted: ({ context, event }) => accepted(context, event),
    navigable: ({ context, event }) =>
      accepted(context, event) && !writingTestOperationBusy(context.phase),
    canPrepare: ({ context, event }) =>
      accepted(context, event) &&
      context.phase === 'editing' &&
      !!context.prepareCandidates &&
      !context.operation.test,
    observed: ({ context, event }) => accepted(context, event) && event.type === 'OBSERVE',
    factor: ({ context }) => context.step === 'factor',
    candidates: ({ context }) => context.step === 'candidates',
    quoted: ({ context }) => context.phase === 'quoted',
    work: ({ context }) =>
      (writingTestOperationBusy(context.phase) && context.phase !== 'deciding') ||
      context.phase === 'running',
    compare: ({ context }) => context.phase === 'match' || context.phase === 'deciding',
    champion: ({ context }) => context.phase === 'champion' || context.phase === 'published',
    recovery: ({ context }) => recoveryPhase(context.phase),
    canPublish: ({ context, event }) =>
      accepted(context, event) && ['champion', 'published', 'conflict'].includes(context.phase),
    canShowCandidate: ({ context, event }) =>
      accepted(context, event) &&
      event.type === 'SHOW_CANDIDATE' &&
      !!context.operation.test?.candidates.some((c) => c.id === event.candidateId),
    canRecordReading: ({ context, event }) =>
      accepted(context, event) &&
      event.type === 'READING' &&
      Number.isFinite(event.position) &&
      event.position >= 0 &&
      !!context.operation.test?.candidates.some((c) => c.id === event.candidateId),
  },
  actions: {
    observe: assign(({ event }) =>
      event.type === 'OBSERVE' ? { operation: event.operation, phase: event.phase } : {},
    ),
    nextCandidates: assign({ step: 'candidates' }),
    nextMaterial: assign({ step: 'material' }),
    previousFactor: assign({ step: 'factor' }),
    previousCandidates: assign({ step: 'candidates' }),
    reading: assign(({ context, event }) =>
      event.type === 'READING'
        ? { reading: { ...context.reading, [event.candidateId]: event.position } }
        : {},
    ),
    visible: assign(({ event }) =>
      event.type === 'SHOW_CANDIDATE' ? { visibleCandidateId: event.candidateId } : {},
    ),
    choices: assign(({ event }) =>
      event.type === 'PUBLICATION_CHOICES' ? { choices: structuredClone(event.choices) } : {},
    ),
    prepare: ({ context }) => context.prepareCandidates?.(),
  },
}).createMachine({
  id: 'writingTestPresentation',
  initial: 'routing',
  context: ({ input }) => ({
    scopeKey: input.scopeKey,
    operation: input.operation,
    phase: input.phase,
    step: input.recovery?.step ?? 'factor',
    reading: input.recovery?.reading ?? {},
    visibleCandidateId: input.recovery?.visibleCandidateId ?? '',
    choices: input.recovery?.choices ?? {
      action: 'save-setting',
      name: '',
      scope: '',
      scopeIds: [],
      makeDefault: false,
    },
    prepareCandidates: input.prepareCandidates,
  }),
  on: {
    OBSERVE: { guard: 'observed', target: '.routing', actions: 'observe' },
    READING: { guard: 'canRecordReading', actions: 'reading' },
    SHOW_CANDIDATE: { guard: 'canShowCandidate', actions: 'visible' },
    PUBLICATION_CHOICES: { guard: 'accepted', actions: 'choices' },
  },
  states: {
    routing: {
      always: [
        { guard: 'quoted', target: 'estimate' },
        { guard: 'compare', target: 'compare' },
        { guard: 'work', target: 'work' },
        { guard: 'champion', target: 'champion' },
        { guard: 'recovery', target: 'recovery' },
        { guard: 'factor', target: 'factor' },
        { guard: 'candidates', target: 'candidates' },
        { target: 'material' },
      ],
    },
    factor: {
      on: { NEXT: { guard: 'navigable', target: 'candidates', actions: 'nextCandidates' } },
    },
    candidates: {
      on: {
        NEXT: { guard: 'navigable', target: 'material', actions: 'nextMaterial' },
        BACK: { guard: 'navigable', target: 'factor', actions: 'previousFactor' },
        PREPARE_CANDIDATES: { guard: 'canPrepare', actions: 'prepare' },
      },
    },
    material: {
      on: { BACK: { guard: 'navigable', target: 'candidates', actions: 'previousCandidates' } },
    },
    estimate: { on: { BACK: { guard: 'accepted', target: 'material', actions: 'nextMaterial' } } },
    work: {},
    compare: {},
    champion: { on: { OPEN_PUBLICATION: { guard: 'canPublish', target: 'publication' } } },
    publication: { on: { BACK: { guard: 'navigable', target: 'champion' } } },
    recovery: { on: { OPEN_PUBLICATION: { guard: 'canPublish', target: 'publication' } } },
  },
})
export type WritingTestPresentationSnapshot = SnapshotFrom<typeof testFlowMachine>
export function writingTestView(snapshot: WritingTestPresentationSnapshot): WritingTestView {
  return snapshot.value as WritingTestView
}
