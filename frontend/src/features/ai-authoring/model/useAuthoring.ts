import { useCallback, useEffect, useRef, useState } from 'react'
import {
  authoringScopeKey,
  useAuthoringAPI,
  useAuthoringSession,
  useLatestAuthoringSession,
  type AuthoringScope,
  type AuthoringSavedRef,
  type AuthoringMode,
  type AuthoringModelRef,
} from '@/entities/ai-authoring'
import { isTerminal, useJob } from '@/entities/generation-job'
import { appFailureFromConnect } from '@/shared/api'
import {
  initialAuthoringState,
  authoringTransition,
  studioBusy,
  type AuthoringEvent,
} from './authoring-machine'

type Retry =
  | { type: 'edit'; requestId: string }
  | { type: 'select'; sessionId: string; revision: number; candidateId: string }
  | { type: 'save'; sessionId: string; revision: number; makeDefault: boolean }
  | { type: 'cancel'; sessionId: string; jobId: string }
export function useAuthoring(
  scope: AuthoringScope,
  callbacks: { onSaved?: (ref: AuthoringSavedRef) => void; onBusyChange?: (busy: boolean) => void },
) {
  const scopeKey = authoringScopeKey(scope)
  const [state, setState] = useState(() => initialAuthoringState(scope))
  const current = useRef(state)
  const mounted = useRef(true)
  const latest = useLatestAuthoringSession(scope)
  const api = useAuthoringAPI(scope)
  const sessionQuery = useAuthoringSession(scope, state.session?.id ?? '')
  const retry = useRef<Retry | undefined>(undefined)
  const callbackRef = useRef(callbacks)
  useEffect(() => {
    callbackRef.current = callbacks
  }, [callbacks])
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      callbackRef.current.onBusyChange?.(false)
    }
  }, [])
  const send = useCallback((event: AuthoringEvent) => {
    const before = current.current
    if (!mounted.current) return before
    const next = authoringTransition(before, event)
    current.current = next
    if (next !== before) {
      callbackRef.current.onBusyChange?.(studioBusy(next))
      setState(next)
    }
    return next
  }, [])
  const fail = (operation: number, error: unknown) =>
    send({ type: 'failed', scopeKey, operation, failure: appFailureFromConnect(error) })
  useEffect(() => {
    if (current.current.phase !== 'checking') return
    if (latest.isError)
      send({
        type: 'failed',
        scopeKey,
        operation: current.current.operation,
        failure: appFailureFromConnect(latest.error),
      })
    else if (latest.data !== undefined) send({ type: 'hydrate', scopeKey, session: latest.data })
  }, [latest.data, latest.error, latest.isError, scopeKey, send])
  useEffect(() => {
    if (!sessionQuery.data) return
    const before = current.current
    const next = send({ type: 'hydrate', scopeKey, session: sessionQuery.data })
    const publication = retry.current
    if (
      next !== before &&
      next.phase === 'saved' &&
      next.session?.saved &&
      publication?.type === 'save' &&
      publication.sessionId === next.session.id &&
      mounted.current
    ) {
      retry.current = undefined
      callbackRef.current.onSaved?.(next.session.saved)
    }
  }, [sessionQuery.data, state.phase, scopeKey, send])
  const job = useJob(state.session?.activeJobId ?? '', [sessionQuery.queryKey])
  const publish = (
    operation: number,
    session: NonNullable<typeof state.session>,
    clearText = false,
  ) => {
    const before = current.current
    const next = send({ type: 'response', scopeKey, operation, session, clearText })
    if (
      next !== before &&
      next.phase === 'saved' &&
      session.saved &&
      mounted.current &&
      next.operation === operation &&
      retry.current?.type === 'save' &&
      retry.current.sessionId === session.id
    ) {
      retry.current = undefined
      callbackRef.current.onSaved?.(session.saved)
    }
  }
  const quote = async (mode: AuthoringMode, model: AuthoringModelRef) => {
    const before = current.current
    const next = send({
      type: 'quote',
      scopeKey,
      command: {
        mode,
        prompt: before.text.trim(),
        writeModel: { ...model },
        sessionId: before.session?.id ?? '',
        expectedRevision: before.session?.revision ?? 0,
        createRequestId: crypto.randomUUID(),
        requestId: crypto.randomUUID(),
      },
    })
    if (next === before || !next.command) return
    try {
      send({
        type: 'quoted',
        scopeKey,
        operation: next.operation,
        estimate: await api.estimate(
          next.command.mode,
          next.command.writeModel,
          next.command.sessionId,
        ),
      })
    } catch (error) {
      fail(next.operation, error)
    }
  }
  const confirm = async () => {
    const before = current.current
    const next = send({ type: 'begin', scopeKey, phase: 'starting' })
    if (next === before || !next.command) return
    let command = next.command
    try {
      if (!command.sessionId) {
        const session = await api.create(command.createRequestId)
        const created = send({ type: 'created', scopeKey, operation: next.operation, session })
        if (!mounted.current || created.operation !== next.operation || !created.command) return
        command = created.command
      }
      const result = await api.start(command)
      publish(next.operation, result.session, true)
    } catch (error) {
      fail(next.operation, error)
    }
  }
  const run = async (action: Retry) => {
    const before = current.current
    const phase =
      action.type === 'edit'
        ? 'creating'
        : action.type === 'select'
          ? 'selecting'
          : action.type === 'save'
            ? 'saving'
            : 'cancelling'
    const next = send({ type: 'begin', scopeKey, phase })
    if (next === before) return
    retry.current = action
    try {
      const session =
        action.type === 'edit'
          ? await api.create(action.requestId)
          : action.type === 'select'
            ? await api.select(action.sessionId, action.revision, action.candidateId)
            : action.type === 'save'
              ? await api.save(action.sessionId, action.revision, action.makeDefault)
              : await api.cancel(action.sessionId, action.jobId)
      publish(next.operation, session)
      retry.current = undefined
    } catch (error) {
      fail(next.operation, error)
    }
  }
  const retryOperation = async () => {
    const snapshot = current.current
    if (snapshot.command?.confirmed) return confirm()
    if (snapshot.command) {
      const next = send({ type: 'retry-quote', scopeKey })
      if (next === snapshot || !next.command) return
      try {
        send({
          type: 'quoted',
          scopeKey,
          operation: next.operation,
          estimate: await api.estimate(
            next.command.mode,
            next.command.writeModel,
            next.command.sessionId,
          ),
        })
      } catch (error) {
        fail(next.operation, error)
      }
      return
    }
    if (retry.current) return run(retry.current)
    send({ type: 'retry-load', scopeKey })
    const result = await latest.refetch()
    if (result.data !== undefined) send({ type: 'hydrate', scopeKey, session: result.data })
  }
  return {
    state,
    latest,
    sessionQuery,
    job,
    busy: studioBusy(state),
    setText: (text: string) => send({ type: 'draft', scopeKey, text }),
    quote,
    confirm,
    retryOperation,
    dismissQuote: () => send({ type: 'dismiss-quote', scopeKey }),
    edit: () =>
      run({
        type: 'edit',
        requestId: retry.current?.type === 'edit' ? retry.current.requestId : crypto.randomUUID(),
      }),
    select: (candidateId: string) => {
      const session = current.current.session
      if (session?.candidates.some((candidate) => candidate.id === candidateId))
        void run({ type: 'select', sessionId: session.id, revision: session.revision, candidateId })
    },
    save: (makeDefault: boolean) => {
      const session = current.current.session
      if (session)
        void run({ type: 'save', sessionId: session.id, revision: session.revision, makeDefault })
    },
    cancel: () => {
      const session = current.current.session
      if (session?.activeJobId)
        void run({ type: 'cancel', sessionId: session.id, jobId: session.activeJobId })
    },
    fresh: () => {
      retry.current = undefined
      send({ type: 'new-session', scopeKey })
    },
    readFailure: sessionQuery.isError ? appFailureFromConnect(sessionQuery.error) : undefined,
    retryRead: () => {
      void sessionQuery.refetch()
      job.refetch()
    },
    activeJobFailed: !!job.job && isTerminal(job.job) && job.job.status !== 'done',
  }
}
