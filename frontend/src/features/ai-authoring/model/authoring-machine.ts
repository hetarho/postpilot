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
} from '@/entities/ai-authoring'
import type { AppFailure } from '@/shared/api'

export interface FrozenAuthoringCommand {
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
  | 'selecting'
  | 'active'
  | 'saving'
  | 'cancelling'
  | 'saved'
  | 'failed'
export interface AuthoringState {
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
    | { type: 'quote'; command: FrozenAuthoringCommand }
    | { type: 'quoted'; operation: number; estimate: AuthoringEstimate }
    | { type: 'dismiss-quote' }
    | { type: 'retry-quote' }
    | { type: 'begin'; phase: 'creating' | 'starting' | 'selecting' | 'saving' | 'cancelling' }
    | { type: 'created'; operation: number; session: AuthoringSession }
    | { type: 'response'; operation: number; session: AuthoringSession; clearText?: boolean }
    | { type: 'failed'; operation: number; failure: AppFailure }
    | { type: 'new-session' | 'retry-load' }
  )
export function initialAuthoringState(scope: AuthoringScope): AuthoringState {
  return { scopeKey: authoringScopeKey(scope), phase: 'checking', text: '', operation: 0 }
}
export function studioBusy(state: AuthoringState): boolean {
  return [
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
  return session.selected ? 'editing' : session.candidates.length ? 'choosing' : 'idle'
}
function acceptsSession(state: AuthoringState, session: AuthoringSession): boolean {
  const [, kind, targetId] = JSON.parse(state.scopeKey) as [string, string, string]
  return (
    session.kind === kind &&
    session.targetId === targetId &&
    (!state.session ||
      (session.id === state.session.id && session.revision >= state.session.revision))
  )
}
export function authoringTransition(state: AuthoringState, event: AuthoringEvent): AuthoringState {
  if (event.scopeKey !== state.scopeKey) return state
  if (event.type === 'hydrate') {
    if (studioBusy(state) && state.phase !== 'active') return state
    if (event.session && !acceptsSession(state, event.session)) return state
    if (!event.session && state.session) return state
    if (
      state.phase === 'failed' &&
      state.failure &&
      event.session &&
      state.session &&
      event.session.revision <= state.session.revision &&
      !authoringSessionBusy(event.session)
    )
      return state
    const finished =
      state.phase === 'active' &&
      event.session &&
      !authoringSessionBusy(event.session) &&
      event.session.phase !== 'failed'
    const admitted =
      event.session &&
      (authoringSessionBusy(event.session) ||
        (state.command && event.session.revision > state.command.expectedRevision))
    return {
      ...state,
      session: event.session ?? undefined,
      phase: remotePhase(event.session ?? undefined),
      text: finished
        ? ''
        : state.text ||
          (event.session?.phase === 'failed' || authoringSessionBusy(event.session ?? undefined)
            ? (event.session?.pendingRequest ?? '')
            : ''),
      failure: undefined,
      command: admitted ? undefined : state.command,
      estimate: admitted ? undefined : state.estimate,
    }
  }
  if (
    event.type === 'quoted' ||
    event.type === 'failed' ||
    event.type === 'created' ||
    event.type === 'response'
  ) {
    if (event.operation !== state.operation || (!studioBusy(state) && state.phase !== 'checking'))
      return state
    if (event.type === 'failed') return { ...state, phase: 'failed', failure: event.failure }
    if (event.type === 'quoted')
      return state.phase === 'quoting'
        ? { ...state, phase: 'confirming', estimate: event.estimate }
        : state
    if (!acceptsSession(state, event.session)) return state
    if (event.type === 'created')
      return {
        ...state,
        session: event.session,
        command: state.command
          ? {
              ...state.command,
              sessionId: event.session.id,
              expectedRevision: event.session.revision,
            }
          : undefined,
      }
    return {
      ...state,
      session: event.session,
      phase: remotePhase(event.session),
      failure: undefined,
      text: event.clearText ? '' : state.text,
      command: undefined,
      estimate: undefined,
    }
  }
  if (event.type === 'draft')
    return !studioBusy(state) && !state.command ? { ...state, text: event.text } : state
  if (event.type === 'retry-load')
    return !studioBusy(state) ? { ...state, phase: 'checking', failure: undefined } : state
  if (event.type === 'new-session')
    return !studioBusy(state)
      ? { scopeKey: state.scopeKey, phase: 'idle', text: '', operation: state.operation + 1 }
      : state
  if (event.type === 'retry-quote')
    return state.phase === 'failed' && state.command && !state.command.confirmed
      ? { ...state, phase: 'quoting', operation: state.operation + 1, failure: undefined }
      : state
  if (event.type === 'dismiss-quote')
    return state.phase === 'confirming'
      ? { ...state, phase: remotePhase(state.session), command: undefined, estimate: undefined }
      : state
  if (event.type === 'quote') {
    if (
      studioBusy(state) ||
      state.phase === 'checking' ||
      state.phase === 'saved' ||
      state.command ||
      authoringSessionBusy(state.session)
    )
      return state
    const command = event.command
    const chars = Array.from(command.prompt.trim()).length
    if (
      chars > AUTHORING_MESSAGE_MAX_CHARS ||
      (command.mode === 'refine' &&
        (!state.session?.selected ||
          chars === 0 ||
          completedAuthoringExchanges(state.session) >= AUTHORING_MAX_EXCHANGES))
    )
      return state
    if (
      !command.writeModel.providerId ||
      !command.writeModel.modelId ||
      !command.requestId ||
      !command.createRequestId ||
      command.sessionId !== (state.session?.id ?? '') ||
      command.expectedRevision !== (state.session?.revision ?? 0)
    )
      return state
    return {
      ...state,
      phase: 'quoting',
      operation: state.operation + 1,
      command,
      failure: undefined,
      estimate: undefined,
    }
  }
  if (event.type === 'begin') {
    if (event.phase === 'starting') {
      if (
        (state.phase !== 'confirming' && !(state.phase === 'failed' && state.command?.confirmed)) ||
        !state.estimate
      )
        return state
    } else if (event.phase === 'creating') {
      if (studioBusy(state) || state.session) return state
    } else if (event.phase === 'cancelling') {
      if (state.phase !== 'active' || !state.session?.activeJobId) return state
    } else if (event.phase === 'selecting') {
      if (studioBusy(state) || !state.session?.candidates.length || state.command) return state
    } else if (event.phase === 'saving') {
      const recoveredSave =
        state.phase === 'active' && state.session?.phase === 'saving' && !state.session.activeJobId
      if (
        (studioBusy(state) && !recoveredSave) ||
        !state.session?.selected ||
        state.phase === 'saved' ||
        state.command
      )
        return state
    }
    return {
      ...state,
      phase: event.phase,
      operation: state.operation + 1,
      failure: undefined,
      command:
        event.phase === 'starting' && state.command
          ? { ...state.command, confirmed: true }
          : state.command,
    }
  }
  return state
}
