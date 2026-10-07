import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  EstimateWritingTestResponseSchema,
  WritingTestFactor,
  WritingTestService,
  WritingTestStatus,
  WritingTestCandidateStatus,
  WritingTestPublicationAction,
  WritingTestPublicationStatus,
  ProtoConfigurationKind,
  contentLanguageToProto,
} from '@/shared/api'
import { writingTestPayloadAvailable, type TestCount } from '../model/types'
import { mapWritingTest, mapWritingTestQuote, planToProto } from './mappers'
import { testPlan, wireTest } from './fixtures.test-support'

describe('exact server writing-test projections', () => {
  it.each<TestCount>([2, 4, 8, 16])(
    'preserves %i outputs and its authoritative completed bracket',
    (count) => {
      const wire = wireTest(count, true)
      const result = mapWritingTest(wire)
      expect(result.candidates).toHaveLength(count)
      expect(result.matches).toHaveLength(count - 1)
      expect(result.matches.filter((match) => match.winnerCandidateId)).toHaveLength(count - 1)
      expect(result.winnerCandidateId).toBe('candidate-0')
      expect(result.targetLanguage).toBe('ko')
    },
  )
  it('drops accidentally projected identity, accounting, secret diagnostics and labels while blind', () => {
    const wire = wireTest()
    wire.candidates[0].displayLabel = 'Secret provider'
    const secret = create(WritingTestService.method.getWritingTest.output, {
      test: {
        candidates: [
          {
            identity: {
              label: 'Private model',
              source: {
                source: {
                  case: 'model',
                  value: { providerId: 'provider', modelId: 'secret-model' },
                },
              },
              synthetic: false,
            },
            usage: { promptTokens: 300n, completionTokens: 600n, latencyMs: 900n },
            failure: {
              reason: 'LLM_PROVIDER_ERROR',
              params: { model: 'secret-model' },
              technicalDetail: 'secret-model provider diagnostics',
            },
          },
        ],
      },
    }).test!.candidates[0]
    Object.assign(wire.candidates[0], {
      identity: secret.identity,
      usage: secret.usage,
      failure: secret.failure,
    })
    const result = mapWritingTest(wire)
    expect(result.candidates[0].identity).toBeUndefined()
    expect(result.candidates[0].usage).toBeUndefined()
    expect(result.candidates[0].displayLabel).toBe('#1')
    expect(JSON.stringify(result)).not.toContain('secret-model')
    wire.status = WritingTestStatus.CANCELLED
    wire.revealed = true
    const revealed = mapWritingTest(wire)
    expect(revealed.candidates[0].identity?.source).toEqual({
      type: 'model',
      model: { providerId: 'provider', modelId: 'secret-model' },
    })
    expect(revealed.candidates[0].usage).toEqual({
      promptTokens: 300,
      completionTokens: 600,
      latencyMs: 900,
    })
  })
  it('keeps frozen content language and metadata when private payload is purged', () => {
    const wire = wireTest(4, true)
    wire.targetLanguage = contentLanguageToProto('en')
    wire.contentExpiresAt = '2020-01-01T00:00:00Z'
    wire.candidates.forEach((candidate) => {
      candidate.output = undefined
    })
    const result = mapWritingTest(wire)
    expect(result.targetLanguage).toBe('en')
    expect(result.matches).toHaveLength(3)
    expect(writingTestPayloadAvailable(result)).toBe(false)
  })
  it.each([
    (wire: ReturnType<typeof wireTest>) => {
      wire.count = 3
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.factor = WritingTestFactor.UNSPECIFIED
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.status = WritingTestStatus.UNSPECIFIED
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.targetLanguage = 0
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.revision = 1.5
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.candidates[1].id = wire.candidates[0].id
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.matches[0].winnerCandidateId = 'foreign'
    },
    (wire: ReturnType<typeof wireTest>) => {
      wire.confirmedCredits = BigInt(Number.MAX_SAFE_INTEGER) + 1n
    },
  ])('refuses malformed server facts without inventing defaults', (change) => {
    const wire = wireTest()
    change(wire)
    expect(() => mapWritingTest(wire)).toThrow()
  })
  it('requires confirmed N-1 decisions and correct final winner for a champion', () => {
    const wire = wireTest(16, true)
    wire.matches[0].winnerCandidateId = ''
    expect(() => mapWritingTest(wire)).toThrow('bracket advancement')
    expect(() => mapWritingTest(wireTest(), 'another')).toThrow('identity mismatch')
  })
  it('requires every successful entrant before review and never reveals identity before abandonment or champion', () => {
    const wire = wireTest(4)
    wire.candidates[3].status = WritingTestCandidateStatus.FAILED
    expect(() => mapWritingTest(wire)).toThrow('complete output barrier')
    wire.candidates[3].status = WritingTestCandidateStatus.SUCCEEDED
    wire.revealed = true
    expect(() => mapWritingTest(wire)).toThrow('premature reveal')
  })
  it('preserves publication receipts and validates their owner-test references', () => {
    const wire = wireTest(2, true)
    const receipt = create(WritingTestService.method.saveWritingTestWinner.output, {
      publication: {
        id: 'receipt',
        testId: wire.id,
        winnerCandidateId: wire.winnerCandidateId,
        action: WritingTestPublicationAction.ADOPT_MODEL,
        status: WritingTestPublicationStatus.CONFIRMED,
        requestKey: 'same-key',
        targetId: 'write',
      },
    }).publication!
    wire.publications = [receipt]
    expect(mapWritingTest(wire).publications[0]).toMatchObject({
      id: 'receipt',
      action: 'adopt-model',
      status: 'confirmed',
    })
    receipt.testId = 'foreign'
    expect(() => mapWritingTest(wire)).toThrow('publication test mismatch')
  })
})

describe('explicit plan and credit boundaries', () => {
  it('preserves bigint source revisions and model/owned/prepared reference variants', () => {
    const plan = testPlan()
    plan.context.expectedInputRevision = 9007199254740993n
    expect(planToProto(plan).context?.expectedInputRevision).toBe(9007199254740993n)
    plan.factor = 'template'
    plan.context.writeModel = { providerId: 'provider', modelId: 'fixed' }
    plan.entrants = [
      { type: 'setting', setting: { id: 'saved', revision: 'frozen-v1', kind: 'post-template' } },
      {
        type: 'authoring',
        authoring: { sessionId: 'session', candidateId: 'candidate', revision: 7 },
      },
    ]
    const wire = planToProto(plan)
    expect(wire.entrants[0].source).toMatchObject({
      case: 'setting',
      value: { kind: ProtoConfigurationKind.POST_TEMPLATE, revision: 'frozen-v1' },
    })
    expect(wire.entrants[1].source).toMatchObject({
      case: 'authoringCandidate',
      value: { revision: 7 },
    })
  })
  it('refuses partial counts, duplicate semantic entrants, wrong factors and observation without attachments', () => {
    const plan = testPlan(4)
    plan.entrants.pop()
    expect(() => planToProto(plan)).toThrow('entrant count')
    const duplicate = testPlan()
    duplicate.entrants[1] = duplicate.entrants[0]
    expect(() => planToProto(duplicate)).toThrow('duplicate entrants')
    const observe = testPlan()
    observe.modelStage = 'observe'
    expect(() => planToProto(observe)).toThrow('observer attachments')
    const wrong = testPlan()
    wrong.factor = 'voice'
    expect(() => planToProto(wrong)).toThrow('entrant factor')
  })
  it('requires bounded known quotes including for free work', () => {
    const quote = {
      free: false,
      credits: 12n,
      quoteKey: 'server-quote',
      expiresAt: '2099-01-01T00:00:00Z',
    }
    expect(mapWritingTestQuote(create(EstimateWritingTestResponseSchema, quote)).credits).toBe(12)
    expect(
      mapWritingTestQuote(
        create(EstimateWritingTestResponseSchema, { ...quote, free: true, credits: undefined }),
      ).credits,
    ).toBe(0)
    for (const credits of [undefined, -1n, BigInt(Number.MAX_SAFE_INTEGER) + 1n])
      expect(() =>
        mapWritingTestQuote(create(EstimateWritingTestResponseSchema, { ...quote, credits })),
      ).toThrow()
    expect(() =>
      mapWritingTestQuote(create(EstimateWritingTestResponseSchema, { ...quote, free: true })),
    ).toThrow('free quote credits')
  })
})
