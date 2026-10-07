import { createActor, fromPromise, waitFor } from 'xstate'
import { expect, it, vi } from 'vitest'
import {
  authoringMachine,
  authoringStateOf,
  type AuthoringQuote,
  type AuthoringQuoted,
  type AuthoringWork,
  type AuthoringResult,
} from './authoring-machine'
import type { AuthoringSession } from '@/entities/ai-authoring'

const scope = { ownerId: 'alice', kind: 'post-guideline' as const }
const session: AuthoringSession = {
  id: 'session',
  kind: scope.kind,
  revision: 3,
  phase: 'editing',
  targetId: '',
  targetVersion: '',
  candidates: [],
  selected: {
    id: 'a',
    name: '방향',
    description: '설명',
    body: '실제로 경험한 것만 써요.',
    titleArea: '',
  },
  turns: [],
  activeJobId: '',
  failureReason: '',
  pendingRequest: '',
}
it('invokes one estimate and one explicitly confirmed command, freezing operation identity and ignoring duplicate actions', async () => {
  const estimates = vi.fn(async (input: AuthoringQuote) => ({
    scopeKey: input.scopeKey,
    operation: input.operation,
    estimate: { free: true },
  }))
  const execute = vi.fn(async (input: AuthoringWork) => ({
    scopeKey: input.scopeKey,
    operation: input.operation,
    session: { ...session, revision: 4 },
    clearText: true,
  }))
  const actor = createActor(
    authoringMachine.provide({
      actors: {
        estimate: fromPromise<AuthoringQuoted, AuthoringQuote>(({ input }) => estimates(input)),
        execute: fromPromise<AuthoringResult, AuthoringWork>(({ input }) => execute(input)),
      },
    }),
    { input: scope },
  ).start()
  const scopeKey = actor.getSnapshot().context.scopeKey
  actor.send({ type: 'hydrate', scopeKey, session })
  const command = {
    mode: 'refine' as const,
    prompt: '더 간결하게',
    writeModel: { providerId: 'p', modelId: 'writer' },
    sessionId: session.id,
    expectedRevision: 3,
    createRequestId: 'create',
    requestId: 'request',
  }
  actor.send({ type: 'quote', scopeKey, command })
  actor.send({ type: 'quote', scopeKey, command })
  await waitFor(actor, (s) => s.matches('confirming'))
  expect(execute).not.toHaveBeenCalled()
  actor.send({ type: 'begin', scopeKey, phase: 'starting' })
  actor.send({ type: 'begin', scopeKey, phase: 'starting' })
  await waitFor(actor, (s) => s.matches('editing'))
  expect(estimates).toHaveBeenCalledTimes(1)
  expect(execute).toHaveBeenCalledTimes(1)
  expect(execute.mock.calls[0][0].command).toMatchObject({
    requestId: 'request',
    confirmed: true,
    writeModel: { providerId: 'p', modelId: 'writer' },
  })
  actor.stop()
})

it('restores durable running state without invoking a model command and rejects foreign or older server results', () => {
  const execute = vi.fn()
  const actor = createActor(
    authoringMachine.provide({
      actors: { execute: fromPromise<AuthoringResult, AuthoringWork>(execute) },
    }),
    { input: scope },
  ).start()
  const scopeKey = actor.getSnapshot().context.scopeKey
  actor.send({
    type: 'hydrate',
    scopeKey,
    session: { ...session, phase: 'refining', activeJobId: 'job', pendingRequest: '진행 중 요청' },
  })
  expect(actor.getSnapshot().matches('active')).toBe(true)
  actor.send({ type: 'hydrate', scopeKey: 'foreign', session })
  actor.send({ type: 'hydrate', scopeKey, session: { ...session, revision: 2 } })
  expect(authoringStateOf(actor.getSnapshot()).text).toBe('진행 중 요청')
  expect(execute).not.toHaveBeenCalled()
  actor.stop()
})

it('admits one manual CAS, retains invalid source separately from preview, and ignores late/foreign responses', async () => {
  let finish!: (value: AuthoringResult) => void
  const execute = vi.fn((input: AuthoringWork) => {
    expect(input.retry?.type).toBe('patch')
    return new Promise<AuthoringResult>((resolve) => {
      finish = resolve
    })
  })
  const actor = createActor(
    authoringMachine.provide({
      actors: {
        execute: fromPromise<AuthoringResult, AuthoringWork>(({ input }) => execute(input)),
      },
    }),
    { input: scope },
  ).start()
  const scopeKey = actor.getSnapshot().context.scopeKey
  actor.send({ type: 'hydrate', scopeKey, session })
  const source = { ...session.selected!, body: '<unfinished' }
  actor.send({ type: 'source', scopeKey, source })
  expect(authoringStateOf(actor.getSnapshot()).sourceDirty).toBe(true)
  actor.send({
    type: 'begin',
    scopeKey,
    phase: 'saving',
    retry: { type: 'save', sessionId: session.id, revision: 3, makeDefault: false },
  })
  expect(execute).not.toHaveBeenCalled()
  const retry = {
    type: 'patch' as const,
    sessionId: session.id,
    revision: 3,
    operationKey: 'one-patch',
    source,
  }
  actor.send({ type: 'begin', scopeKey, phase: 'patching', retry })
  actor.send({ type: 'begin', scopeKey, phase: 'patching', retry })
  expect(execute).toHaveBeenCalledTimes(1)
  const input = execute.mock.calls[0][0]
  actor.send({
    type: 'response',
    scopeKey: 'foreign',
    operation: input.operation,
    session: { ...session, revision: 100 },
  })
  actor.send({
    type: 'response',
    scopeKey,
    operation: input.operation - 1,
    session: { ...session, revision: 100 },
  })
  expect(actor.getSnapshot().matches('patching')).toBe(true)
  finish({
    scopeKey,
    operation: input.operation,
    session: {
      ...session,
      revision: 4,
      workingSource: source,
      draftState: 'invalid',
      hasUnpublishedChanges: true,
    },
  })
  await waitFor(actor, (s) => s.matches('editing'))
  const state = authoringStateOf(actor.getSnapshot())
  expect(state.directSource?.body).toBe('<unfinished')
  expect(state.session?.selected?.body).toBe(session.selected!.body)
  expect(state.sourceDirty).toBe(false)
  actor.send({
    type: 'begin',
    scopeKey,
    phase: 'saving',
    retry: { type: 'save', sessionId: session.id, revision: 4, makeDefault: false },
  })
  expect(execute).toHaveBeenCalledTimes(1)
  actor.send({ type: 'hydrate', scopeKey, session })
  expect(authoringStateOf(actor.getSnapshot()).session?.revision).toBe(4)
  actor.stop()
})

it('keeps the direct input baseline and reports a conflict when a newer read arrives', () => {
  const actor = createActor(authoringMachine, { input: scope }).start()
  const scopeKey = actor.getSnapshot().context.scopeKey
  actor.send({ type: 'hydrate', scopeKey, session })
  const source = { ...session.selected!, body: 'My unfinished local edit' }
  actor.send({ type: 'source', scopeKey, source })
  actor.send({
    type: 'hydrate',
    scopeKey,
    session: {
      ...session,
      revision: 4,
      workingSource: { ...source, body: 'Other tab edit' },
      selected: { ...source, body: 'Other tab edit' },
    },
  })
  let state = authoringStateOf(actor.getSnapshot())
  expect(state.session?.revision).toBe(4)
  expect(state.directSource?.body).toBe(source.body)
  expect(state.sourceRevision).toBe(3)
  expect(state.failure?.reason).toBe('AUTHORING_REVISION_CONFLICT')
  actor.send({ type: 'source', scopeKey, source: { ...source, name: 'Keep my new name' } })
  state = authoringStateOf(actor.getSnapshot())
  expect(state.sourceRevision).toBe(3)
  expect(state.directSource?.name).toBe('Keep my new name')
  actor.stop()
})
