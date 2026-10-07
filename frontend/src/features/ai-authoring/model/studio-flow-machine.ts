import { assign, setup, type SnapshotFrom } from 'xstate'
import {
  authoringScopeKey,
  completedAuthoringExchanges,
  AUTHORING_MESSAGE_MAX_CHARS,
  AUTHORING_MAX_EXCHANGES,
  type AuthoringScope,
} from '@/entities/ai-authoring'
import { studioBusy, type AuthoringState } from './authoring-machine'

export type StudioAIAvailability = 'preparing' | 'ready' | 'unavailable'
export type StudioView =
  | 'restoring'
  | 'purpose'
  | 'existing'
  | 'choices'
  | 'review'
  | 'direct'
  | 'refining'
  | 'publication'
  | 'working'
  | 'confirmed'
type Intent = 'recommend' | 'refine' | 'select' | 'edit' | 'save' | 'cancel'
type InputProblem = 'required' | 'length' | 'history'
interface FlowInput {
  scope: AuthoringScope
  operation: AuthoringState
  ai: StudioAIAvailability
  initialMakeDefault?: boolean
}
interface FlowContext {
  scopeKey: string
  targetId: string
  operation: AuthoringState
  ai: StudioAIAvailability
  purpose: string
  intent?: Intent
  resume?: StudioView
  problem?: InputProblem
  makeDefault: boolean
  initialMakeDefault: boolean
  resetOpen: boolean
}
type Scoped = { scopeKey: string }
export type StudioFlowEvent = Scoped &
  (
    | { type: 'OBSERVE'; operation: AuthoringState; ai: StudioAIAvailability }
    | { type: 'PURPOSE_CHANGED' | 'CHAT_CHANGED'; text: string }
    | { type: 'CHOOSE'; candidateId: string }
    | { type: 'DEFAULT_CHANGED'; checked: boolean }
    | {
        type:
          | 'RECOMMEND'
          | 'LOAD_EXISTING'
          | 'OPEN_DIRECT'
          | 'FINISH_DIRECT'
          | 'ASK_RESET'
          | 'DISMISS_RESET'
          | 'CONFIRM_RESET'
          | 'OPEN_REFINEMENT'
          | 'REFINE'
          | 'FINISH_REFINEMENT'
          | 'OPEN_PUBLICATION'
          | 'PUBLISH'
          | 'CHANGE_SELECTION'
          | 'BACK'
          | 'RETRY'
          | 'CONTINUE'
          | 'FRESH'
          | 'ASK_CANCEL'
          | 'DISMISS_CANCEL'
          | 'CONFIRM_CANCEL'
      }
  )

function accepted(context: FlowContext, event: StudioFlowEvent): boolean {
  if (event.scopeKey !== context.scopeKey) return false
  if (event.type !== 'OBSERVE') return true
  const before = context.operation
  const next = event.operation
  if (next.scopeKey !== context.scopeKey || next.operation < before.operation) return false
  if (before.session && next.session) {
    if (next.session.id !== before.session.id && next.operation <= before.operation) return false
    if (next.session.id === before.session.id && next.session.revision < before.session.revision)
      return false
  }
  return true
}
function available(context: FlowContext, event: StudioFlowEvent): boolean {
  return (
    accepted(context, event) &&
    (!studioBusy(context.operation) || recoveredPublication(context.operation)) &&
    !context.operation.command &&
    context.operation.phase !== 'checking' &&
    context.operation.phase !== 'saved' &&
    !(context.operation.failure && context.intent === 'save')
  )
}
function recoveredPublication(operation: AuthoringState): boolean {
  return (
    operation.phase === 'active' &&
    operation.session?.phase === 'saving' &&
    !operation.session.activeJobId
  )
}
function navigable(context: FlowContext, event: StudioFlowEvent): boolean {
  return accepted(context, event) && !studioBusy(context.operation) && !context.operation.command
}
function problem(context: FlowContext, refine: boolean): InputProblem | undefined {
  const text = refine ? context.operation.text.trim() : context.purpose.trim()
  if (refine && text === '') return 'required'
  if (Array.from(text).length > AUTHORING_MESSAGE_MAX_CHARS) return 'length'
  if (refine && completedAuthoringExchanges(context.operation.session) >= AUTHORING_MAX_EXCHANGES)
    return 'history'
  return undefined
}

/** Presentation owns navigation and publication choices. The operation actor remains the
 * authority for requests and durable facts; no navigation action calls a provider or resets it. */
export const studioFlowMachine = setup({
  types: {
    context: {} as FlowContext,
    input: {} as FlowInput,
    events: {} as StudioFlowEvent,
  },
  guards: {
    observedSaved: ({ context, event }) =>
      accepted(context, event) && event.type === 'OBSERVE' && event.operation.phase === 'saved',
    observedWorking: ({ context, event }) =>
      accepted(context, event) &&
      event.type === 'OBSERVE' &&
      studioBusy(event.operation) &&
      !recoveredPublication(event.operation) &&
      !studioBusy(context.operation),
    observedQuoteDismissed: ({ context, event }) =>
      accepted(context, event) &&
      event.type === 'OBSERVE' &&
      context.operation.phase === 'confirming' &&
      !studioBusy(event.operation) &&
      !event.operation.failure &&
      context.operation.operation === event.operation.operation,
    observedSettled: ({ context, event }) =>
      accepted(context, event) &&
      event.type === 'OBSERVE' &&
      ((studioBusy(context.operation) && !studioBusy(event.operation)) ||
        (context.operation.phase === 'checking' && event.operation.phase !== 'checking') ||
        context.operation.session?.id !== event.operation.session?.id ||
        context.operation.session?.selected?.id !== event.operation.session?.selected?.id),
    accepted: ({ context, event }) => accepted(context, event),
    available: ({ context, event }) => available(context, event),
    navigable: ({ context, event }) => navigable(context, event),
    saved: ({ context }) => context.operation.phase === 'saved',
    working: ({ context }) =>
      studioBusy(context.operation) &&
      !(context.operation.session?.phase === 'saving' && !context.operation.session.activeJobId),
    restoring: ({ context }) => context.operation.phase === 'checking',
    directResult: ({ context }) =>
      context.intent === 'edit' && context.resume === 'direct' && !!context.operation.session,
    refineResult: ({ context }) =>
      (context.intent === 'refine' ||
        (context.intent === 'edit' && context.resume === 'refining')) &&
      !!(context.operation.session?.workingSource ?? context.operation.session?.selected),
    publishResult: ({ context }) =>
      (context.intent === 'save' || context.operation.session?.phase === 'saving') &&
      !!context.operation.session?.selected,
    recommended: ({ context }) =>
      context.operation.session?.phase === 'choosing' &&
      !!context.operation.session.candidates.length,
    incompleteSource: ({ context }) =>
      !!context.operation.session?.workingSource && !context.operation.session.selected,
    canRefinement: ({ context, event }) =>
      navigable(context, event) &&
      !!(context.operation.session?.workingSource ?? context.operation.session?.selected),
    selected: ({ context }) => !!context.operation.session?.selected,
    hasCandidates: ({ context }) => !!context.operation.session?.candidates.length,
    existing: ({ context }) => context.targetId !== '',
    resumeChoices: ({ context }) => context.resume === 'choices',
    resumeRefinement: ({ context }) => context.resume === 'refining',
    canRecommend: ({ context, event }) =>
      available(context, event) && context.ai === 'ready' && !problem(context, false),
    canRefine: ({ context, event }) =>
      available(context, event) &&
      context.ai === 'ready' &&
      !!(context.operation.session?.workingSource ?? context.operation.session?.selected) &&
      !problem(context, true),
    canChoose: ({ context, event }) =>
      available(context, event) &&
      event.type === 'CHOOSE' &&
      !!context.operation.session?.candidates.some(
        (candidate) => candidate.id === event.candidateId,
      ),
    alreadySelected: ({ context, event }) =>
      available(context, event) &&
      event.type === 'CHOOSE' &&
      event.candidateId === context.operation.session?.selected?.id,
    canReview: ({ context, event }) =>
      navigable(context, event) && !!context.operation.session?.selected,
    canPublish: ({ context, event }) =>
      available(context, event) &&
      !context.operation.sourceDirty &&
      context.operation.session?.draftState !== 'invalid' &&
      context.operation.session?.draftState !== 'incomplete' &&
      !!context.operation.session?.selected?.body.trim(),
    hasSource: ({ context, event }) => available(context, event) && !!context.operation.session,
    sourceDirty: ({ context, event }) =>
      available(context, event) && !!context.operation.sourceDirty,
    canReset: ({ context, event }) =>
      available(context, event) && !!context.operation.session?.savedBaseline,
    canChangeSelection: ({ context, event }) =>
      navigable(context, event) && !!context.operation.session?.candidates.length,
    canRetry: ({ context, event }) =>
      accepted(context, event) &&
      (context.operation.phase === 'failed' || recoveredPublication(context.operation)),
    canContinue: ({ context, event }) =>
      accepted(context, event) &&
      !context.operation.command &&
      !!(context.operation.session?.workingSource ?? context.operation.session?.selected) &&
      (context.operation.phase === 'saved' ||
        ((context.operation.phase === 'failed' || recoveredPublication(context.operation)) &&
          (context.operation.failure?.reason === 'AUTHORING_SAVE_CONFLICT' ||
            context.operation.session?.failureReason === 'AUTHORING_SAVE_CONFLICT'))),
    canFresh: ({ context, event }) => accepted(context, event) && !studioBusy(context.operation),
    canCancel: ({ context, event }) =>
      accepted(context, event) &&
      context.operation.phase === 'active' &&
      !!context.operation.session?.activeJobId,
  },
  actions: {
    observe: assign(({ event }) =>
      event.type === 'OBSERVE' ? { operation: event.operation, ai: event.ai } : {},
    ),
    purposeChanged: assign(({ event }) =>
      event.type === 'PURPOSE_CHANGED' ? { purpose: event.text, problem: undefined } : {},
    ),
    defaultChanged: assign(({ event }) =>
      event.type === 'DEFAULT_CHANGED' ? { makeDefault: event.checked } : {},
    ),
    purposeProblem: assign({ problem: ({ context }) => problem(context, false) }),
    chatProblem: assign({ problem: ({ context }) => problem(context, true) }),
    clearProblem: assign({ problem: undefined }),
    recommending: assign({ intent: 'recommend', resume: 'purpose', problem: undefined }),
    rerolling: assign({ intent: 'recommend', resume: 'choices', problem: undefined }),
    refining: assign({ intent: 'refine', resume: 'refining', problem: undefined }),
    selecting: assign({ intent: 'select', problem: undefined }),
    editing: assign({ intent: 'edit', problem: undefined }),
    editingDirect: assign({ intent: 'edit', resume: 'direct', problem: undefined }),
    editingAI: assign({ intent: 'edit', resume: 'refining', problem: undefined }),
    publishing: assign({ intent: 'save', problem: undefined }),
    cancelling: assign({ intent: 'cancel' }),
    showReset: assign({ resetOpen: true }),
    hideReset: assign({ resetOpen: false }),
    keepingDirect: assign({ intent: 'edit', resume: 'review', problem: undefined }),
    reset: assign(({ context }) => ({
      purpose: '',
      intent: undefined,
      resume: undefined,
      problem: undefined,
      makeDefault: context.initialMakeDefault,
    })),
    // Implemented at the React boundary; only admitted machine events reach these ports.
    setChat: () => {},
    requestRecommend: () => {},
    requestRefine: () => {},
    loadExisting: () => {},
    selectCandidate: () => {},
    publish: () => {},
    retry: () => {},
    fresh: () => {},
    cancel: () => {},
    keepDirect: () => {},
    continueEditing: () => {},
    resetBaseline: () => {},
  },
}).createMachine({
  id: 'studioFlow',
  initial: 'deciding',
  context: ({ input }) => ({
    resetOpen: false,
    scopeKey: authoringScopeKey(input.scope),
    targetId: input.scope.targetId ?? '',
    operation: input.operation,
    ai: input.ai,
    purpose: input.operation.session ? '' : input.operation.text,
    makeDefault: input.scope.kind === 'writing-voice' && !!input.initialMakeDefault,
    initialMakeDefault: input.scope.kind === 'writing-voice' && !!input.initialMakeDefault,
  }),
  on: {
    OBSERVE: [
      { guard: 'observedSaved', target: '.confirmed', actions: 'observe' },
      { guard: 'observedWorking', target: '.working', actions: 'observe' },
      { guard: 'observedQuoteDismissed', target: '.returning', actions: 'observe' },
      { guard: 'observedSettled', target: '.deciding', actions: 'observe' },
      { guard: 'accepted', actions: 'observe' },
    ],
    RETRY: { guard: 'canRetry', target: '.working', actions: 'retry' },
    CONTINUE: {
      guard: 'canContinue',
      target: '.working',
      actions: ['keepingDirect', 'continueEditing'],
    },
    FRESH: { guard: 'canFresh', target: '.working', actions: ['refining', 'fresh'] },
    ASK_RESET: { guard: 'canReset', actions: 'showReset' },
    DISMISS_RESET: { guard: 'accepted', actions: 'hideReset' },
    CONFIRM_RESET: {
      guard: 'canReset',
      target: '.working',
      actions: ['hideReset', 'keepingDirect', 'resetBaseline'],
    },
  },
  states: {
    deciding: {
      always: [
        { guard: 'saved', target: 'confirmed' },
        { guard: 'restoring', target: 'restoring' },
        { guard: 'working', target: 'working' },
        { guard: 'directResult', target: 'direct' },
        { guard: 'refineResult', target: 'refining' },
        { guard: 'publishResult', target: 'publication' },
        { guard: 'recommended', target: 'choices' },
        { guard: 'incompleteSource', target: 'direct' },
        { guard: 'selected', target: 'review' },
        { guard: 'hasCandidates', target: 'choices' },
        { guard: 'existing', target: 'existing' },
        { target: 'purpose' },
      ],
    },
    resetting: {
      always: [{ guard: 'existing', target: 'existing' }, { target: 'purpose' }],
    },
    returning: {
      always: [
        { guard: 'resumeChoices', target: 'choices' },
        { guard: 'resumeRefinement', target: 'refining' },
        { guard: 'selected', target: 'review' },
        { guard: 'existing', target: 'existing' },
        { target: 'purpose' },
      ],
    },
    restoring: {},
    purpose: {
      on: {
        OPEN_DIRECT: {
          guard: 'available',
          target: 'working',
          actions: ['editingDirect', 'loadExisting'],
        },
        PURPOSE_CHANGED: { guard: 'available', actions: 'purposeChanged' },
        RECOMMEND: [
          {
            guard: 'canRecommend',
            target: 'working',
            actions: ['recommending', 'requestRecommend'],
          },
          { guard: 'available', actions: 'purposeProblem' },
        ],
      },
    },
    existing: {
      on: {
        OPEN_DIRECT: [
          { guard: 'hasSource', target: 'direct' },
          { guard: 'available', target: 'working', actions: ['editingDirect', 'loadExisting'] },
        ],
        OPEN_REFINEMENT: [
          { guard: 'canRefinement', target: 'refining' },
          { guard: 'available', target: 'working', actions: ['editingAI', 'loadExisting'] },
        ],
        LOAD_EXISTING: [
          { guard: 'canReview', target: 'review' },
          { guard: 'available', target: 'working', actions: ['editing', 'loadExisting'] },
        ],
      },
    },
    choices: {
      on: {
        CHOOSE: [
          { guard: 'alreadySelected', target: 'review' },
          { guard: 'canChoose', target: 'working', actions: ['selecting', 'selectCandidate'] },
        ],
        BACK: [
          { guard: 'canReview', target: 'review' },
          { guard: 'available', target: 'purpose' },
        ],
        RECOMMEND: {
          guard: 'canRecommend',
          target: 'working',
          actions: ['rerolling', 'requestRecommend'],
        },
      },
    },
    review: {
      on: {
        OPEN_DIRECT: { guard: 'hasSource', target: 'direct', actions: 'clearProblem' },
        OPEN_REFINEMENT: { guard: 'canReview', target: 'refining', actions: 'clearProblem' },
        OPEN_PUBLICATION: { guard: 'canPublish', target: 'publication' },
        CHANGE_SELECTION: { guard: 'canChangeSelection', target: 'choices' },
        BACK: [
          { guard: 'canChangeSelection', target: 'choices' },
          { guard: 'navigable', target: 'existing' },
        ],
      },
    },
    direct: {
      on: {
        FINISH_DIRECT: {
          guard: 'hasSource',
          target: 'working',
          actions: ['keepingDirect', 'keepDirect'],
        },
        OPEN_REFINEMENT: [
          { guard: 'sourceDirty', target: 'working', actions: ['refining', 'keepDirect'] },
          { guard: 'canRefinement', target: 'refining' },
        ],
        BACK: [
          { guard: 'sourceDirty', target: 'working', actions: ['keepingDirect', 'keepDirect'] },
          { guard: 'canReview', target: 'review' },
        ],
      },
    },
    refining: {
      on: {
        OPEN_DIRECT: { guard: 'hasSource', target: 'direct' },
        CHAT_CHANGED: { guard: 'available', actions: ['clearProblem', 'setChat'] },
        REFINE: [
          { guard: 'canRefine', target: 'working', actions: ['refining', 'requestRefine'] },
          { guard: 'available', actions: 'chatProblem' },
        ],
        FINISH_REFINEMENT: { guard: 'canReview', target: 'review' },
        BACK: { guard: 'canReview', target: 'review' },
      },
    },
    publication: {
      on: {
        DEFAULT_CHANGED: { guard: 'available', actions: 'defaultChanged' },
        BACK: { guard: 'navigable', target: 'review' },
        PUBLISH: { guard: 'canPublish', target: 'working', actions: ['publishing', 'publish'] },
      },
    },
    working: {
      on: { RETRY: {}, CONTINUE: {} },
      initial: 'running',
      states: {
        running: { on: { ASK_CANCEL: { guard: 'canCancel', target: 'confirmingCancellation' } } },
        confirmingCancellation: {
          on: {
            DISMISS_CANCEL: { guard: 'accepted', target: 'running' },
            CONFIRM_CANCEL: {
              guard: 'canCancel',
              target: 'running',
              actions: ['cancelling', 'cancel'],
            },
          },
        },
      },
    },
    confirmed: {
      on: {
        OPEN_DIRECT: {
          guard: 'canContinue',
          target: 'working',
          actions: ['editingDirect', 'continueEditing'],
        },
        OPEN_REFINEMENT: {
          guard: 'canContinue',
          target: 'working',
          actions: ['editingAI', 'continueEditing'],
        },
      },
    },
  },
})

export function studioFlowView(snapshot: SnapshotFrom<typeof studioFlowMachine>): StudioView {
  return snapshot.matches('working') ? 'working' : (snapshot.value as StudioView)
}
