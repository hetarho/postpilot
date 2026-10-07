import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import type { WritingTestClient, WritingTestPlan } from '@/entities/writing-test'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  writingTestMachine,
  writingTestPhase,
  writingTestScopeKey,
  writingTestPlanShape,
  writingTestCommandShape,
  type WritingTestRecovery,
  type WritingTestOperationEvent,
  type WritingTestOperationContext,
  type WritingTestPhase,
} from './writing-test-machine'
import {
  testFlowMachine,
  writingTestView,
  type WritingTestPresentationEvent,
  type TestPresentationRecovery,
  type WritingTestPresentationContext,
  type WritingTestView,
} from './test-flow-machine'

type Unscoped<E> = E extends { scopeKey: string } ? Omit<E, 'scopeKey'> : never
export type WritingTestAction = Unscoped<WritingTestOperationEvent>
export type WritingTestPresentationAction = Unscoped<WritingTestPresentationEvent>
export interface WritingTestFlowOptions {
  ownerId: string
  seedKey: string
  client: WritingTestClient
  initialDraft?: WritingTestPlan
  testId?: string
  prepareCandidates?: () => void
  /** Browser session storage holds only this owner/entry's resumable work, never an actor snapshot. */
  recoveryStorage?: Pick<Storage, 'getItem' | 'setItem'> | null
}
interface PublicActorRef<C, E> {
  send(event: E): void
  getSnapshot(): { context: C; value: unknown }
}
export interface WritingTestFlowResult {
  actorRef: PublicActorRef<WritingTestOperationContext, WritingTestOperationEvent>
  presentationActorRef: PublicActorRef<WritingTestPresentationContext, WritingTestPresentationEvent>
  snapshot: { context: WritingTestOperationContext; value: unknown }
  context: WritingTestOperationContext
  phase: WritingTestPhase
  presentation: WritingTestPresentationContext
  view: WritingTestView
  send(event: WritingTestAction): void
  sendPresentation(event: WritingTestPresentationAction): void
}
interface StoredFlow {
  scopeKey: string
  operation: WritingTestRecovery
  presentation: TestPresentationRecovery
}
function browserStorage(): Storage | undefined {
  try {
    return globalThis.sessionStorage
  } catch {
    return undefined
  }
}
function restore(
  storage: WritingTestFlowOptions['recoveryStorage'],
  key: string,
): StoredFlow | undefined {
  try {
    const raw = storage?.getItem(key)
    if (!raw) return undefined
    const result: unknown = JSON.parse(raw, (_name, value: unknown) => {
      if (
        value &&
        typeof value === 'object' &&
        '$writingTestBigInt' in value &&
        typeof value.$writingTestBigInt === 'string'
      )
        return BigInt(value.$writingTestBigInt)
      return value
    })
    if (
      !result ||
      typeof result !== 'object' ||
      !('scopeKey' in result) ||
      typeof result.scopeKey !== 'string' ||
      !('operation' in result) ||
      !result.operation ||
      typeof result.operation !== 'object' ||
      !('presentation' in result) ||
      !result.presentation ||
      typeof result.presentation !== 'object'
    )
      return undefined
    const candidate = result as StoredFlow
    const operation = candidate.operation
    if (
      (operation.draft !== undefined && !writingTestPlanShape(operation.draft)) ||
      (operation.command !== undefined && !writingTestCommandShape(operation.command)) ||
      (operation.testId !== undefined && typeof operation.testId !== 'string')
    )
      return undefined
    const presentation = candidate.presentation
    if (
      (presentation.step !== undefined &&
        !['factor', 'candidates', 'material'].includes(presentation.step)) ||
      (presentation.visibleCandidateId !== undefined &&
        typeof presentation.visibleCandidateId !== 'string') ||
      (presentation.reading !== undefined &&
        (typeof presentation.reading !== 'object' ||
          presentation.reading === null ||
          !Object.values(presentation.reading).every(
            (position) =>
              typeof position === 'number' && Number.isFinite(position) && position >= 0,
          )))
    )
      return undefined
    if (
      presentation.choices &&
      (typeof presentation.choices !== 'object' ||
        !['save-setting', 'use-setting', 'adopt-model', 'apply-output'].includes(
          presentation.choices.action,
        ) ||
        typeof presentation.choices.name !== 'string' ||
        typeof presentation.choices.scope !== 'string' ||
        typeof presentation.choices.makeDefault !== 'boolean' ||
        !Array.isArray(presentation.choices.scopeIds) ||
        !presentation.choices.scopeIds.every((id) => typeof id === 'string'))
    )
      return undefined
    return candidate
  } catch {
    return undefined
  }
}
function store(
  storage: WritingTestFlowOptions['recoveryStorage'],
  key: string,
  value: StoredFlow,
): void {
  try {
    storage?.setItem(
      key,
      JSON.stringify(value, (_name, field: unknown) =>
        typeof field === 'bigint' ? { $writingTestBigInt: field.toString() } : field,
      ),
    )
  } catch {
    /* Durable server history remains usable when browser storage is unavailable. */
  }
}

/** Mount inside an owner/seed-keyed host. Resizing never remounts these React actors. */
export function useWritingTestFlow(options: WritingTestFlowOptions): WritingTestFlowResult {
  const scopeKey = writingTestScopeKey(options.ownerId, options.seedKey)
  const storage = options.recoveryStorage === undefined ? browserStorage() : options.recoveryStorage
  const key = `postpilot:writing-test:${scopeKey}`
  const preparationRef = useRef(options.prepareCandidates)
  useLayoutEffect(() => {
    preparationRef.current = options.prepareCandidates
  }, [options.prepareCandidates])
  const [initialRecovery] = useState(() => {
    const candidate = restore(storage, key)
    return candidate?.scopeKey === scopeKey ? candidate : undefined
  })
  const actorRef = useActorRef(writingTestMachine, {
    input: {
      ownerId: options.ownerId,
      seedKey: options.seedKey,
      client: options.client,
      initialDraft: options.initialDraft,
      testId: options.testId,
      recovery: initialRecovery?.operation,
    },
  })
  const snapshot = useSelector(actorRef, (value) => value)
  const phase = writingTestPhase(snapshot)
  const presentationActorRef = useActorRef(testFlowMachine, {
    input: {
      scopeKey,
      operation: snapshot.context,
      phase,
      recovery: initialRecovery?.presentation,
      prepareCandidates: options.prepareCandidates ? () => preparationRef.current?.() : undefined,
    },
  })
  const presentationSnapshot = useSelector(presentationActorRef, (value) => value)
  const send = useCallback(
    (event: WritingTestAction) => {
      if (actorRef.getSnapshot().context.scopeKey !== scopeKey) return
      actorRef.send({ ...event, scopeKey } as WritingTestOperationEvent)
    },
    [actorRef, scopeKey],
  )
  const sendPresentation = useCallback(
    (event: WritingTestPresentationAction) => {
      if (actorRef.getSnapshot().context.scopeKey !== scopeKey) return
      presentationActorRef.send({ ...event, scopeKey } as WritingTestPresentationEvent)
      if (event.type === 'BACK' && actorRef.getSnapshot().matches('quoted'))
        actorRef.send({ type: 'DISMISS_QUOTE', scopeKey })
    },
    [actorRef, presentationActorRef, scopeKey],
  )
  useLayoutEffect(() => {
    const current = actorRef.getSnapshot().context
    if (current.scopeKey !== scopeKey && !current.suspended)
      actorRef.send({ type: 'SUSPEND', scopeKey: current.scopeKey })
  }, [actorRef, scopeKey])
  useEffect(() => {
    const publish = () => {
      const current = actorRef.getSnapshot()
      if (current.context.scopeKey !== scopeKey) return
      presentationActorRef.send({
        type: 'OBSERVE',
        scopeKey,
        operation: current.context,
        phase: writingTestPhase(current),
      })
    }
    const subscription = actorRef.subscribe(publish)
    publish()
    return () => subscription.unsubscribe()
  }, [actorRef, presentationActorRef, scopeKey])
  useEffect(() => {
    const persist = () => {
      const context = actorRef.getSnapshot().context
      if (context.scopeKey !== scopeKey || context.suspended) return
      const view = presentationActorRef.getSnapshot().context
      store(storage, key, {
        scopeKey,
        operation: { draft: context.draft, testId: context.testId, command: context.command },
        presentation: {
          step: view.step,
          reading: view.reading,
          visibleCandidateId: view.visibleCandidateId,
          choices: view.choices,
        },
      })
    }
    const operationSubscription = actorRef.subscribe(persist)
    const presentationSubscription = presentationActorRef.subscribe(persist)
    persist()
    return () => {
      operationSubscription.unsubscribe()
      presentationSubscription.unsubscribe()
    }
  }, [actorRef, presentationActorRef, storage, key, scopeKey])
  useEffect(() => {
    if (phase !== 'running' || snapshot.context.scopeKey !== scopeKey) return
    const interval = window.setInterval(() => send({ type: 'RELOAD' }), POLL_INTERVAL_MS)
    return () => window.clearInterval(interval)
  }, [phase, send, scopeKey, snapshot.context.scopeKey])
  return {
    actorRef,
    presentationActorRef,
    snapshot,
    context: snapshot.context,
    phase,
    presentation: presentationSnapshot.context,
    view: writingTestView(presentationSnapshot),
    send,
    sendPresentation,
  }
}
