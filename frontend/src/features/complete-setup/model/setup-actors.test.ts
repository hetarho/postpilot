import { createActor } from 'xstate'
import { expect, it } from 'vitest'
import { setupMachine, setupStateOf, emptySetupProgress } from './setup-machine'

it('runs setup as a real scoped actor and locks navigation until its confirmed operation settles', () => {
  const actor = createActor(setupMachine, { input: { ownerId: 'alice' } }).start()
  actor.send({
    type: 'hydrate',
    ownerId: 'bob',
    missing: ['voice'],
    progress: emptySetupProgress(),
  })
  expect(actor.getSnapshot().matches('checking')).toBe(true)
  actor.send({
    type: 'hydrate',
    ownerId: 'alice',
    missing: ['voice', 'post-template'],
    progress: emptySetupProgress(),
  })
  actor.send({ type: 'next', ownerId: 'alice', step: 'welcome', confirmed: true })
  actor.send({ type: 'begin', ownerId: 'alice', step: 'voice' })
  const reserved = actor.getSnapshot()
  actor.send({ type: 'begin', ownerId: 'alice', step: 'voice' })
  actor.send({ type: 'skip', ownerId: 'alice', step: 'voice' })
  actor.send({ type: 'back', ownerId: 'alice' })
  expect(actor.getSnapshot()).toBe(reserved)
  const operation = reserved.context.operation
  actor.send({
    type: 'success',
    ownerId: 'alice',
    step: 'voice',
    operation: operation - 1,
    complete: true,
  })
  expect(actor.getSnapshot().matches('saving')).toBe(true)
  actor.send({ type: 'running', ownerId: 'alice', step: 'voice', operation })
  actor.send({ type: 'success', ownerId: 'alice', step: 'voice', operation, complete: true })
  expect(setupStateOf(actor.getSnapshot())).toMatchObject({
    phase: 'editing',
    step: 'post-template',
    resolved: ['voice'],
  })
  actor.stop()
})

it('restores only setup metadata and preserves a completed destination against repeated finish events', () => {
  const actor = createActor(setupMachine, { input: { ownerId: 'alice' } }).start()
  actor.send({
    type: 'hydrate',
    ownerId: 'alice',
    missing: ['voice'],
    progress: { ...emptySetupProgress(), skipped: ['voice'] },
  })
  expect(setupStateOf(actor.getSnapshot()).step).toBe('ready')
  actor.send({ type: 'finish', ownerId: 'alice', target: '/clips/new' })
  actor.send({ type: 'finish', ownerId: 'alice', target: '/posts/new' })
  expect(setupStateOf(actor.getSnapshot()).target).toBe('/clips/new')
  actor.stop()
})
