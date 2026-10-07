import { createActor, waitFor, type ActorRefFrom } from 'xstate'
import { create } from '@bufbuild/protobuf'
import { expect, it, vi } from 'vitest'
import { BlockType, PostContentSchema } from '@/shared/api'
import type {
  WritingTest,
  WritingTestClient,
  WritingTestPlan,
  TestCount,
  TestDecisionInput,
} from '@/entities/writing-test'
import {
  POST_TARGET_LENGTH_MIN,
  POST_TARGET_LENGTH_MAX,
  POST_TAG_COUNT_MIN,
  POST_TAG_COUNT_MAX,
} from '@/entities/post'
import {
  writingTestMachine,
  currentWritingTestMatch,
  writingTestDraftProblem,
  writingTestScopeKey,
  type FrozenTestCommand,
  type WritingTestOperationEvent,
} from './writing-test-machine'

const scopeKey = writingTestScopeKey('alice', 'draft')
const future = '2099-01-01T00:00:00Z'
const quote = { free: false, credits: 160, quoteKey: 'quote', expiresAt: future }
function plan(count: TestCount = 2): WritingTestPlan {
  return {
    factor: 'model',
    modelStage: 'write',
    count,
    entrants: Array.from({ length: count }, (_, index) => ({
      type: 'model',
      model: { providerId: 'p', modelId: `model-${index}` },
    })),
    context: {
      sourcePostSlug: '',
      expectedInputRevision: 0n,
      expectedContentRevision: 0n,
      material: {
        text: 'Read my own factual material.',
        fictional: false,
        attachmentIds: [],
        templateAnswers: [],
      },
      writeModel: { providerId: 'p', modelId: 'writer' },
      observeModel: { providerId: 'p', modelId: 'observer' },
      voiceId: '',
      templateId: '',
      guidelineSlotId: '',
      targetLanguage: 'en',
      targetLength: 1000,
      tagCount: 3,
      useMemory: false,
      qualityRules: [],
    },
  }
}
function test(count: TestCount = 2, changes: Partial<WritingTest> = {}): WritingTest {
  return {
    id: 'test',
    revision: 1,
    factor: 'model',
    modelStage: 'write',
    count,
    status: 'review',
    sourcePostSlug: '',
    jobId: '',
    candidates: Array.from({ length: count }, (_, index) => ({
      id: `candidate-${index}`,
      status: 'succeeded',
      displayLabel: `Draft ${index + 1}`,
      output: create(PostContentSchema, {
        title: `Actual full post ${index}`,
        blocks: [{ type: BlockType.TEXT, content: `Long preserved output ${index}` }],
        tags: ['test'],
      }),
    })),
    matches: [
      {
        id: 'match-0',
        round: 1,
        index: 0,
        leftCandidateId: 'candidate-0',
        rightCandidateId: 'candidate-1',
        winnerCandidateId: changes.status === 'completed' ? (changes.winnerCandidateId ?? '') : '',
      },
    ],
    winnerCandidateId: '',
    publications: [],
    revealed: false,
    createdAt: '',
    updatedAt: '',
    contentExpiresAt: future,
    fictional: false,
    confirmedCredits: 0,
    reservedCredits: 160,
    targetLanguage: 'en',
    ...changes,
  }
}
function client(initial = test()): WritingTestClient {
  return {
    estimate: vi.fn(async () => quote),
    estimateRetry: vi.fn(async () => quote),
    start: vi.fn(async () => initial),
    get: vi.fn(async () => initial),
    list: vi.fn(async () => ({ tests: [], nextPageToken: '' })),
    retry: vi.fn(async () => initial),
    decide: vi.fn(async () => initial),
    cancel: vi.fn(async () => initial),
    saveWinner: vi.fn(),
    applyOutput: vi.fn(),
  }
}
function actor(
  api: WritingTestClient,
  options: { testId?: string; recovery?: { command?: FrozenTestCommand; testId?: string } } = {},
): ActorRefFrom<typeof writingTestMachine> {
  let key = 0
  return createActor(writingTestMachine, {
    input: {
      ownerId: 'alice',
      seedKey: 'draft',
      client: api,
      initialDraft: plan(),
      requestKey: () => `key-${++key}`,
      ...options,
    },
  }).start()
}
function send(
  ref: ReturnType<typeof actor>,
  event: Omit<
    Extract<
      WritingTestOperationEvent,
      { type: 'ESTIMATE' | 'CONFIRM' | 'DISMISS_QUOTE' | 'RELOAD' | 'RETRY_OPERATION' | 'SUSPEND' }
    >,
    'scopeKey'
  >,
) {
  ref.send({ ...event, scopeKey })
}
async function generate(ref: ReturnType<typeof actor>) {
  send(ref, { type: 'ESTIMATE' })
  await waitFor(ref, (s) => s.matches('quoted'))
  send(ref, { type: 'CONFIRM' })
}
it('mount, Back-equivalent quote dismissal and draft selection make no mutation; explicit estimate/confirm run once with frozen keys and plan', async () => {
  const api = client()
  const ref = actor(api)
  expect(api.get).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  const draft = plan()
  ref.send({ type: 'DRAFT', scopeKey, draft })
  send(ref, { type: 'ESTIMATE' })
  send(ref, { type: 'ESTIMATE' })
  draft.context.material.text = 'Mutated external object'
  await waitFor(ref, (s) => s.matches('quoted'))
  expect(api.start).not.toHaveBeenCalled()
  send(ref, { type: 'DISMISS_QUOTE' })
  expect(ref.getSnapshot().context.draft?.context.material.text).toBe(
    'Read my own factual material.',
  )
  await generate(ref)
  send(ref, { type: 'CONFIRM' })
  await waitFor(ref, (s) => s.matches('match'))
  expect(api.estimate).toHaveBeenCalledTimes(2)
  expect(api.start).toHaveBeenCalledTimes(1)
  expect(vi.mocked(api.start).mock.calls[0]![0]).toMatchObject({
    requestKey: 'key-1',
    quoteKey: 'quote',
    plan: { context: { material: { text: 'Read my own factual material.' } } },
  })
  ref.stop()
})
it.each([2, 4, 8, 16] as const)(
  'keeps %i successful full posts and admits exactly N-1 server-current binary decisions with no provider operation',
  async (count) => {
    let durable = test(count)
    const originalOutputs = durable.candidates.map((c) => c.output)
    let ids = durable.candidates.map((c) => c.id)
    let round = 1
    let index = 0
    const winners: string[] = []
    const api = client(durable)
    vi.mocked(api.decide).mockImplementation(async (request: TestDecisionInput) => {
      const match = durable.matches.at(-1)!
      expect(request).toMatchObject({
        testId: durable.id,
        expectedRevision: durable.revision,
        matchId: match.id,
      })
      match.winnerCandidateId = request.winnerCandidateId
      winners.push(request.winnerCandidateId)
      index += 2
      if (index === ids.length) {
        ids = winners.splice(0)
        round++
        index = 0
      }
      durable = { ...durable, revision: durable.revision + 1, matches: [...durable.matches] }
      if (ids.length === 1)
        durable = { ...durable, status: 'completed', winnerCandidateId: ids[0]!, revealed: true }
      else
        durable.matches.push({
          id: `match-${durable.matches.length}`,
          round,
          index: index / 2,
          leftCandidateId: ids[index]!,
          rightCandidateId: ids[index + 1]!,
          winnerCandidateId: '',
        })
      return durable
    })
    const ref = actor(api)
    ref.send({ type: 'DRAFT', scopeKey, draft: plan(count) })
    await generate(ref)
    await waitFor(ref, (s) => s.matches('match'))
    for (let decision = 0; decision < count - 1; decision++) {
      const state = ref.getSnapshot().context
      const match = currentWritingTestMatch(state.test)!
      const event = {
        type: 'VOTE' as const,
        scopeKey,
        testId: state.test!.id,
        expectedRevision: state.test!.revision,
        matchId: match.id,
        winnerCandidateId: match.leftCandidateId,
      }
      ref.send(event)
      ref.send(event)
      await waitFor(ref, (s) => s.matches(decision === count - 2 ? 'champion' : 'match'))
      ref.send(event)
    }
    expect(api.decide).toHaveBeenCalledTimes(count - 1)
    expect(api.start).toHaveBeenCalledTimes(1)
    expect(api.estimate).toHaveBeenCalledTimes(1)
    expect(api.retry).not.toHaveBeenCalled()
    expect(api.get).not.toHaveBeenCalled()
    expect(api.saveWinner).not.toHaveBeenCalled()
    expect(ref.getSnapshot().context.test?.candidates.map((c) => c.output)).toEqual(originalOutputs)
    expect(ref.getSnapshot().context.test?.winnerCandidateId).toBe(durable.winnerCandidateId)
    ref.stop()
  },
)
it('refuses duplicate/incomplete/factor-inconsistent entrants and observer tests without real attachments before a quote', () => {
  const draft = plan(4)
  draft.entrants.pop()
  expect(writingTestDraftProblem(draft)?.reason).toBe('WRITING_TEST_COUNT_INVALID')
  const duplicate = plan()
  duplicate.entrants[1] = structuredClone(duplicate.entrants[0]!)
  expect(writingTestDraftProblem(duplicate)?.reason).toBe('WRITING_TEST_ENTRANTS_DUPLICATE')
  const observe = { ...plan(), modelStage: 'observe' as const }
  expect(writingTestDraftProblem(observe)?.reason).toBe('WRITING_TEST_MATERIAL_INVALID')
  const commonAttachments = plan()
  commonAttachments.context.material.attachmentIds = ['owned-image']
  commonAttachments.context.observeModel = undefined
  expect(writingTestDraftProblem(commonAttachments)?.reason).toBe('WRITING_TEST_ENTRANT_INVALID')
  const variedWriter = plan()
  variedWriter.context.writeModel = undefined
  expect(writingTestDraftProblem(variedWriter)).toBeUndefined()
  const api = client()
  const ref = actor(api)
  ref.send({ type: 'DRAFT', scopeKey, draft: observe })
  send(ref, { type: 'ESTIMATE' })
  expect(api.estimate).not.toHaveBeenCalled()
  expect(ref.getSnapshot().context.failure?.reason).toBe('WRITING_TEST_MATERIAL_INVALID')
  ref.stop()
})
it('enforces the all-success barrier and rejects noncurrent matches, foreign owners, stale revisions and impossible winners', async () => {
  const partial = test(4)
  partial.candidates[3] = { ...partial.candidates[3]!, status: 'failed', output: undefined }
  expect(currentWritingTestMatch(partial)).toBeUndefined()
  const api = client(partial)
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('failed'))
  ref.send({
    type: 'VOTE',
    scopeKey,
    testId: 'test',
    expectedRevision: 1,
    matchId: 'match-0',
    winnerCandidateId: 'candidate-0',
  })
  expect(api.decide).not.toHaveBeenCalled()
  ref.stop()
  const validAPI = client()
  const validRef = actor(validAPI, { testId: 'test' })
  await waitFor(validRef, (s) => s.matches('match'))
  const event = {
    type: 'VOTE' as const,
    scopeKey,
    testId: 'test',
    expectedRevision: 1,
    matchId: 'match-0',
    winnerCandidateId: 'candidate-0',
  }
  for (const invalid of [
    { ...event, scopeKey: 'other' },
    { ...event, expectedRevision: 0 },
    { ...event, testId: 'other' },
    { ...event, matchId: 'future-round' },
    { ...event, winnerCandidateId: 'candidate-3' },
  ])
    validRef.send(invalid)
  expect(validAPI.decide).not.toHaveBeenCalled()
  validRef.stop()
})
it('quotes and retries only failed candidates while retaining successful output and frozen material', async () => {
  const partial = test(4, { status: 'partial' })
  partial.candidates[3] = { ...partial.candidates[3]!, status: 'failed', output: undefined }
  const api = client(partial)
  vi.mocked(api.retry).mockResolvedValue({ ...partial, status: 'running', revision: 2 })
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('partial'))
  ref.send({
    type: 'ESTIMATE_RETRY',
    scopeKey,
    testId: 'test',
    expectedRevision: 1,
    candidateIds: ['candidate-0', 'candidate-3'],
  })
  expect(api.estimateRetry).not.toHaveBeenCalled()
  ref.send({
    type: 'ESTIMATE_RETRY',
    scopeKey,
    testId: 'test',
    expectedRevision: 1,
    candidateIds: ['candidate-3'],
  })
  await waitFor(ref, (s) => s.matches('quoted'))
  expect(api.retry).not.toHaveBeenCalled()
  send(ref, { type: 'CONFIRM' })
  send(ref, { type: 'CONFIRM' })
  await waitFor(ref, (s) => s.matches('running'))
  expect(api.estimate).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  expect(api.retry).toHaveBeenCalledExactlyOnceWith(
    {
      testId: 'test',
      expectedRevision: 1,
      candidateIds: ['candidate-3'],
      requestKey: 'key-1',
      quoteKey: 'quote',
    },
    expect.any(AbortSignal),
  )
  expect(ref.getSnapshot().context.test?.candidates[0]?.output).toBe(partial.candidates[0]?.output)
  ref.stop()
})
it('an expired quote requests correction and requires a new explicit estimate rather than starting on confirmation', async () => {
  const api = client()
  vi.mocked(api.estimate).mockResolvedValue({ ...quote, expiresAt: '2000-01-01T00:00:00Z' })
  const ref = actor(api)
  send(ref, { type: 'ESTIMATE' })
  await waitFor(ref, (s) => s.matches('quoted'))
  send(ref, { type: 'CONFIRM' })
  expect(ref.getSnapshot().matches('quoteExpired')).toBe(true)
  expect(api.start).not.toHaveBeenCalled()
  expect(ref.getSnapshot().context.draft?.context.material.text).toBe(plan().context.material.text)
  vi.mocked(api.estimate).mockResolvedValue(quote)
  await generate(ref)
  await waitFor(ref, (s) => s.matches('match'))
  expect(api.start).toHaveBeenCalledTimes(1)
  ref.stop()
})
it('a lost start response preserves the frozen key; a remounted actor never automatically retries it', async () => {
  const api = client()
  vi.mocked(api.start)
    .mockRejectedValueOnce(new Error('response lost'))
    .mockResolvedValueOnce(test())
  const ref = actor(api)
  await generate(ref)
  await waitFor(ref, (s) => s.matches('uncertain'))
  const recovery = { command: ref.getSnapshot().context.command }
  expect(recovery.command?.kind).toBe('start')
  ref.stop()
  const restored = actor(api, { recovery })
  expect(restored.getSnapshot().matches('uncertain')).toBe(true)
  expect(api.start).toHaveBeenCalledTimes(1)
  expect(api.get).not.toHaveBeenCalled()
  send(restored, { type: 'RETRY_OPERATION' })
  send(restored, { type: 'RETRY_OPERATION' })
  await waitFor(restored, (s) => s.matches('match'))
  expect(vi.mocked(api.start).mock.calls[0]![0]).toEqual(vi.mocked(api.start).mock.calls[1]![0])
  restored.stop()
})
it.each(['vote', 'cancel'] as const)(
  'recovers uncertain %s by a read, preserves the operation if the read does not prove it settled, and permits only an explicit identical-key retry',
  async (kind) => {
    const api = client()
    vi.mocked(api.decide).mockRejectedValue(new Error('lost'))
    vi.mocked(api.cancel).mockRejectedValue(new Error('lost'))
    const ref = actor(api, { testId: 'test' })
    await waitFor(ref, (s) => s.matches('match'))
    if (kind === 'vote')
      ref.send({
        type: 'VOTE',
        scopeKey,
        testId: 'test',
        expectedRevision: 1,
        matchId: 'match-0',
        winnerCandidateId: 'candidate-0',
      })
    else ref.send({ type: 'CANCEL', scopeKey, testId: 'test', expectedRevision: 1 })
    await waitFor(ref, (s) => s.matches('uncertain'))
    const original = structuredClone(ref.getSnapshot().context.command)
    send(ref, { type: 'RELOAD' })
    await waitFor(ref, (s) => s.matches('uncertain'))
    expect(ref.getSnapshot().context.command).toEqual(original)
    const recovered =
      kind === 'cancel'
        ? test(2, { status: 'cancelled', revision: 2 })
        : test(2, {
            status: 'completed',
            revision: 2,
            winnerCandidateId: 'candidate-0',
            matches: [{ ...test().matches[0]!, winnerCandidateId: 'candidate-0' }],
          })
    const request = kind === 'vote' ? api.decide : api.cancel
    vi.mocked(request).mockResolvedValue(recovered)
    send(ref, { type: 'RETRY_OPERATION' })
    send(ref, { type: 'RETRY_OPERATION' })
    await waitFor(ref, (s) => s.matches(kind === 'vote' ? 'champion' : 'cancelled'))
    expect(vi.mocked(request).mock.calls[1]![0]).toEqual(vi.mocked(request).mock.calls[0]![0])
    expect(api.start).not.toHaveBeenCalled()
    expect(api.retry).not.toHaveBeenCalled()
    ref.stop()
  },
)
it('readonly recovery confirms an already received vote without resending; a conflicting vote preserves the server bracket', async () => {
  for (const conflicting of [false, true]) {
    const api = client()
    vi.mocked(api.decide).mockRejectedValue(new Error('lost'))
    const ref = actor(api, { testId: 'test' })
    await waitFor(ref, (s) => s.matches('match'))
    ref.send({
      type: 'VOTE',
      scopeKey,
      testId: 'test',
      expectedRevision: 1,
      matchId: 'match-0',
      winnerCandidateId: 'candidate-0',
    })
    await waitFor(ref, (s) => s.matches('uncertain'))
    const winnerCandidateId = conflicting ? 'candidate-1' : 'candidate-0'
    vi.mocked(api.get).mockResolvedValue(
      test(2, {
        revision: 2,
        status: 'completed',
        winnerCandidateId,
        matches: [{ ...test().matches[0]!, winnerCandidateId }],
      }),
    )
    send(ref, { type: 'RELOAD' })
    await waitFor(ref, (s) => s.matches(conflicting ? 'conflict' : 'champion'))
    expect(ref.getSnapshot().context.test?.winnerCandidateId).toBe(winnerCandidateId)
    expect(api.decide).toHaveBeenCalledTimes(1)
    ref.stop()
  }
})
it('publication is explicit, duplicate-fenced and readonly lost-response recovery finds its receipt without resaving/defaulting', async () => {
  const champion = test(2, {
    status: 'completed',
    winnerCandidateId: 'candidate-0',
    revealed: true,
  })
  const api = client(champion)
  vi.mocked(api.saveWinner).mockRejectedValue(new Error('lost publication'))
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('champion'))
  expect(api.saveWinner).not.toHaveBeenCalled()
  const input = {
    testId: 'test',
    expectedRevision: 1,
    winnerCandidateId: 'candidate-0',
    action: 'adopt-model' as const,
    name: '',
    makeDefault: false,
    scope: '',
    scopeIds: [],
  }
  ref.send({ type: 'PUBLISH', scopeKey, input })
  ref.send({ type: 'PUBLISH', scopeKey, input })
  await waitFor(ref, (s) => s.matches('uncertainPublication'))
  const command = ref.getSnapshot().context.command!
  const publication = {
    id: 'receipt',
    testId: 'test',
    winnerCandidateId: 'candidate-0',
    action: 'adopt-model' as const,
    status: 'confirmed' as const,
    requestKey: command.input.requestKey,
    targetId: 'new-default',
  }
  vi.mocked(api.get).mockResolvedValue({ ...champion, revision: 2, publications: [publication] })
  send(ref, { type: 'RELOAD' })
  await waitFor(ref, (s) => s.matches('published'))
  expect(api.saveWinner).toHaveBeenCalledTimes(1)
  expect(ref.getSnapshot().context.publication).toEqual(publication)
  expect(ref.getSnapshot().context.command).toBeUndefined()
  ref.stop()
})
it('an unsettled publication remains uncertain after reread and explicitly retries the exact frozen publication choices/key', async () => {
  const champion = test(2, {
    status: 'completed',
    factor: 'guideline',
    winnerCandidateId: 'candidate-0',
    revealed: true,
  })
  const api = client(champion)
  vi.mocked(api.saveWinner).mockRejectedValueOnce(new Error('lost'))
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('champion'))
  const input = {
    testId: 'test',
    expectedRevision: 1,
    winnerCandidateId: 'candidate-0',
    action: 'save-setting' as const,
    name: 'My tested guideline',
    makeDefault: true,
    scope: 'templates',
    scopeIds: ['template-a'],
  }
  ref.send({ type: 'PUBLISH', scopeKey, input })
  await waitFor(ref, (s) => s.matches('uncertainPublication'))
  input.name = 'Changed later UI draft'
  send(ref, { type: 'RELOAD' })
  await waitFor(ref, (s) => s.matches('uncertainPublication'))
  const command = ref.getSnapshot().context.command!
  const publication = {
    id: 'receipt',
    testId: 'test',
    winnerCandidateId: 'candidate-0',
    action: 'save-setting' as const,
    status: 'confirmed' as const,
    requestKey: command.input.requestKey,
    targetId: 'owned-setting',
  }
  vi.mocked(api.saveWinner).mockResolvedValue({
    test: { ...champion, publications: [publication] },
    publication,
  })
  send(ref, { type: 'RETRY_OPERATION' })
  await waitFor(ref, (s) => s.matches('published'))
  expect(vi.mocked(api.saveWinner).mock.calls[1]![0]).toEqual(
    vi.mocked(api.saveWinner).mock.calls[0]![0],
  )
  ref.stop()
})
it('payload expiry fences votes, retry and publication while preserving durable metadata', async () => {
  const expired = test(2, {
    status: 'completed',
    winnerCandidateId: 'candidate-0',
    contentExpiresAt: '2000-01-01T00:00:00Z',
    candidates: [],
  })
  const api = client(expired)
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('expired'))
  ref.send({
    type: 'VOTE',
    scopeKey,
    testId: 'test',
    expectedRevision: 1,
    matchId: 'match-0',
    winnerCandidateId: 'candidate-0',
  })
  ref.send({
    type: 'PUBLISH',
    scopeKey,
    input: {
      testId: 'test',
      expectedRevision: 1,
      winnerCandidateId: 'candidate-0',
      action: 'adopt-model',
      makeDefault: false,
      name: '',
      scope: '',
      scopeIds: [],
    },
  })
  expect(api.decide).not.toHaveBeenCalled()
  expect(api.saveWinner).not.toHaveBeenCalled()
  expect(ref.getSnapshot().context.test?.winnerCandidateId).toBe('candidate-0')
  ref.stop()
})
it('owner suspension rejects late reads and old events, retains the paid command and never cancels on close', async () => {
  let resolve!: (value: WritingTest) => void
  const api = client()
  vi.mocked(api.start).mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done
      }),
  )
  const ref = actor(api)
  await generate(ref)
  expect(ref.getSnapshot().matches('starting')).toBe(true)
  send(ref, { type: 'SUSPEND' })
  resolve(test())
  await Promise.resolve()
  await Promise.resolve()
  expect(ref.getSnapshot().matches('suspended')).toBe(true)
  expect(ref.getSnapshot().context.test).toBeUndefined()
  expect(ref.getSnapshot().context.command?.kind).toBe('start')
  ref.send({ type: 'RESULT', scopeKey, operation: 2, test: test() })
  expect(ref.getSnapshot().context.test).toBeUndefined()
  expect(api.cancel).not.toHaveBeenCalled()
  ref.stop()
})
it('stale operation/revision hydration cannot downgrade durable outputs', async () => {
  let resolve!: (value: WritingTest) => void
  const api = client(test(2, { revision: 3 }))
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('match'))
  vi.mocked(api.get).mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done
      }),
  )
  send(ref, { type: 'RELOAD' })
  const operation = ref.getSnapshot().context.operation
  for (const event of [
    {
      type: 'HYDRATE' as const,
      scopeKey,
      operation: operation - 1,
      test: test(2, { revision: 9 }),
    },
    { type: 'HYDRATE' as const, scopeKey, operation, test: test(2, { revision: 2 }) },
    { type: 'HYDRATE' as const, scopeKey: 'bob', operation, test: test(2, { revision: 9 }) },
  ])
    ref.send(event)
  expect(ref.getSnapshot().context.test?.revision).toBe(3)
  resolve(test(2, { revision: 4 }))
  await waitFor(ref, (s) => s.matches('match'))
  expect(ref.getSnapshot().context.test?.revision).toBe(4)
  ref.stop()
})

it('a server-refused stale quote preserves entered material and requires a new explicit estimate', async () => {
  const api = client()
  vi.mocked(api.start).mockRejectedValue({ reason: 'WRITING_TEST_QUOTE_REQUIRED', params: {} })
  const ref = actor(api)
  await generate(ref)
  await waitFor(ref, (s) => s.matches('quoteExpired'))
  expect(ref.getSnapshot().context.command).toBeUndefined()
  expect(ref.getSnapshot().context.draft?.context.material.text).toBe(plan().context.material.text)
  expect(api.estimate).toHaveBeenCalledTimes(1)
  send(ref, { type: 'CONFIRM' })
  expect(api.start).toHaveBeenCalledTimes(1)
  ref.stop()
})

it('an uncertain failed-only retry resolves by reading the advanced job and does not regenerate successful entrants', async () => {
  const partial = test(2, { status: 'partial' })
  partial.candidates[1] = { ...partial.candidates[1]!, status: 'failed', output: undefined }
  const api = client(partial)
  vi.mocked(api.retry).mockRejectedValue(new Error('retry response lost'))
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('partial'))
  ref.send({
    type: 'ESTIMATE_RETRY',
    scopeKey,
    testId: 'test',
    expectedRevision: 1,
    candidateIds: ['candidate-1'],
  })
  await waitFor(ref, (s) => s.matches('quoted'))
  send(ref, { type: 'CONFIRM' })
  await waitFor(ref, (s) => s.matches('uncertain'))
  vi.mocked(api.get).mockResolvedValue({
    ...partial,
    revision: 2,
    status: 'running',
    jobId: 'retry-job',
  })
  send(ref, { type: 'RELOAD' })
  await waitFor(ref, (s) => s.matches('running'))
  expect(ref.getSnapshot().context.command).toBeUndefined()
  expect(ref.getSnapshot().context.test?.candidates[0]?.output).toBe(partial.candidates[0]?.output)
  expect(api.retry).toHaveBeenCalledTimes(1)
  expect(api.start).not.toHaveBeenCalled()
  ref.stop()
})

it('purged successful payloads and incomplete server champion records refuse all decision and publication actions', async () => {
  for (const scenario of ['purged', 'incomplete'] as const) {
    const invalid = test(2, { status: 'completed', winnerCandidateId: 'candidate-0' })
    if (scenario === 'purged')
      invalid.candidates[0] = { ...invalid.candidates[0]!, output: undefined }
    else invalid.matches = []
    const api = client(invalid)
    const ref = actor(api, { testId: 'test' })
    await waitFor(ref, (s) => s.matches(scenario === 'purged' ? 'expired' : 'failed'))
    ref.send({
      type: 'PUBLISH',
      scopeKey,
      input: {
        testId: 'test',
        expectedRevision: 1,
        winnerCandidateId: 'candidate-0',
        action: 'adopt-model',
        makeDefault: false,
        name: '',
        scope: '',
        scopeIds: [],
      },
    })
    expect(api.saveWinner).not.toHaveBeenCalled()
    expect(ref.getSnapshot().context.test?.winnerCandidateId).toBe('candidate-0')
    ref.stop()
  }
})

it('a mismatched publication receipt remains recoverable and cannot confirm a foreign write', async () => {
  const champion = test(2, {
    status: 'completed',
    winnerCandidateId: 'candidate-0',
    revealed: true,
  })
  const api = client(champion)
  vi.mocked(api.saveWinner).mockResolvedValue({
    test: champion,
    publication: {
      id: 'foreign-receipt',
      testId: 'foreign-test',
      winnerCandidateId: 'candidate-0',
      action: 'adopt-model',
      status: 'confirmed',
      requestKey: 'wrong-key',
      targetId: 'wrong-target',
    },
  })
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('champion'))
  ref.send({
    type: 'PUBLISH',
    scopeKey,
    input: {
      testId: 'test',
      expectedRevision: 1,
      winnerCandidateId: 'candidate-0',
      action: 'adopt-model',
      makeDefault: false,
      name: '',
      scope: '',
      scopeIds: [],
    },
  })
  await waitFor(ref, (s) => s.matches('uncertainPublication'))
  expect(ref.getSnapshot().context.publication).toBeUndefined()
  expect(ref.getSnapshot().context.test?.id).toBe('test')
  expect(ref.getSnapshot().context.command?.input.requestKey).toBe('key-1')
  ref.stop()
})
it('checks POST numeric bounds and rejects dense empty, sparse and nonconcrete references before estimating', () => {
  for (const [targetLength, tagCount, reason] of [
    [POST_TARGET_LENGTH_MIN - 1, POST_TAG_COUNT_MIN, 'POST_TARGET_LENGTH_INVALID'],
    [POST_TARGET_LENGTH_MAX + 1, POST_TAG_COUNT_MIN, 'POST_TARGET_LENGTH_INVALID'],
    [1000.5, POST_TAG_COUNT_MIN, 'POST_TARGET_LENGTH_INVALID'],
    [NaN, POST_TAG_COUNT_MIN, 'POST_TARGET_LENGTH_INVALID'],
    [POST_TARGET_LENGTH_MIN, POST_TAG_COUNT_MIN - 1, 'POST_TAG_COUNT_INVALID'],
    [POST_TARGET_LENGTH_MIN, POST_TAG_COUNT_MAX + 1, 'POST_TAG_COUNT_INVALID'],
  ] as const) {
    const invalid = plan()
    invalid.context.targetLength = targetLength
    invalid.context.tagCount = tagCount
    expect(writingTestDraftProblem(invalid)?.reason).toBe(reason)
    const api = client()
    const ref = actor(api)
    ref.send({ type: 'DRAFT', scopeKey, draft: invalid })
    send(ref, { type: 'ESTIMATE' })
    expect(api.estimate).not.toHaveBeenCalled()
    ref.stop()
  }
  for (const [targetLength, tagCount] of [
    [POST_TARGET_LENGTH_MIN, POST_TAG_COUNT_MIN],
    [POST_TARGET_LENGTH_MAX, POST_TAG_COUNT_MAX],
  ]) {
    const valid = plan()
    valid.context.targetLength = targetLength!
    valid.context.tagCount = tagCount!
    expect(writingTestDraftProblem(valid)).toBeUndefined()
  }
  const empty = plan()
  empty.entrants = Array.from({ length: 2 }, () => ({
    type: 'model',
    model: { providerId: '', modelId: '' },
  }))
  expect(writingTestDraftProblem(empty)?.reason).toBe('WRITING_TEST_ENTRANT_INVALID')
  const sparse = plan()
  sparse.entrants = new Array(2)
  expect(writingTestDraftProblem(sparse)).toBeDefined()
  const undefinedEntrant = plan()
  undefinedEntrant.entrants = [undefined, undefined] as unknown as WritingTestPlan['entrants']
  expect(writingTestDraftProblem(undefinedEntrant)).toBeDefined()
  const blank = plan()
  blank.entrants[0] = { type: 'model', model: { providerId: '  ', modelId: 'model' } }
  expect(writingTestDraftProblem(blank)?.reason).toBe('WRITING_TEST_ENTRANT_INVALID')
  const style = plan()
  style.factor = 'voice'
  style.entrants = [
    { type: 'setting', setting: { kind: 'writing-voice', id: 'voice-a', revision: '  ' } },
    {
      type: 'setting',
      setting: { kind: 'writing-voice', id: 'voice-b', revision: 'real-revision' },
    },
  ]
  expect(writingTestDraftProblem(style)?.reason).toBe('WRITING_TEST_ENTRANT_INVALID')
})
it('source application and model adoption remain separate explicit receipt-fenced actions after publication', async () => {
  const champion = test(2, {
    status: 'completed',
    winnerCandidateId: 'candidate-0',
    sourcePostSlug: 'source',
    revealed: true,
  })
  const api = client(champion)
  vi.mocked(api.applyOutput).mockImplementation(async (input) => {
    const publication = {
      id: 'apply-receipt',
      testId: 'test',
      winnerCandidateId: 'candidate-0',
      action: 'apply-output' as const,
      status: 'confirmed' as const,
      requestKey: input.requestKey,
      targetId: 'source',
    }
    return { test: { ...champion, revision: 2, publications: [publication] }, publication }
  })
  vi.mocked(api.saveWinner).mockImplementation(async (input) => {
    const publication = {
      id: 'adopt-receipt',
      testId: 'test',
      winnerCandidateId: 'candidate-0',
      action: 'adopt-model' as const,
      status: 'confirmed' as const,
      requestKey: input.requestKey,
      targetId: 'writer',
    }
    const previous = await vi.mocked(api.applyOutput).mock.results[0]!.value
    return {
      test: {
        ...previous.test,
        revision: 3,
        publications: [...previous.test.publications, publication],
      },
      publication,
    }
  })
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('champion'))
  const apply = {
    testId: 'test',
    expectedRevision: 1,
    winnerCandidateId: 'candidate-0',
    expectedInputRevision: 7n,
    expectedContentRevision: 12n,
  }
  ref.send({ type: 'APPLY', scopeKey, input: apply })
  await waitFor(ref, (s) => s.matches('published'))
  ref.send({ type: 'APPLY', scopeKey, input: { ...apply, expectedRevision: 2 } })
  expect(api.applyOutput).toHaveBeenCalledTimes(1)
  ref.send({
    type: 'PUBLISH',
    scopeKey,
    input: {
      testId: 'test',
      expectedRevision: 2,
      winnerCandidateId: 'candidate-0',
      action: 'adopt-model',
      makeDefault: false,
      name: '',
      scope: '',
      scopeIds: [],
    },
  })
  await waitFor(ref, (s) => s.matches('published') && s.context.test?.revision === 3)
  ref.send({
    type: 'PUBLISH',
    scopeKey,
    input: {
      testId: 'test',
      expectedRevision: 3,
      winnerCandidateId: 'candidate-0',
      action: 'adopt-model',
      makeDefault: false,
      name: '',
      scope: '',
      scopeIds: [],
    },
  })
  expect(api.saveWinner).toHaveBeenCalledTimes(1)
  expect(api.start).not.toHaveBeenCalled()
  ref.stop()
})
it('guideline publication accepts an optional title only after an explicit valid scope choice', async () => {
  const champion = test(2, {
    status: 'completed',
    factor: 'guideline',
    winnerCandidateId: 'candidate-0',
  })
  const api = client(champion)
  vi.mocked(api.saveWinner).mockImplementation(async (input) => {
    const publication = {
      id: 'guideline-receipt',
      testId: 'test',
      winnerCandidateId: 'candidate-0',
      action: 'save-setting' as const,
      status: 'confirmed' as const,
      requestKey: input.requestKey,
      targetId: 'guideline',
    }
    return { test: { ...champion, publications: [publication] }, publication }
  })
  const ref = actor(api, { testId: 'test' })
  await waitFor(ref, (s) => s.matches('champion'))
  const input = {
    testId: 'test',
    expectedRevision: 1,
    winnerCandidateId: 'candidate-0',
    action: 'save-setting' as const,
    makeDefault: false,
    name: '',
    scope: '',
    scopeIds: [] as string[],
  }
  for (const invalid of [
    input,
    { ...input, scope: 'unknown' },
    { ...input, scope: 'templates' },
    { ...input, scope: 'fields', scopeIds: ['  '] },
    { ...input, scope: 'global', scopeIds: ['unasked-template'] },
  ])
    ref.send({ type: 'PUBLISH', scopeKey, input: invalid })
  expect(api.saveWinner).not.toHaveBeenCalled()
  ref.send({ type: 'PUBLISH', scopeKey, input: { ...input, scope: 'global' } })
  await waitFor(ref, (s) => s.matches('published'))
  expect(api.saveWinner).toHaveBeenCalledTimes(1)
  ref.stop()
})
it.each([false, true])(
  'personal-profile copy is refused while synthetic=%s follows its explicit reusable-setting publication action',
  async (synthetic) => {
    const champion = test(2, {
      status: 'completed',
      factor: 'voice',
      winnerCandidateId: 'candidate-0',
    })
    champion.candidates[0]!.identity = {
      label: synthetic ? 'Synthetic draft' : 'Owned personal voice',
      synthetic,
      source: synthetic
        ? {
            type: 'authoring',
            authoring: { sessionId: 'authoring', candidateId: 'profile', revision: 3 },
          }
        : {
            type: 'setting',
            setting: { kind: 'writing-voice', id: 'owned', revision: 'accepted-profile' },
          },
    }
    const api = client(champion)
    vi.mocked(api.saveWinner).mockImplementation(async (input) => {
      const publication = {
        id: 'profile-receipt',
        testId: 'test',
        winnerCandidateId: 'candidate-0',
        action: input.action,
        status: 'confirmed' as const,
        requestKey: input.requestKey,
        targetId: 'owned-profile',
      }
      return { test: { ...champion, publications: [publication] }, publication }
    })
    const ref = actor(api, { testId: 'test' })
    await waitFor(ref, (s) => s.matches('champion'))
    const input = {
      testId: 'test',
      expectedRevision: 1,
      winnerCandidateId: 'candidate-0',
      action: 'save-setting' as const,
      makeDefault: false,
      name: 'New named style',
      scope: '',
      scopeIds: [],
    }
    ref.send({ type: 'PUBLISH', scopeKey, input })
    if (!synthetic) {
      expect(api.saveWinner).not.toHaveBeenCalled()
      ref.send({ type: 'PUBLISH', scopeKey, input: { ...input, action: 'use-setting' } })
    }
    await waitFor(ref, (s) => s.matches('published'))
    expect(api.saveWinner).toHaveBeenCalledTimes(1)
    ref.stop()
  },
)
