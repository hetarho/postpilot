import { create } from '@bufbuild/protobuf'
import {
  BlockType,
  ObservationSchema,
  WritingTestOutput,
  WritingTestPhotoSchema,
  WritingTestSchema,
  WritingTestFactor,
  WritingTestStage,
  WritingTestStatus,
  WritingTestCandidateStatus,
  contentLanguageToProto,
} from '@/shared/api'
import type { TestCount, WritingTestPlan } from '../model/types'

export function testPlan(count: TestCount = 2): WritingTestPlan {
  return {
    factor: 'model',
    modelStage: 'write',
    count,
    entrants: Array.from({ length: count }, (_, index) => ({
      type: 'model' as const,
      model: { providerId: 'openrouter', modelId: `model-${index}` },
    })),
    context: {
      sourcePostSlug: '',
      expectedInputRevision: 0n,
      expectedContentRevision: 0n,
      material: {
        text: 'Common fictional scenario',
        fictional: true,
        attachmentIds: [],
        templateAnswers: [],
      },
      voiceId: '',
      templateId: '',
      guidelineSlotId: '',
      targetLanguage: 'ko',
      targetLength: 0,
      tagCount: 3,
      useMemory: false,
      qualityRules: [],
      photoIds: [],
    },
  }
}
/** An observation plan: model entrants at the observe stage over test photos alone. */
export function observationPlan(photoIds: string[], count: TestCount = 2): WritingTestPlan {
  const plan = testPlan(count)
  return {
    ...plan,
    output: 'observation',
    modelStage: 'observe',
    context: {
      ...plan.context,
      material: { text: '', fictional: true, attachmentIds: [], templateAnswers: [] },
      tagCount: 0,
      photoIds,
    },
  }
}
export function wireTest(count: TestCount = 2, completed = false) {
  const candidates = Array.from({ length: count }, (_, index) => ({
    id: `candidate-${index}`,
    status: WritingTestCandidateStatus.SUCCEEDED,
    displayLabel: `Candidate ${index + 1}`,
    output: {
      title: `Complete post ${index}`,
      blocks: [{ type: BlockType.TEXT, content: `Complete prose ${index}` }],
    },
  }))
  const matches: {
    id: string
    round: number
    index: number
    leftCandidateId: string
    rightCandidateId: string
    winnerCandidateId: string
  }[] = []
  let roundIds = candidates.map((candidate) => candidate.id)
  for (let round = 1; roundIds.length > 1; round++) {
    const next = []
    for (let index = 0; index < roundIds.length / 2; index++) {
      const left = roundIds[index * 2]
      const right = roundIds[index * 2 + 1]
      matches.push({
        id: `match-${round}-${index}`,
        round,
        index,
        leftCandidateId: completed || round === 1 ? left : '',
        rightCandidateId: completed || round === 1 ? right : '',
        winnerCandidateId: completed ? left : '',
      })
      next.push(left)
    }
    roundIds = next
  }
  return create(WritingTestSchema, {
    id: 'test',
    revision: 4,
    count,
    factor: WritingTestFactor.MODEL,
    modelStage: WritingTestStage.WRITE,
    status: completed ? WritingTestStatus.COMPLETED : WritingTestStatus.REVIEW,
    candidates,
    matches,
    winnerCandidateId: completed ? roundIds[0] : '',
    revealed: completed,
    targetLanguage: contentLanguageToProto('ko'),
    confirmedCredits: 12n,
    reservedCredits: 20n,
    createdAt: '2026-10-07T00:00:00Z',
    updatedAt: '2026-10-07T00:00:01Z',
    contentExpiresAt: '2099-01-01T00:00:00Z',
    fictional: true,
  })
}
/** A finished observation test: each candidate holds one observation per photo, no post. */
export function wireObservationTest(photoIds: string[], count: TestCount = 2, completed = false) {
  const test = wireTest(count, completed)
  test.output = WritingTestOutput.OBSERVATION
  test.modelStage = WritingTestStage.OBSERVE
  test.photos = photoIds.map((id) =>
    create(WritingTestPhotoSchema, {
      id,
      filename: `${id}.jpg`,
      width: 800,
      height: 600,
      viewUrl: `https://storage.test/${id}`,
      viewExpiresAt: '2099-01-01T00:00:00Z',
    }),
  )
  test.candidates.forEach((candidate, index) => {
    candidate.output = undefined
    candidate.observations = photoIds.map((id) =>
      create(ObservationSchema, {
        file: `${id}.jpg`,
        scene: `Scene ${index}`,
        mood: 'calm',
        objects: ['table'],
        // The observer is identity: it stays empty while the test is blind (MODEL-32).
        model: completed ? `model-${index}` : '',
      }),
    )
  })
  return test
}
