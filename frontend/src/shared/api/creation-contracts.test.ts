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
} from './gen/postpilot/v1/writing_test_pb'
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
