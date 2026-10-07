import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { expect, it } from 'vitest'
import {
  WritingTestService as Service,
  WritingTestPublicationAction,
  WritingTestPublicationStatus,
} from '@/shared/api'
import { createWritingTestClient } from './client'
import { testPlan, wireTest } from './fixtures.test-support'

it('separates quoting from one keyed start and does not issue generation for a vote or a read', async () => {
  const calls: string[] = []
  const plan = testPlan(16)
  plan.context.expectedInputRevision = 9007199254740993n
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.estimateWritingTest, (request) => {
      calls.push('estimate')
      expect(request.plan?.count).toBe(16)
      expect(request.plan?.context?.expectedInputRevision).toBe(9007199254740993n)
      return create(Service.method.estimateWritingTest.output, {
        credits: 64n,
        quoteKey: 'frozen',
        expiresAt: '2099-01-01T00:00:00Z',
      })
    })
    rpc(Service.method.startWritingTest, (request) => {
      calls.push('start')
      expect(request).toMatchObject({
        requestKey: 'start-key',
        quoteKey: 'frozen',
        plan: { count: 16 },
      })
      return create(Service.method.startWritingTest.output, { test: wireTest(16) })
    })
    rpc(Service.method.getWritingTest, () => {
      calls.push('get')
      return create(Service.method.getWritingTest.output, { test: wireTest(16) })
    })
    rpc(Service.method.decideTestMatch, (request) => {
      calls.push('decide')
      expect(request).toMatchObject({
        testId: 'test',
        expectedRevision: 4,
        requestKey: 'vote-key',
        matchId: 'match-1-0',
        winnerCandidateId: 'candidate-0',
      })
      const result = wireTest(16)
      result.revision = 5
      result.matches[0].winnerCandidateId = 'candidate-0'
      return create(Service.method.decideTestMatch.output, { test: result })
    })
  })
  const client = createWritingTestClient(transport)
  const quote = await client.estimate(plan)
  expect(calls).toEqual(['estimate'])
  await client.start({ plan, requestKey: 'start-key', quoteKey: quote.quoteKey })
  await client.get('test')
  await client.decide({
    testId: 'test',
    expectedRevision: 4,
    requestKey: 'vote-key',
    matchId: 'match-1-0',
    winnerCandidateId: 'candidate-0',
  })
  expect(calls).toEqual(['estimate', 'start', 'get', 'decide'])
})
it('re-estimates and retries only the explicitly selected failed IDs with no replacement plan', async () => {
  const calls: unknown[] = []
  const client = createWritingTestClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.estimateFailedTestCandidates, (request) => {
        calls.push(request)
        return create(Service.method.estimateFailedTestCandidates.output, {
          credits: 5n,
          quoteKey: 'retry-quote',
          expiresAt: '2099-01-01T00:00:00Z',
        })
      })
      rpc(Service.method.retryFailedTestCandidates, (request) => {
        calls.push(request)
        return create(Service.method.retryFailedTestCandidates.output, { test: wireTest() })
      })
    }),
  )
  const selection = { testId: 'test', expectedRevision: 4, candidateIds: ['candidate-1'] }
  const quote = await client.estimateRetry(selection)
  await client.retry({ ...selection, quoteKey: quote.quoteKey, requestKey: 'failed-only-key' })
  expect(calls).toEqual([
    expect.objectContaining(selection),
    expect.objectContaining({
      ...selection,
      quoteKey: 'retry-quote',
      requestKey: 'failed-only-key',
    }),
  ])
  expect(calls.some((request) => request && typeof request === 'object' && 'plan' in request)).toBe(
    false,
  )
})
it('refuses malformed request fields before transport, and preserves uncertain transport errors without auto retry', async () => {
  let calls = 0
  const uncertain = new ConnectError('unavailable', Code.Unavailable)
  const client = createWritingTestClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.startWritingTest, () => {
        calls++
        throw uncertain
      })
      rpc(Service.method.decideTestMatch, () => {
        calls++
        throw uncertain
      })
    }),
  )
  await expect(
    client.start({ plan: testPlan(), requestKey: 'stable-key', quoteKey: 'approved' }),
  ).rejects.toMatchObject({ code: Code.Unavailable })
  expect(calls).toBe(1)
  await expect(
    client.retry({
      testId: 'test',
      expectedRevision: 1,
      candidateIds: ['same', 'same'],
      requestKey: 'key',
      quoteKey: 'quote',
    }),
  ).rejects.toThrow('retry candidates')
  await expect(
    client.decide({
      testId: 'test',
      expectedRevision: -1,
      requestKey: 'key',
      matchId: 'match',
      winnerCandidateId: 'winner',
    }),
  ).rejects.toThrow('revision')
  expect(calls).toBe(1)
})
it('validates response IDs and keyed publication receipts without retrying an uncertain save', async () => {
  let saveCalls = 0
  let actionOnWire = WritingTestPublicationAction.ADOPT_MODEL
  const client = createWritingTestClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.getWritingTest, () =>
        create(Service.method.getWritingTest.output, { test: wireTest() }),
      )
      rpc(Service.method.saveWritingTestWinner, (request) => {
        saveCalls++
        const test = wireTest(2, true)
        return create(Service.method.saveWritingTestWinner.output, {
          test,
          publication: {
            id: 'receipt',
            testId: 'test',
            winnerCandidateId: test.winnerCandidateId,
            action: actionOnWire,
            status: WritingTestPublicationStatus.CONFIRMED,
            targetId: 'write',
            requestKey: request.requestKey,
          },
        })
      })
    }),
  )
  await expect(client.get('foreign')).rejects.toThrow('identity mismatch')
  const action = {
    testId: 'test',
    expectedRevision: 4,
    requestKey: 'same-receipt',
    winnerCandidateId: 'candidate-0',
    action: 'adopt-model' as const,
    makeDefault: false,
    name: '',
    scope: '',
    scopeIds: [],
  }
  const result = await client.saveWinner(action)
  expect(result.publication).toMatchObject({ id: 'receipt', requestKey: action.requestKey })
  expect(saveCalls).toBe(1)
  actionOnWire = WritingTestPublicationAction.SAVE_SETTING
  await expect(client.saveWinner(action)).rejects.toThrow('publication operation mismatch')
})
it('refuses an admission response for a different frozen plan', async () => {
  const client = createWritingTestClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.startWritingTest, () =>
        create(Service.method.startWritingTest.output, { test: wireTest(2) }),
      )
    }),
  )
  await expect(
    client.start({ plan: testPlan(4), requestKey: 'same-key', quoteKey: 'quote' }),
  ).rejects.toThrow('start plan mismatch')
})

it('accepts an admitted owner source hidden by the blind boundary without admitting again', async () => {
  let starts = 0
  const plan = testPlan()
  plan.context.sourcePostSlug = 'owned-source'
  const client = createWritingTestClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.startWritingTest, () => {
        starts++
        return create(Service.method.startWritingTest.output, { test: wireTest() })
      })
    }),
  )
  const admitted = await client.start({ plan, requestKey: 'same-key', quoteKey: 'quote' })
  expect(admitted.sourcePostSlug).toBe('')
  expect(starts).toBe(1)
})
it('accepts an original keyed publication receipt proven by the same returned champion record', async () => {
  const input = {
    testId: 'test',
    expectedRevision: 4,
    requestKey: 'fresh-retry-key',
    winnerCandidateId: 'candidate-0',
    action: 'adopt-model' as const,
    makeDefault: false,
    name: '',
    scope: '',
    scopeIds: [],
  }
  const receipt = {
    id: 'receipt',
    testId: 'test',
    winnerCandidateId: 'candidate-0',
    action: WritingTestPublicationAction.ADOPT_MODEL,
    status: WritingTestPublicationStatus.CONFIRMED,
    targetId: 'write',
    requestKey: 'original-key',
  }
  let includeReceipt = true
  const client = createWritingTestClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.saveWritingTestWinner, () => {
        const test = wireTest(2, true)
        if (includeReceipt)
          test.publications.push(
            create(Service.method.saveWritingTestWinner.output, { publication: receipt })
              .publication!,
          )
        return create(Service.method.saveWritingTestWinner.output, { test, publication: receipt })
      })
    }),
  )
  expect((await client.saveWinner(input)).publication.requestKey).toBe('original-key')
  includeReceipt = false
  await expect(client.saveWinner(input)).rejects.toThrow('publication operation mismatch')
})
