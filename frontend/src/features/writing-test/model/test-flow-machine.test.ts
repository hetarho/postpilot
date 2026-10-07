import { createActor, getInitialSnapshot } from 'xstate'
import { create } from '@bufbuild/protobuf'
import { expect, it, vi } from 'vitest'
import { PostContentSchema } from '@/shared/api'
import type { WritingTestClient, WritingTest } from '@/entities/writing-test'
import { testFlowMachine, writingTestView } from './test-flow-machine'
import { writingTestMachine, writingTestScopeKey } from './writing-test-machine'

const scopeKey = writingTestScopeKey('alice', 'entry')
function operation() {
  return getInitialSnapshot(writingTestMachine, {
    ownerId: 'alice',
    seedKey: 'entry',
    client: {} as WritingTestClient,
  })
}
function durable(): WritingTest {
  return {
    id: 'test',
    revision: 2,
    factor: 'model',
    modelStage: 'write',
    count: 2,
    status: 'review',
    sourcePostSlug: '',
    jobId: '',
    candidates: ['a', 'b'].map((id) => ({
      id,
      status: 'succeeded',
      displayLabel: id,
      output: create(PostContentSchema, { title: id }),
    })),
    matches: [
      {
        id: 'match',
        round: 1,
        index: 0,
        leftCandidateId: 'a',
        rightCandidateId: 'b',
        winnerCandidateId: '',
      },
    ],
    winnerCandidateId: '',
    publications: [],
    revealed: false,
    createdAt: '',
    updatedAt: '',
    contentExpiresAt: '',
    fictional: false,
    confirmedCredits: 0,
    reservedCredits: 0,
    targetLanguage: 'ko',
  }
}
it('Back retains presentation state and only explicit candidate preparation invokes its port', () => {
  const preparation = vi.fn()
  const op = operation()
  const ref = createActor(testFlowMachine, {
    input: { scopeKey, operation: op.context, phase: 'editing', prepareCandidates: preparation },
  }).start()
  expect(preparation).not.toHaveBeenCalled()
  ref.send({ type: 'NEXT', scopeKey })
  expect(writingTestView(ref.getSnapshot())).toBe('candidates')
  ref.send({ type: 'PREPARE_CANDIDATES', scopeKey: 'other' })
  expect(preparation).not.toHaveBeenCalled()
  ref.send({ type: 'PREPARE_CANDIDATES', scopeKey })
  expect(preparation).toHaveBeenCalledTimes(1)
  ref.send({ type: 'NEXT', scopeKey })
  ref.send({ type: 'BACK', scopeKey })
  expect(writingTestView(ref.getSnapshot())).toBe('candidates')
  ref.send({ type: 'BACK', scopeKey })
  expect(writingTestView(ref.getSnapshot())).toBe('factor')
  expect(preparation).toHaveBeenCalledTimes(1)
  ref.stop()
})
it('records full-post reading positions and unsent publication choices through Back and later server rounds', () => {
  const op = { ...operation().context, test: durable(), testId: 'test', operation: 1 }
  const ref = createActor(testFlowMachine, {
    input: { scopeKey, operation: op, phase: 'match' },
  }).start()
  ref.send({ type: 'READING', scopeKey, candidateId: 'a', position: 423 })
  ref.send({ type: 'READING', scopeKey, candidateId: 'b', position: 117 })
  ref.send({ type: 'READING', scopeKey, candidateId: 'a', position: -1 })
  ref.send({ type: 'READING', scopeKey, candidateId: 'other', position: 200 })
  ref.send({ type: 'SHOW_CANDIDATE', scopeKey, candidateId: 'b' })
  const champion = {
    ...op,
    operation: 2,
    test: { ...durable(), revision: 3, status: 'completed' as const, winnerCandidateId: 'a' },
  }
  ref.send({ type: 'OBSERVE', scopeKey, operation: champion, phase: 'champion' })
  ref.send({ type: 'OPEN_PUBLICATION', scopeKey })
  const choices = {
    action: 'adopt-model' as const,
    name: 'My chosen model',
    makeDefault: true,
    scope: '',
    scopeIds: [],
  }
  ref.send({ type: 'PUBLICATION_CHOICES', scopeKey, choices })
  choices.name = 'External mutation'
  ref.send({ type: 'BACK', scopeKey })
  expect(writingTestView(ref.getSnapshot())).toBe('champion')
  ref.send({ type: 'OPEN_PUBLICATION', scopeKey })
  expect(ref.getSnapshot().context.choices.name).toBe('My chosen model')
  expect(ref.getSnapshot().context.reading).toEqual({ a: 423, b: 117 })
  expect(ref.getSnapshot().context.visibleCandidateId).toBe('b')
  ref.stop()
})
it('ignores stale owner/test/revision/operation observations and never exposes local champion progression', () => {
  const op = { ...operation().context, test: durable(), testId: 'test', operation: 3 }
  const ref = createActor(testFlowMachine, {
    input: { scopeKey, operation: op, phase: 'match' },
  }).start()
  for (const [incoming, incomingScope] of [
    [{ ...op, operation: 2 }, scopeKey],
    [{ ...op, test: { ...durable(), revision: 1 } }, scopeKey],
    [{ ...op, test: { ...durable(), id: 'other' } }, scopeKey],
    [op, 'foreign'],
  ] as const)
    ref.send({ type: 'OBSERVE', scopeKey: incomingScope, operation: incoming, phase: 'champion' })
  expect(writingTestView(ref.getSnapshot())).toBe('compare')
  expect(ref.getSnapshot().context.operation.test?.winnerCandidateId).toBe('')
  ref.stop()
})
it('keeps both complete posts and their reading positions visible while a server decision is pending', () => {
  const op = { ...operation().context, test: durable(), testId: 'test', operation: 1 }
  const ref = createActor(testFlowMachine, {
    input: { scopeKey, operation: op, phase: 'match' },
  }).start()
  ref.send({ type: 'READING', scopeKey, candidateId: 'a', position: 432 })
  ref.send({ type: 'OBSERVE', scopeKey, operation: { ...op, operation: 2 }, phase: 'deciding' })
  expect(writingTestView(ref.getSnapshot())).toBe('compare')
  expect(ref.getSnapshot().context.reading.a).toBe(432)
  expect(ref.getSnapshot().context.operation.test?.matches[0]?.winnerCandidateId).toBe('')
  ref.stop()
})
it('a confirmed source application does not hide the separate explicit model adoption choices', () => {
  const op = {
    ...operation().context,
    test: { ...durable(), status: 'completed' as const, winnerCandidateId: 'a' },
    testId: 'test',
    operation: 3,
  }
  const ref = createActor(testFlowMachine, {
    input: { scopeKey, operation: op, phase: 'published' },
  }).start()
  expect(writingTestView(ref.getSnapshot())).toBe('champion')
  ref.send({ type: 'OPEN_PUBLICATION', scopeKey })
  expect(writingTestView(ref.getSnapshot())).toBe('publication')
  ref.stop()
})
it('a source conflict opens publication choices explicitly and retains all unsent naming and scope selections', () => {
  const op = {
    ...operation().context,
    test: { ...durable(), status: 'completed' as const, winnerCandidateId: 'a' },
    operation: 4,
  }
  const ref = createActor(testFlowMachine, {
    input: { scopeKey, operation: op, phase: 'conflict' },
  }).start()
  expect(writingTestView(ref.getSnapshot())).toBe('recovery')
  ref.send({ type: 'OPEN_PUBLICATION', scopeKey })
  expect(writingTestView(ref.getSnapshot())).toBe('publication')
  ref.send({
    type: 'PUBLICATION_CHOICES',
    scopeKey,
    choices: {
      action: 'save-setting',
      name: 'Explicit new copy',
      makeDefault: false,
      scope: 'templates',
      scopeIds: ['template'],
    },
  })
  expect(ref.getSnapshot().context.choices.scopeIds).toEqual(['template'])
  ref.stop()
})
