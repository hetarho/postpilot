import { useCallback, useEffect, useRef } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { fromPromise, type SnapshotFrom } from 'xstate'
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
  authoringMachine,
  authoringStateOf,
  studioBusy,
  type AuthoringEvent,
  type AuthoringRetry,
  type AuthoringQuote,
  type AuthoringQuoted,
  type AuthoringWork,
  type AuthoringResult,
} from './authoring-machine'

export function useAuthoring(
  scope: AuthoringScope,
  callbacks: { onSaved?: (ref: AuthoringSavedRef) => void; onBusyChange?: (busy: boolean) => void },
) {
  const scopeKey = authoringScopeKey(scope)
  const latest = useLatestAuthoringSession(scope)
  const api = useAuthoringAPI(scope)
  const callbackRef = useRef(callbacks)
  useEffect(() => {
    callbackRef.current = callbacks
  }, [callbacks])
  const onBusyChange = callbacks.onBusyChange
  const observer = useCallback(
    (snapshot: SnapshotFrom<typeof authoringMachine>) => {
      onBusyChange?.(studioBusy(authoringStateOf(snapshot)))
    },
    [onBusyChange],
  )
  const logic = authoringMachine.provide({
    actors: {
      estimate: fromPromise<AuthoringQuoted, AuthoringQuote>(async ({ input }) => ({
        scopeKey: input.scopeKey,
        operation: input.operation,
        estimate: await api.estimate(
          input.command.mode,
          input.command.writeModel,
          input.command.sessionId,
        ),
      })),
      execute: fromPromise<AuthoringResult, AuthoringWork>(async ({ input, signal }) => {
        let session
        if (input.phase === 'starting') {
          let command = input.command
          if (!command) throw new Error('Authoring confirmed command unavailable')
          if (!command.sessionId) {
            session = await api.create(command.createRequestId)
            // A confirmed create may finish after the view closes. Never continue its paid start.
            if (signal.aborted) throw new DOMException('Authoring view closed', 'AbortError')
            input.created(session)
            command = { ...command, sessionId: session.id, expectedRevision: session.revision }
          }
          if (signal.aborted) throw new DOMException('Authoring view closed', 'AbortError')
          session = (await api.start(command)).session
        } else {
          const retry = input.retry
          if (!retry) throw new Error('Authoring explicit operation unavailable')
          if (retry.type === 'edit') session = await api.create(retry.requestId)
          else if (retry.type === 'select')
            session = await api.select(retry.sessionId, retry.revision, retry.candidateId)
          else if (retry.type === 'save')
            session = await api.save(retry.sessionId, retry.revision, retry.makeDefault)
          else session = await api.cancel(retry.sessionId, retry.jobId)
        }
        return {
          scopeKey: input.scopeKey,
          operation: input.operation,
          session,
          clearText: input.phase === 'starting',
        }
      }),
    },
    actions: {
      notifySaved: ({ context, event }) => {
        const response =
          (event as unknown as { output?: AuthoringResult }).output ??
          (event.type === 'response' ? event : undefined)
        if (
          response?.session.saved &&
          context.retry?.type === 'save' &&
          response.scopeKey === scopeKey &&
          response.operation === context.operation &&
          context.retry.sessionId === response.session.id
        )
          callbacks.onSaved?.(response.session.saved)
      },
      notifyHydratedSave: ({ context, event }) => {
        if (
          event.type === 'hydrate' &&
          event.scopeKey === scopeKey &&
          event.session?.phase === 'saved' &&
          event.session.saved &&
          context.retry?.type === 'save' &&
          context.retry.sessionId === event.session.id
        )
          callbacks.onSaved?.(event.session.saved)
      },
    },
  })
  const actorRef = useActorRef(logic, { input: scope }, observer)
  const state = useSelector(actorRef, authoringStateOf)
  const getSnapshot = useCallback(() => authoringStateOf(actorRef.getSnapshot()), [actorRef])
  const send = useCallback(
    (event: AuthoringEvent) => {
      if (actorRef.getSnapshot().status === 'active') actorRef.send(event)
      return authoringStateOf(actorRef.getSnapshot())
    },
    [actorRef],
  )
  useEffect(() => () => callbackRef.current.onBusyChange?.(false), [])
  const sessionQuery = useAuthoringSession(scope, state.session?.id ?? '')
  const job = useJob(state.session?.activeJobId ?? '', [sessionQuery.queryKey])
  useEffect(() => {
    if (actorRef.getSnapshot().value !== 'checking') return
    if (latest.isError)
      send({
        type: 'failed',
        scopeKey,
        operation: getSnapshot().operation,
        failure: appFailureFromConnect(latest.error),
      })
    else if (latest.data !== undefined) send({ type: 'hydrate', scopeKey, session: latest.data })
  }, [
    latest.data,
    latest.error,
    latest.isError,
    state.phase,
    scopeKey,
    send,
    actorRef,
    getSnapshot,
  ])
  useEffect(() => {
    if (sessionQuery.data) send({ type: 'hydrate', scopeKey, session: sessionQuery.data })
  }, [sessionQuery.data, state.phase, scopeKey, send])
  const run = (retry: AuthoringRetry) => {
    const phase =
      retry.type === 'edit'
        ? 'creating'
        : retry.type === 'select'
          ? 'selecting'
          : retry.type === 'save'
            ? 'saving'
            : 'cancelling'
    return send({ type: 'begin', scopeKey, phase, retry })
  }
  const quote = (mode: AuthoringMode, model: AuthoringModelRef) => {
    const before = getSnapshot()
    return send({
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
  }
  const confirm = () => send({ type: 'begin', scopeKey, phase: 'starting' })
  const retryOperation = async () => {
    const context = actorRef.getSnapshot().context
    if (context.command?.confirmed) {
      confirm()
      return
    }
    if (context.command) {
      send({ type: 'retry-quote', scopeKey })
      return
    }
    if (context.retry) {
      run(context.retry)
      return
    }
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
    edit: () => {
      const retry = actorRef.getSnapshot().context.retry
      return run({
        type: 'edit',
        requestId: retry?.type === 'edit' ? retry.requestId : crypto.randomUUID(),
      })
    },
    select: (candidateId: string) => {
      const session = getSnapshot().session
      if (session?.candidates.some((candidate) => candidate.id === candidateId))
        run({ type: 'select', sessionId: session.id, revision: session.revision, candidateId })
    },
    save: (makeDefault: boolean) => {
      const session = getSnapshot().session
      if (session)
        run({ type: 'save', sessionId: session.id, revision: session.revision, makeDefault })
    },
    cancel: () => {
      const session = getSnapshot().session
      if (session?.activeJobId)
        run({ type: 'cancel', sessionId: session.id, jobId: session.activeJobId })
    },
    fresh: () => send({ type: 'new-session', scopeKey }),
    readFailure: sessionQuery.isError ? appFailureFromConnect(sessionQuery.error) : undefined,
    retryRead: () => {
      void sessionQuery.refetch()
      job.refetch()
    },
    activeJobFailed: !!job.job && isTerminal(job.job) && job.job.status !== 'done',
  }
}
