import { createClient, type Transport } from '@connectrpc/connect'
import { WritingTestService as Service } from '@/shared/api'
import type {
  TestPublicationAction,
  TestOperationInput,
  TestRetryEstimateInput,
  TestRevisionInput,
  WritingTestClient,
} from '../model/types'
import {
  mapWritingTest,
  mapWritingTestPublication,
  mapWritingTestQuote,
  planToProto,
  publicationActionToProto,
  requirePostRevision,
  requireRevision,
  requireText,
  WritingTestValidationError,
} from './mappers'

function revision(input: TestRevisionInput) {
  requireText(input.testId, 'test')
  requireRevision(input.expectedRevision)
}
function operation(input: TestOperationInput) {
  revision(input)
  requireText(input.requestKey, 'operation key')
}
function retry(input: TestRetryEstimateInput) {
  revision(input)
  if (
    !input.candidateIds.length ||
    new Set(input.candidateIds).size !== input.candidateIds.length ||
    input.candidateIds.length > 16
  )
    throw new WritingTestValidationError('retry candidates')
  input.candidateIds.forEach((id) => requireText(id, 'retry candidate'))
}
/** Authentication supplies ownership; no RPC accepts a caller-asserted owner ID. */
export function createWritingTestClient(transport: Transport): WritingTestClient {
  const client = createClient(Service, transport)
  const response = (
    wire: Pick<Awaited<ReturnType<typeof client.getWritingTest>>, 'test'>,
    expectedId?: string,
  ) => {
    if (!wire.test) throw new WritingTestValidationError('response')
    return mapWritingTest(wire.test, expectedId)
  }
  const publication = (
    wire: Awaited<ReturnType<typeof client.saveWritingTestWinner>>,
    input: TestOperationInput & { winnerCandidateId: string },
    expectedAction: TestPublicationAction,
  ) => {
    const test = response(wire, input.testId)
    if (!wire.publication) throw new WritingTestValidationError('publication response')
    const receipt = mapWritingTestPublication(wire.publication, input.testId)
    if (
      (receipt.requestKey !== input.requestKey &&
        !test.publications.some(
          (record) =>
            record.id === receipt.id &&
            record.requestKey === receipt.requestKey &&
            record.action === receipt.action &&
            record.winnerCandidateId === receipt.winnerCandidateId &&
            record.targetId === receipt.targetId &&
            record.status === receipt.status,
        )) ||
      receipt.action !== expectedAction ||
      receipt.winnerCandidateId !== input.winnerCandidateId ||
      test.winnerCandidateId !== input.winnerCandidateId
    )
      throw new WritingTestValidationError('publication operation mismatch')
    return { test, publication: receipt }
  }
  return {
    estimate: async (plan, signal) =>
      mapWritingTestQuote(
        await client.estimateWritingTest({ plan: planToProto(plan) }, { signal }),
      ),
    estimateRetry: async (input, signal) => {
      retry(input)
      return mapWritingTestQuote(
        await client.estimateFailedTestCandidates(
          { ...input, candidateIds: [...input.candidateIds] },
          { signal },
        ),
      )
    },
    start: async (input, signal) => {
      requireText(input.requestKey, 'operation key')
      requireText(input.quoteKey, 'quote')
      const result = response(
        await client.startWritingTest({ ...input, plan: planToProto(input.plan) }, { signal }),
      )
      if (
        result.factor !== input.plan.factor ||
        result.count !== input.plan.count ||
        result.modelStage !== input.plan.modelStage ||
        result.targetLanguage !== input.plan.context.targetLanguage ||
        (result.sourcePostSlug !== '' &&
          result.sourcePostSlug !== input.plan.context.sourcePostSlug)
      )
        throw new WritingTestValidationError('start plan mismatch')
      return result
    },
    get: async (testId, signal) => {
      requireText(testId, 'test')
      return response(await client.getWritingTest({ testId }, { signal }), testId)
    },
    list: async (input = {}, signal) => {
      const pageSize = input.pageSize ?? 20
      if (!Number.isInteger(pageSize) || pageSize < 1 || pageSize > 100)
        throw new WritingTestValidationError('page size')
      const wire = await client.listWritingTests(
        {
          pageSize,
          pageToken: input.pageToken ?? '',
          sourcePostSlug: input.sourcePostSlug ?? '',
          voiceId: input.voiceId ?? '',
        },
        { signal },
      )
      const tests = wire.tests.map((test) => mapWritingTest(test))
      if (new Set(tests.map((test) => test.id)).size !== tests.length)
        throw new WritingTestValidationError('history identities')
      return { tests, nextPageToken: wire.nextPageToken }
    },
    retry: async (input, signal) => {
      retry(input)
      operation(input)
      requireText(input.quoteKey, 'quote')
      return response(
        await client.retryFailedTestCandidates(
          { ...input, candidateIds: [...input.candidateIds] },
          { signal },
        ),
        input.testId,
      )
    },
    decide: async (input, signal) => {
      operation(input)
      requireText(input.matchId, 'match')
      requireText(input.winnerCandidateId, 'winner')
      return response(await client.decideTestMatch(input, { signal }), input.testId)
    },
    cancel: async (input, signal) => {
      operation(input)
      return response(await client.cancelWritingTest(input, { signal }), input.testId)
    },
    saveWinner: async (input, signal) => {
      operation(input)
      requireText(input.winnerCandidateId, 'winner')
      if (input.action === ('apply-output' as string))
        throw new WritingTestValidationError('setting publication action')
      return publication(
        await client.saveWritingTestWinner(
          {
            ...input,
            action: publicationActionToProto(input.action),
            scopeIds: [...input.scopeIds],
          },
          { signal },
        ),
        input,
        input.action,
      )
    },
    applyOutput: async (input, signal) => {
      operation(input)
      requireText(input.winnerCandidateId, 'winner')
      requirePostRevision(input.expectedInputRevision)
      requirePostRevision(input.expectedContentRevision)
      return publication(
        await client.applyWritingTestOutput(input, { signal }),
        input,
        'apply-output',
      )
    },
  }
}
