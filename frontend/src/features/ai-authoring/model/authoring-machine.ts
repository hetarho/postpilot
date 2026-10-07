import {
  assign,
  fromPromise,
  getInitialSnapshot,
  getNextSnapshot,
  setup,
  type SnapshotFrom,
} from 'xstate'
import {
  authoringScopeKey,
  authoringSessionBusy,
  completedAuthoringExchanges,
  AUTHORING_MESSAGE_MAX_CHARS,
  AUTHORING_MAX_EXCHANGES,
  type AuthoringScope,
  type AuthoringSession,
  type AuthoringEstimate,
  type AuthoringModelRef,
  type AuthoringMode,
  type AuthoringArtifact,
  type AuthoringCandidateCount,
} from '@/entities/ai-authoring'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

export interface FrozenAuthoringCommand {
  candidateCount?: AuthoringCandidateCount
  mode: AuthoringMode
  prompt: string
  writeModel: AuthoringModelRef
  sessionId: string
  expectedRevision: number
  createRequestId: string
  requestId: string
  confirmed?: boolean
}
export type StudioPhase =
  | 'checking'
  | 'idle'
  | 'choosing'
  | 'editing'
  | 'quoting'
  | 'confirming'
  | 'creating'
  | 'starting'
  | 'patching'
  | 'resetting'
  | 'selecting'
  | 'active'
  | 'saving'
  | 'cancelling'
  | 'saved'
  | 'failed'
export interface AuthoringState {
  directSource?: AuthoringArtifact
  sourceDirty?: boolean
  scopeKey: string
  phase: StudioPhase
  session?: AuthoringSession
  text: string
  operation: number
  command?: FrozenAuthoringCommand
  estimate?: AuthoringEstimate
  failure?: AppFailure
}
type Scoped = { scopeKey: string }
export type AuthoringEvent = Scoped &
  (
    | { type: 'hydrate'; session: AuthoringSession | null }
    | { type: 'draft'; text: string }
    | { type: 'source'; source: AuthoringArtifact }
    | { type: 'quote'; command: FrozenAuthoringCommand }
    | { type: 'quoted'; operation: number; estimate: AuthoringEstimate }
    | { type: 'dismiss-quote' }
    | { type: 'retry-quote' }
    | {
        type: 'begin'
        phase:
          'creating' | 'starting' | 'selecting' | 'saving' | 'cancelling' | 'patching' | 'resetting'
        retry?: AuthoringRetry
      }
    | { type: 'created'; operation: number; session: AuthoringSession }
    | { type: 'response'; operation: number; session: AuthoringSession; clearText?: boolean }
    | { type: 'failed'; operation: number; failure: AppFailure }
    | { type: 'new-session' | 'retry-load' }
  )

export type AuthoringRetry =
  | { type: 'edit'; requestId: string }
  | {
      type: 'patch'
      sessionId: string
      revision: number
      operationKey: string
      source: AuthoringArtifact
    }
  | {
      type: 'reset-chat' | 'reset-baseline'
      sessionId: string
      revision: number
      operationKey: string
    }
  | {
      type: 'select'
      sessionId: string
      revision: number
      candidateId: string
      operationKey?: string
    }
  | {
      type: 'save'
      sessionId: string
      revision: number
      makeDefault: boolean
      operationKey?: string
    }
  | { type: 'cancel'; sessionId: string; jobId: string }
type AuthoringContext = Omit<AuthoringState, 'phase'> & { retry?: AuthoringRetry }
export interface AuthoringWork {
  scopeKey: string
  operation: number
  phase: 'creating' | 'starting' | 'selecting' | 'saving' | 'cancelling' | 'patching' | 'resetting'
  command?: FrozenAuthoringCommand
  retry?: AuthoringRetry
  created: (session: AuthoringSession) => void
}
export interface AuthoringResult {
  scopeKey: string
  operation: number
  session: AuthoringSession
  clearText?: boolean
}
export interface AuthoringQuote {
  scopeKey: string
  operation: number
  command: FrozenAuthoringCommand
}
export interface AuthoringQuoted {
  scopeKey: string
  operation: number
  estimate: AuthoringEstimate
}
export function studioBusy(state: AuthoringState): boolean {
  return [
    'patching',
    'resetting',
    'quoting',
    'confirming',
    'creating',
    'starting',
    'selecting',
    'active',
    'saving',
    'cancelling',
  ].includes(state.phase)
}
function remotePhase(session: AuthoringSession | undefined): StudioPhase {
  if (!session) return 'idle'
  if (authoringSessionBusy(session)) return 'active'
  if (session.phase === 'saved') return 'saved'
  if (session.phase === 'failed') return 'failed'
  if (session.phase === 'choosing' && session.candidates.length) return 'choosing'
  return session.selected ? 'editing' : session.candidates.length ? 'choosing' : 'idle'
}
function owns(context: AuthoringContext, event: AuthoringEvent) {
  return event.scopeKey === context.scopeKey
}
function acceptsSession(context: AuthoringContext, session: AuthoringSession) {
  const [, kind, targetId] = JSON.parse(context.scopeKey) as [string, string, string]
  return (
    session.kind === kind &&
    session.targetId === targetId &&
    (!context.session ||
      (session.id === context.session.id && session.revision >= context.session.revision))
  )
}
function responseOf(event: unknown): AuthoringResult | undefined {
  const incoming = event as { type?: string; output?: AuthoringResult } & AuthoringEvent
  if (incoming.output) return incoming.output
  return incoming.type === 'response' ? incoming : undefined
}
function quoteOf(event: unknown): AuthoringQuoted | undefined {
  const incoming = event as { type?: string; output?: AuthoringQuoted } & AuthoringEvent
  return incoming.output ?? (incoming.type === 'quoted' ? incoming : undefined)
}
function validResult(context: AuthoringContext, event: unknown) {
  const response = responseOf(event)
  return (
    !!response &&
    response.scopeKey === context.scopeKey &&
    response.operation === context.operation &&
    acceptsSession(context, response.session)
  )
}
function validHydrate(context: AuthoringContext, event: AuthoringEvent) {
  return (
    owns(context, event) &&
    event.type === 'hydrate' &&
    (!event.session || acceptsSession(context, event.session)) &&
    (!!event.session || !context.session)
  )
}
function quoteValid(context: AuthoringContext, event: AuthoringEvent) {
  if (
    !owns(context, event) ||
    event.type !== 'quote' ||
    context.command ||
    authoringSessionBusy(context.session)
  )
    return false
  const command = event.command
  const chars = Array.from(command.prompt.trim()).length
  return (
    chars <= AUTHORING_MESSAGE_MAX_CHARS &&
    (command.mode !== 'refine' ||
      (!!(context.session?.workingSource ?? context.session?.selected) &&
        chars > 0 &&
        completedAuthoringExchanges(context.session) < AUTHORING_MAX_EXCHANGES)) &&
    !!command.writeModel.providerId &&
    !!command.writeModel.modelId &&
    !!command.requestId &&
    !!command.createRequestId &&
    command.sessionId === (context.session?.id ?? '') &&
    command.expectedRevision === (context.session?.revision ?? 0)
  )
}
function priced(estimate: AuthoringEstimate | undefined) {
  return (
    !!estimate &&
    (estimate.free ||
      (typeof estimate.credits === 'number' &&
        Number.isFinite(estimate.credits) &&
        estimate.credits >= 0))
  )
}
const responseTransitions = (
  ['active', 'saved', 'failed', 'editing', 'choosing', 'idle'] as const
).map((phase) => ({
  guard: { type: 'resultPhase' as const, params: { phase } },
  target: phase,
  actions: ['notifySaved', 'response'] as const,
}))
const hydrateTransitions = (
  ['active', 'saved', 'failed', 'editing', 'choosing', 'idle'] as const
).map((phase) => ({
  guard: { type: 'hydrationPhase' as const, params: { phase } },
  target: phase,
  actions: ['notifyHydratedSave', 'hydrate'] as const,
}))
const available = {
  hydrate: hydrateTransitions,
  draft: { guard: 'canDraft', actions: 'draft' },
  quote: { guard: 'validQuote', target: 'quoting', actions: 'freezeQuote' },
  source: { guard: 'canDraft', actions: 'source' },
  begin: [
    { guard: 'patchAllowed', target: 'patching', actions: 'begin' },
    { guard: 'resetAllowed', target: 'resetting', actions: 'begin' },
    { guard: 'createAllowed', target: 'creating', actions: 'begin' },
    { guard: 'selectAllowed', target: 'selecting', actions: 'begin' },
    { guard: 'saveAllowed', target: 'saving', actions: 'begin' },
  ],
  'retry-load': { guard: 'owned', target: 'checking', actions: 'clearFailure' },
  'new-session': { guard: 'owned', target: 'idle', actions: 'fresh' },
} as const
const invocation = (phase: AuthoringWork['phase']) => ({
  src: 'execute' as const,
  input: ({
    context,
    self,
  }: {
    context: AuthoringContext
    self: { send: (event: AuthoringEvent) => void }
  }) => ({
    scopeKey: context.scopeKey,
    operation: context.operation,
    phase,
    command: context.command,
    retry: context.retry,
    created: (session: AuthoringSession) =>
      self.send({
        type: 'created',
        scopeKey: context.scopeKey,
        operation: context.operation,
        session,
      }),
  }),
  onDone: responseTransitions,
  onError: { target: 'failed', actions: 'invocationFailed' as const },
})
export const authoringMachine = setup({
  types: {
    context: {} as AuthoringContext,
    events: {} as AuthoringEvent,
    input: {} as AuthoringScope,
  },
  actors: {
    estimate: fromPromise<AuthoringQuoted, AuthoringQuote>(async () => {
      throw new Error('Authoring estimate actor unavailable')
    }),
    execute: fromPromise<AuthoringResult, AuthoringWork>(async () => {
      throw new Error('Authoring command actor unavailable')
    }),
  },
  guards: {
    patchAllowed: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'patching' &&
      !!context.session &&
      !context.command,
    resetAllowed: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'resetting' &&
      !!context.session &&
      !context.command,
    owned: ({ context, event }) => owns(context, event),
    canDraft: ({ context, event }) => owns(context, event) && !context.command,
    validQuote: ({ context, event }) => quoteValid(context, event),
    hydrationPhase: ({ context, event }, params: { phase: StudioPhase }) =>
      validHydrate(context, event) &&
      event.type === 'hydrate' &&
      remotePhase(event.session ?? undefined) === params.phase,
    staleFailureHydrate: ({ context, event }) =>
      validHydrate(context, event) &&
      event.type === 'hydrate' &&
      !!context.failure &&
      !!event.session &&
      !!context.session &&
      event.session.revision <= context.session.revision &&
      !authoringSessionBusy(event.session),
    resultPhase: ({ context, event }, params: { phase: StudioPhase }) =>
      validResult(context, event) && remotePhase(responseOf(event)!.session) === params.phase,
    validQuoted: ({ context, event }) => {
      const result = quoteOf(event)
      return (
        !!result && result.scopeKey === context.scopeKey && result.operation === context.operation
      )
    },
    operationFailure: ({ context, event }) =>
      owns(context, event) && event.type === 'failed' && event.operation === context.operation,
    created: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'created' &&
      event.operation === context.operation &&
      acceptsSession(context, event.session),
    starting: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'starting' &&
      priced(context.estimate),
    retryStarting: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'starting' &&
      priced(context.estimate) &&
      !!context.command?.confirmed,
    retryQuote: ({ context, event }) =>
      owns(context, event) && !!context.command && !context.command.confirmed,
    createAllowed: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'creating' &&
      !context.session,
    selectAllowed: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'selecting' &&
      !!context.session?.candidates.length &&
      !context.command,
    saveAllowed: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'saving' &&
      !!context.session?.selected &&
      context.session?.draftState !== 'invalid' &&
      context.session?.draftState !== 'incomplete' &&
      !context.sourceDirty &&
      !context.command,
    recoveredSave: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'saving' &&
      context.session?.phase === 'saving' &&
      !context.session.activeJobId &&
      !!context.session.selected &&
      !context.command,
    conflictPatch: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'patching' &&
      context.session?.phase === 'saving' &&
      context.session.failureReason === 'AUTHORING_SAVE_CONFLICT' &&
      !context.session.activeJobId &&
      !context.command,
    cancelAllowed: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'begin' &&
      event.phase === 'cancelling' &&
      !!context.session?.activeJobId,
  },
  actions: {
    source: assign(({ event }) =>
      event.type === 'source' ? { directSource: { ...event.source }, sourceDirty: true } : {},
    ),
    draft: assign(({ event }) => (event.type === 'draft' ? { text: event.text } : {})),
    freezeQuote: assign(({ context, event }) =>
      event.type === 'quote'
        ? {
            command: { ...event.command, writeModel: { ...event.command.writeModel } },
            operation: context.operation + 1,
            failure: undefined,
            estimate: undefined,
          }
        : {},
    ),
    quoted: assign(({ event }) => ({ estimate: quoteOf(event)!.estimate })),
    begin: assign(({ context, event }) => ({
      operation: context.operation + 1,
      failure: undefined,
      retry: event.type === 'begin' ? (event.retry ?? context.retry) : context.retry,
      command:
        event.type === 'begin' && event.phase === 'starting' && context.command
          ? { ...context.command, confirmed: true }
          : context.command,
    })),
    created: assign(({ context, event }) =>
      event.type === 'created'
        ? {
            session: event.session,
            command: context.command
              ? {
                  ...context.command,
                  sessionId: event.session.id,
                  expectedRevision: event.session.revision,
                }
              : undefined,
          }
        : {},
    ),
    response: assign(({ context, event }) => {
      const response = responseOf(event)!
      return {
        session: response.session,
        directSource: response.session.workingSource ?? response.session.selected,
        sourceDirty: false,
        text: response.clearText ? '' : context.text,
        failure: undefined,
        command: undefined,
        estimate: undefined,
        retry: undefined,
      }
    }),
    hydrate: assign(({ context, event }) => {
      if (event.type !== 'hydrate') return {}
      const session = event.session ?? undefined
      const admitted =
        session &&
        (authoringSessionBusy(session) ||
          (context.command && session.revision > context.command.expectedRevision))
      const successful =
        context.session &&
        authoringSessionBusy(context.session) &&
        session &&
        !authoringSessionBusy(session) &&
        session.phase !== 'failed'
      return {
        session,
        directSource: context.sourceDirty
          ? context.directSource
          : (session?.workingSource ?? session?.selected),
        sourceDirty: context.sourceDirty,
        text: successful
          ? ''
          : context.text ||
            (session?.phase === 'failed' || authoringSessionBusy(session)
              ? (session?.pendingRequest ?? '')
              : ''),
        failure: undefined,
        command: admitted ? undefined : context.command,
        estimate: admitted ? undefined : context.estimate,
        retry: session?.phase === 'saved' ? undefined : context.retry,
      }
    }),
    failure: assign(({ event }) => (event.type === 'failed' ? { failure: event.failure } : {})),
    invocationFailed: assign(({ event }) => ({
      failure: appFailureFromConnect((event as unknown as { error: unknown }).error),
    })),
    clearFailure: assign({ failure: undefined }),
    dismiss: assign({ command: undefined, estimate: undefined }),
    retryQuote: assign(({ context }) => ({ operation: context.operation + 1, failure: undefined })),
    fresh: assign(({ context }) => ({
      session: undefined,
      text: '',
      operation: context.operation + 1,
      command: undefined,
      estimate: undefined,
      failure: undefined,
      retry: undefined,
    })),
    notifySaved: () => {},
    notifyHydratedSave: () => {},
  },
}).createMachine({
  id: 'configurationAuthoring',
  initial: 'checking',
  context: ({ input }) => ({ scopeKey: authoringScopeKey(input), text: '', operation: 0 }),
  states: {
    checking: {
      on: {
        hydrate: hydrateTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    idle: { on: available },
    choosing: { on: available },
    editing: { on: available },
    failed: {
      on: {
        ...available,
        hydrate: [{ guard: 'staleFailureHydrate' }, ...hydrateTransitions],
        'retry-quote': { guard: 'retryQuote', target: 'quoting', actions: 'retryQuote' },
        begin: [
          { guard: 'retryStarting', target: 'starting', actions: 'begin' },
          ...available.begin,
        ],
      },
    },
    quoting: {
      invoke: {
        src: 'estimate',
        input: ({ context }) => ({
          scopeKey: context.scopeKey,
          operation: context.operation,
          command: context.command!,
        }),
        onDone: { guard: 'validQuoted', target: 'confirming', actions: 'quoted' },
        onError: { target: 'failed', actions: 'invocationFailed' },
      },
      on: {
        quoted: { guard: 'validQuoted', target: 'confirming', actions: 'quoted' },
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    confirming: {
      on: {
        begin: { guard: 'starting', target: 'starting', actions: 'begin' },
        'dismiss-quote': [
          {
            guard: ({ context, event }) =>
              owns(context, event) && remotePhase(context.session) === 'editing',
            target: 'editing',
            actions: 'dismiss',
          },
          {
            guard: ({ context, event }) =>
              owns(context, event) && remotePhase(context.session) === 'choosing',
            target: 'choosing',
            actions: 'dismiss',
          },
          { guard: 'owned', target: 'idle', actions: 'dismiss' },
        ],
      },
    },
    creating: {
      invoke: invocation('creating'),
      on: {
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    starting: {
      invoke: invocation('starting'),
      on: {
        created: { guard: 'created', actions: 'created' },
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    patching: {
      invoke: invocation('patching'),
      on: {
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    resetting: {
      invoke: invocation('resetting'),
      on: {
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    selecting: {
      invoke: invocation('selecting'),
      on: {
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    saving: {
      invoke: invocation('saving'),
      on: {
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    cancelling: {
      invoke: invocation('cancelling'),
      on: {
        response: responseTransitions,
        failed: { guard: 'operationFailure', target: 'failed', actions: 'failure' },
      },
    },
    active: {
      on: {
        hydrate: hydrateTransitions,
        begin: [
          { guard: 'recoveredSave', target: 'saving', actions: 'begin' },
          { guard: 'conflictPatch', target: 'patching', actions: 'begin' },
          { guard: 'cancelAllowed', target: 'cancelling', actions: 'begin' },
        ],
      },
    },
    saved: {
      on: {
        hydrate: hydrateTransitions,
        begin: available.begin,
        source: available.source,
        'new-session': { guard: 'owned', target: 'idle', actions: 'fresh' },
        'retry-load': { guard: 'owned', target: 'checking', actions: 'clearFailure' },
      },
    },
  },
})
const projections = new WeakMap<object, AuthoringState>()
export function authoringStateOf(snapshot: SnapshotFrom<typeof authoringMachine>): AuthoringState {
  let state = projections.get(snapshot)
  if (!state) {
    const {
      scopeKey,
      session,
      text,
      operation,
      command,
      estimate,
      failure,
      directSource,
      sourceDirty,
    } = snapshot.context
    const context = {
      scopeKey,
      session,
      text,
      operation,
      command,
      estimate,
      failure,
      directSource,
      sourceDirty,
    }
    state = { ...context, phase: snapshot.value as StudioPhase }
    projections.set(snapshot, state)
  }
  return state
}
export function initialAuthoringState(scope: AuthoringScope): AuthoringState {
  return authoringStateOf(getInitialSnapshot(authoringMachine, scope))
}
export function authoringTransition(state: AuthoringState, event: AuthoringEvent): AuthoringState {
  const { phase, ...context } = state
  const snapshot = authoringMachine.resolveState({ value: phase, context })
  const next = getNextSnapshot(authoringMachine, snapshot, event)
  return next.context === snapshot.context && next.value === snapshot.value
    ? state
    : authoringStateOf(next)
}
