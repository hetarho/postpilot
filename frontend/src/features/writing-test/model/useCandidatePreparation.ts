import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import type { CandidatePreparationClient } from '@/entities/writing-test'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  candidatePreparationMachine,
  preparationRecovery,
  type PreparationDraft,
  type PreparationEvent,
  type PreparationRecovery,
  type PreparationPhase,
  type PreparationContext,
} from './candidate-preparation-machine'

type PreparationAction = PreparationEvent extends infer E
  ? E extends PreparationEvent
    ? Omit<E, 'scopeKey'>
    : never
  : never
export interface PreparationFlowResult {
  actorRef: {
    send(event: PreparationEvent): void
    getSnapshot(): { context: PreparationContext; value: unknown }
  }
  snapshot: { context: PreparationContext; value: unknown }
  context: PreparationContext
  phase: PreparationPhase
  send(event: PreparationAction): void
}
export function useCandidatePreparation(input: {
  ownerId: string
  seedKey: string
  client: CandidatePreparationClient
  draft: PreparationDraft
}): PreparationFlowResult {
  const scopeKey = JSON.stringify([input.ownerId, input.seedKey])
  const storageKey = `postpilot:test-candidates:${scopeKey}`
  const [recovery] = useState<PreparationRecovery | undefined>(() => {
    try {
      const stored: unknown = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
      if (
        !stored ||
        typeof stored !== 'object' ||
        !('scopeKey' in stored) ||
        stored.scopeKey !== scopeKey ||
        !('recovery' in stored)
      )
        return undefined
      return preparationRecovery(stored.recovery)
    } catch {
      return undefined
    }
  })
  const actorRef = useActorRef(candidatePreparationMachine, { input: { ...input, recovery } })
  const previousParameters = useRef({ kind: input.draft.kind, count: input.draft.count })
  const snapshot = useSelector(actorRef, (value) => value)
  const phase = snapshot.value as PreparationPhase
  const send = useCallback(
    (
      event: PreparationEvent extends infer E
        ? E extends PreparationEvent
          ? Omit<E, 'scopeKey'>
          : never
        : never,
    ) => {
      if (actorRef.getSnapshot().context.scopeKey !== scopeKey) return
      actorRef.send({ ...event, scopeKey } as PreparationEvent)
    },
    [actorRef, scopeKey],
  )
  useLayoutEffect(() => {
    const current = actorRef.getSnapshot().context
    if (current.scopeKey !== scopeKey && !current.suspended)
      actorRef.send({ type: 'SUSPEND', scopeKey: current.scopeKey })
  }, [actorRef, scopeKey])
  useEffect(() => {
    const changed =
      previousParameters.current.kind !== input.draft.kind ||
      previousParameters.current.count !== input.draft.count
    previousParameters.current = { kind: input.draft.kind, count: input.draft.count }
    if (!changed) return
    const current = actorRef.getSnapshot().context
    if (
      current.scopeKey !== scopeKey ||
      current.uncertain ||
      current.pending ||
      !['idle', 'quoted', 'ready', 'failed'].includes(phase) ||
      (current.draft.kind === input.draft.kind && current.draft.count === input.draft.count)
    )
      return
    send({
      type: 'EDIT',
      draft: {
        ...input.draft,
        prompt: current.draft.prompt,
        writeModel: input.draft.writeModel ?? current.draft.writeModel,
      },
    })
  }, [actorRef, scopeKey, input.draft, phase, send])
  useEffect(() => {
    const persist = () => {
      const context = actorRef.getSnapshot().context
      if (context.suspended || context.scopeKey !== scopeKey) return
      try {
        sessionStorage.setItem(
          storageKey,
          JSON.stringify({
            scopeKey,
            recovery: {
              draft: context.draft,
              command: context.command,
              session: context.session,
              pending: context.pending,
              estimate: context.estimate,
            },
          }),
        )
      } catch {
        /* Server records retain paid work. */
      }
    }
    const subscription = actorRef.subscribe(persist)
    persist()
    return () => subscription.unsubscribe()
  }, [actorRef, scopeKey, storageKey])
  useEffect(() => {
    if (phase !== 'running' || actorRef.getSnapshot().context.scopeKey !== scopeKey) return
    const timer = window.setInterval(() => send({ type: 'REFRESH' }), POLL_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [actorRef, phase, scopeKey, send])
  return { actorRef, snapshot, context: snapshot.context, phase, send }
}
