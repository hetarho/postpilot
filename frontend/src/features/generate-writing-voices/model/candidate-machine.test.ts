import { expect, it } from 'vitest'
import type { WritingVoiceCandidateBatch } from '@/entities/voice-candidate'
import {
  candidateBusy,
  candidateTransition,
  initialCandidateState,
  type CandidateState,
  type CandidateEvent,
} from './candidate-machine'

const model = { providerId: 'p', modelId: 'writer' }
const batch = (jobId = 'first'): WritingVoiceCandidateBatch => ({
  jobId,
  resultJobId: jobId,
  candidates: Array.from({ length: 8 }, (_, n) => ({
    id: `${n}`,
    name: `style ${n}`,
    description: 'a clear style',
    sample: 'A fictional example.',
  })),
})
const send = (state: CandidateState, event: Omit<CandidateEvent, 'ownerId'>) =>
  candidateTransition(state, { ...event, ownerId: 'alice' } as CandidateEvent)
const ready = () =>
  send(initialCandidateState('alice'), { type: 'latest', batch: batch() } as Omit<
    CandidateEvent,
    'ownerId'
  >)

it('requires explicit confirmation and ignores duplicate and obsolete operation events', () => {
  let state = initialCandidateState('alice')
  expect(send(state, { type: 'start' })).toBe(state)
  const source = { ...model }
  state = candidateTransition(state, { type: 'confirm', ownerId: 'alice', model: source })
  source.modelId = 'changed-later'
  expect(state.frozenModel?.modelId).toBe('writer')
  state = send(state, { type: 'start' })
  expect(candidateBusy(state)).toBe(true)
  expect(send(state, { type: 'start' })).toBe(state)
  expect(
    candidateTransition(state, {
      type: 'started',
      ownerId: 'alice',
      operation: state.operation + 1,
      jobId: 'obsolete',
    }),
  ).toBe(state)
  expect(
    candidateTransition(state, {
      type: 'started',
      ownerId: 'bob',
      operation: state.operation,
      jobId: 'foreign',
    }),
  ).toBe(state)
  state = candidateTransition(state, {
    type: 'started',
    ownerId: 'alice',
    operation: state.operation,
    jobId: 'running',
  })
  expect(state.phase).toBe('running')
})

it('retains the prior eight styles through reroll failure and permits prior-result adoption', () => {
  let state = ready()
  state = candidateTransition(state, { type: 'select', ownerId: 'alice', candidateId: '2' })
  state = candidateTransition(state, { type: 'confirm', ownerId: 'alice', model })
  state = send(state, { type: 'start' })
  state = candidateTransition(state, {
    type: 'started',
    ownerId: 'alice',
    operation: state.operation,
    jobId: 'second',
  })
  state = candidateTransition(state, {
    type: 'terminal',
    ownerId: 'alice',
    jobId: 'second',
    status: 'failed',
  })
  expect(state.phase).toBe('failed')
  expect(state.candidates).toHaveLength(8)
  expect(state.resultJobId).toBe('first')
  expect(send(state, { type: 'adopt' }).phase).toBe('adopting')
})

it('clears a selected style whenever the result batch changes even if ids repeat', () => {
  let state = ready()
  state = candidateTransition(state, { type: 'select', ownerId: 'alice', candidateId: '2' })
  state = candidateTransition(state, { type: 'latest', ownerId: 'alice', batch: batch('second') })
  expect(state.selectedId).toBe('')
  expect(send(state, { type: 'adopt' })).toBe(state)
})

it('requires a cancel dialog and stays busy until the durable terminal status', () => {
  let state = candidateTransition(initialCandidateState('alice'), {
    type: 'latest',
    ownerId: 'alice',
    batch: { ...batch(), jobId: 'second' },
  })
  expect(send(state, { type: 'cancel' })).toBe(state)
  state = send(state, { type: 'open-cancel' })
  state = send(state, { type: 'cancel' })
  expect(state.phase).toBe('cancelling')
  expect(send(state, { type: 'cancel' })).toBe(state)
  state = candidateTransition(state, {
    type: 'cancelled',
    ownerId: 'alice',
    operation: state.operation,
  })
  expect(candidateBusy(state)).toBe(true)
  state = candidateTransition(state, {
    type: 'terminal',
    ownerId: 'alice',
    jobId: 'second',
    status: 'cancelled',
  })
  expect(state.phase).toBe('ready')
  state = candidateTransition(state, {
    type: 'latest',
    ownerId: 'alice',
    batch: { ...batch(), jobId: 'second' },
  })
  expect(candidateBusy(state)).toBe(false)
})

it('handles a done job that races cancellation before its result arrives', () => {
  let state = candidateTransition(initialCandidateState('alice'), {
    type: 'latest',
    ownerId: 'alice',
    batch: { ...batch(), jobId: 'second' },
  })
  state = candidateTransition(state, {
    type: 'terminal',
    ownerId: 'alice',
    jobId: 'second',
    status: 'done',
  })
  state = send(state, { type: 'open-cancel' })
  state = send(state, { type: 'cancel' })
  state = candidateTransition(state, { type: 'latest', ownerId: 'alice', batch: batch('second') })
  expect(state.phase).toBe('ready')
})

it('rejects partial batches and obsolete job completion', () => {
  const state = ready()
  const partial = { ...batch('second'), candidates: batch().candidates.slice(0, 7) }
  const next = candidateTransition(state, { type: 'latest', ownerId: 'alice', batch: partial })
  expect(next.resultJobId).toBe('first')
  expect(next.candidates).toHaveLength(8)
  expect(
    candidateTransition(next, {
      type: 'terminal',
      ownerId: 'alice',
      jobId: 'obsolete',
      status: 'done',
    }),
  ).toBe(next)
})
