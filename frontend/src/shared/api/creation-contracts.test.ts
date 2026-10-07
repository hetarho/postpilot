import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import {
  StartAuthoringOperationRequestSchema,
  AuthoringSessionSchema,
} from './gen/postpilot/v1/configuration_authoring_pb'
import {
  WritingTestService,
  WritingTestEntrantSchema,
  WritingTestPlanSchema,
  EstimateFailedTestCandidatesRequestSchema,
  WritingTestSchema,
  WritingTestResponseSchema,
  ListWritingTestsResponseSchema,
  WritingTestUsageSchema,
  WritingTestCandidateSchema,
  WritingTestStatus,
  WritingTestCandidateStatus,
} from './gen/postpilot/v1/writing_test_pb'
import { ContentLanguage } from './gen/postpilot/v1/language_pb'
import { PostSchema } from './gen/postpilot/v1/post_pb'
import { VoiceAnalysisSchema, VoiceSampleSchema } from './gen/postpilot/v1/voice_pb'

describe('additive creation transport contracts', () => {
  it('keeps old authoring input backward readable with an unspecified eight-candidate default', () => {
    const original = create(StartAuthoringOperationRequestSchema, {
      sessionId: 'session',
      expectedRevision: 7,
      requestId: 'operation',
      prompt: 'change',
    })
    const roundTrip = fromBinary(
      StartAuthoringOperationRequestSchema,
      toBinary(StartAuthoringOperationRequestSchema, original),
    )
    expect(roundTrip.candidateCount).toBe(0)
    expect(roundTrip.expectedRevision).toBe(7)
    expect(roundTrip.requestId).toBe('operation')
    expect(create(AuthoringSessionSchema).workingSource).toBeUndefined()
  })
  it('does not fabricate accepted revisions for historical material or analysis', () => {
    const legacy = create(VoiceAnalysisSchema, { materialCount: 2 })
    const roundTrip = fromBinary(VoiceAnalysisSchema, toBinary(VoiceAnalysisSchema, legacy))
    expect(roundTrip.sourceVersionsKnown).toBe(false)
    expect(roundTrip.acceptedSources).toEqual([])
    expect(create(VoiceSampleSchema).contentRevision).toBe(0n)
  })
  it('uses server-resolved source references and owner-free mutation requests', () => {
    const entrant = create(WritingTestEntrantSchema, {
      source: {
        case: 'authoringCandidate',
        value: { sessionId: 'session', candidateId: 'opaque', revision: 4 },
      },
    })
    const plan = create(WritingTestPlanSchema, { count: 16, entrants: [entrant] })
    expect(plan.entrants[0].source.case).toBe('authoringCandidate')
    for (const method of Object.values(WritingTestService.method)) {
      const fields = method.input.fields.map((field) => field.name)
      expect(fields).not.toContain('user_id')
      expect(fields).not.toContain('owner_id')
    }
    expect(
      WritingTestService.method.decideTestMatch.input.fields.map((field) => field.name),
    ).toEqual(['test_id', 'expected_revision', 'request_key', 'match_id', 'winner_candidate_id'])
  })

  it('recovers frozen export language from detail and history after the source target changes', () => {
    const source = create(PostSchema, { targetLanguage: ContentLanguage.ENGLISH })
    const stored = create(WritingTestSchema, {
      id: 'english-test',
      sourcePostSlug: 'source',
      targetLanguage: source.targetLanguage,
      candidates: [{ id: 'opaque', output: { title: 'An English post' } }],
    })
    source.targetLanguage = ContentLanguage.KOREAN
    const detail = fromBinary(
      WritingTestResponseSchema,
      toBinary(WritingTestResponseSchema, create(WritingTestResponseSchema, { test: stored })),
    )
    const history = fromBinary(
      ListWritingTestsResponseSchema,
      toBinary(
        ListWritingTestsResponseSchema,
        create(ListWritingTestsResponseSchema, { tests: [stored] }),
      ),
    )
    // Reopening through either read has all provenance needed without source-post or UI-locale state.
    expect(detail.test?.targetLanguage).toBe(ContentLanguage.ENGLISH)
    expect(history.tests[0].targetLanguage).toBe(ContentLanguage.ENGLISH)
    expect(detail.test?.candidates[0].output?.title).toBe('An English post')
    expect(source.targetLanguage).toBe(ContentLanguage.KOREAN)
    // An absent value is never a fabricated language fallback for older responses.
    expect(create(WritingTestSchema).targetLanguage).toBe(ContentLanguage.UNSPECIFIED)
  })

  it('keeps blind usage absent and carries only timing and tokens after completion or abandonment', () => {
    expect(WritingTestUsageSchema.fields.map((field) => field.name)).toEqual([
      'prompt_tokens',
      'completion_tokens',
      'latency_ms',
    ])
    const blind = create(WritingTestCandidateSchema, { id: 'opaque' })
    expect(
      fromBinary(WritingTestCandidateSchema, toBinary(WritingTestCandidateSchema, blind)).usage,
    ).toBeUndefined()
    for (const status of [WritingTestStatus.COMPLETED, WritingTestStatus.CANCELLED]) {
      const stored = create(WritingTestSchema, {
        status,
        revealed: true,
        candidates: [
          {
            id: 'succeeded',
            status: WritingTestCandidateStatus.SUCCEEDED,
            usage: { promptTokens: 120n, completionTokens: 45n, latencyMs: 678n },
          },
          {
            id: 'failed',
            status: WritingTestCandidateStatus.FAILED,
            usage: { promptTokens: 90n, completionTokens: 10n, latencyMs: 400n },
          },
        ],
      })
      const recovered = fromBinary(WritingTestSchema, toBinary(WritingTestSchema, stored))
      expect(recovered.candidates.map((candidate) => candidate.usage)).toMatchObject([
        { promptTokens: 120n, completionTokens: 45n, latencyMs: 678n },
        { promptTokens: 90n, completionTokens: 10n, latencyMs: 400n },
      ])
    }
  })
})

it('requotes failed-only work from the owned stored revision after reload or quote expiry', () => {
  const request = create(EstimateFailedTestCandidatesRequestSchema, {
    testId: 'test-four',
    expectedRevision: 7,
    candidateIds: ['failed-one'],
  })
  const recovered = fromBinary(
    EstimateFailedTestCandidatesRequestSchema,
    toBinary(EstimateFailedTestCandidatesRequestSchema, request),
  )
  expect(recovered.testId).toBe('test-four')
  expect(recovered.expectedRevision).toBe(7)
  expect(recovered.candidateIds).toEqual(['failed-one'])
  expect(
    WritingTestService.method.estimateFailedTestCandidates.input.fields.map((field) => field.name),
  ).toEqual(['test_id', 'expected_revision', 'candidate_ids'])
})
