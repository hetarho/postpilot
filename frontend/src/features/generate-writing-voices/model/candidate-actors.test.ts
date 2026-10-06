import { createActor } from 'xstate'
import { expect, it } from 'vitest'
import { candidateMachine, candidateStateOf } from './candidate-machine'

const ownerId = 'alice'
const batch = {
  jobId: 'old',
  resultJobId: 'old',
  candidates: Array.from({ length: 8 }, (_, i) => ({
    id: String(i),
    name: String(i),
    description: '느낌',
    sample: '가상의 예시',
  })),
}
it('uses nested cancellation confirmation and preserves it while latest results are read', () => {
  const actor = createActor(candidateMachine, { input: { ownerId } }).start()
  actor.send({ type: 'latest', ownerId, batch })
  actor.send({ type: 'confirm', ownerId, model: { providerId: 'p', modelId: 'writer' } })
  actor.send({ type: 'start', ownerId })
  const operation = actor.getSnapshot().context.operation
  actor.send({ type: 'started', ownerId, operation, jobId: 'new' })
  actor.send({ type: 'cancel', ownerId })
  expect(actor.getSnapshot().matches({ running: 'active' })).toBe(true)
  actor.send({ type: 'open-cancel', ownerId })
  actor.send({ type: 'latest', ownerId, batch: { ...batch, jobId: 'new' } })
  expect(actor.getSnapshot().matches({ running: 'confirming' })).toBe(true)
  actor.send({ type: 'cancel', ownerId })
  expect(actor.getSnapshot().matches('cancelling')).toBe(true)
  actor.send({ type: 'terminal', ownerId, jobId: 'foreign', status: 'cancelled' })
  expect(actor.getSnapshot().matches('cancelling')).toBe(true)
  actor.send({ type: 'terminal', ownerId, jobId: 'new', status: 'cancelled' })
  expect(candidateStateOf(actor.getSnapshot())).toMatchObject({
    phase: 'ready',
    candidates: batch.candidates,
  })
  actor.stop()
})

it('freezes a concrete ref, guards adoption and clears selection for a different result batch', () => {
  const actor = createActor(candidateMachine, { input: { ownerId } }).start()
  actor.send({ type: 'latest', ownerId, batch })
  actor.send({ type: 'select', ownerId, candidateId: '1' })
  actor.send({
    type: 'latest',
    ownerId,
    batch: { ...batch, jobId: 'second', resultJobId: 'second' },
  })
  expect(actor.getSnapshot().context.selectedId).toBe('')
  actor.send({ type: 'adopt', ownerId })
  expect(actor.getSnapshot().matches('ready')).toBe(true)
  const model = { providerId: 'p', modelId: 'writer' }
  actor.send({ type: 'confirm', ownerId, model })
  model.modelId = 'changed'
  actor.send({ type: 'start', ownerId })
  actor.send({ type: 'start', ownerId })
  expect(actor.getSnapshot().context.operation).toBe(1)
  expect(actor.getSnapshot().context.frozenModel?.modelId).toBe('writer')
  actor.stop()
})
