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
