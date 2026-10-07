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
  type AuthoringArtifact,
  type AuthoringCandidateCount,
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
  initialSessionId = '',
) {
  const scopeKey = authoringScopeKey(scope)
  const latest = useLatestAuthoringSession(scope, !initialSessionId)
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
          input.command.candidateCount ?? 8,
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
            session = await api.select(
              retry.sessionId,
              retry.revision,
              retry.candidateId,
              retry.operationKey,
            )
          else if (retry.type === 'save')
            session = await api.save(
              retry.sessionId,
              retry.revision,
              retry.makeDefault,
              retry.operationKey,
            )
          else if (retry.type === 'patch')
            session = await api.patch(
              retry.sessionId,
              retry.revision,
              retry.operationKey,
              retry.source,
            )
          else if (retry.type === 'reset-chat')
            session = await api.resetChat(retry.sessionId, retry.revision, retry.operationKey)
          else if (retry.type === 'reset-baseline')
            session = await api.resetBaseline(retry.sessionId, retry.revision, retry.operationKey)
          else if (retry.type === 'cancel') session = await api.cancel(retry.sessionId, retry.jobId)
          else throw new Error('Authoring action unavailable')
        }
        return {
          scopeKey: input.scopeKey,
          operation: input.operation,
          session,
          clearText: input.phase === 'starting' || input.retry?.type === 'reset-chat',
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
  const sessionQuery = useAuthoringSession(scope, state.session?.id ?? initialSessionId)
  const restoreQuery = initialSessionId ? sessionQuery : latest
  const job = useJob(state.session?.activeJobId ?? '', [sessionQuery.queryKey])
  useEffect(() => {
    if (actorRef.getSnapshot().value !== 'checking') return
    if (restoreQuery.isError)
      send({
        type: 'failed',
        scopeKey,
        operation: getSnapshot().operation,
        failure: appFailureFromConnect(restoreQuery.error),
      })
    else if (restoreQuery.data !== undefined)
      send({ type: 'hydrate', scopeKey, session: restoreQuery.data })
  }, [
    restoreQuery.data,
    restoreQuery.error,
    restoreQuery.isError,
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
      retry.type === 'patch'
        ? 'patching'
        : retry.type === 'reset-chat' || retry.type === 'reset-baseline'
          ? 'resetting'
          : retry.type === 'edit'
            ? 'creating'
            : retry.type === 'select'
              ? 'selecting'
              : retry.type === 'save'
                ? 'saving'
                : 'cancelling'
    return send({ type: 'begin', scopeKey, phase, retry })
  }
  const quote = (
    mode: AuthoringMode,
    model: AuthoringModelRef,
    candidateCount: AuthoringCandidateCount = 8,
  ) => {
    const before = getSnapshot()
    return send({
      type: 'quote',
      scopeKey,
      command: {
        mode,
        candidateCount,
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
    const result = await restoreQuery.refetch()
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
        run({
          type: 'select',
          sessionId: session.id,
          revision: session.revision,
          candidateId,
          operationKey: crypto.randomUUID(),
        })
    },
    save: (makeDefault: boolean) => {
      const session = getSnapshot().session
      if (session)
        run({
          type: 'save',
          sessionId: session.id,
          revision: session.revision,
          makeDefault,
          operationKey: crypto.randomUUID(),
        })
    },
    cancel: () => {
      const session = getSnapshot().session
      if (session?.activeJobId)
        run({ type: 'cancel', sessionId: session.id, jobId: session.activeJobId })
    },
    setSource: (source: AuthoringArtifact) => send({ type: 'source', scopeKey, source }),
    patch: () => {
      const before = getSnapshot()
      const source =
        before.directSource ?? before.session?.workingSource ?? before.session?.selected
      if (before.session && source)
        return run({
          type: 'patch',
          sessionId: before.session.id,
          revision: before.session.revision,
          operationKey: crypto.randomUUID(),
          source: { ...source },
        })
    },
    resetBaseline: () => {
      const session = getSnapshot().session
      if (session)
        return run({
          type: 'reset-baseline',
          sessionId: session.id,
          revision: session.revision,
          operationKey: crypto.randomUUID(),
        })
    },
    fresh: () => {
      const session = getSnapshot().session
      if (session)
        return run({
          type: 'reset-chat',
          sessionId: session.id,
          revision: session.revision,
          operationKey: crypto.randomUUID(),
        })
      return send({ type: 'new-session', scopeKey })
    },
    readFailure: sessionQuery.isError ? appFailureFromConnect(sessionQuery.error) : undefined,
    retryRead: () => {
      void sessionQuery.refetch()
      job.refetch()
    },
    activeJobFailed: !!job.job && isTerminal(job.job) && job.job.status !== 'done',
  }
}
