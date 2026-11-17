import type { WritingTest, WritingTestPhoto } from '@/entities/writing-test'

export const OBSERVATION_PHOTOS: WritingTestPhoto[] = [
  {
    id: 'photo-1',
    filename: 'dish.jpg',
    width: 800,
    height: 600,
    viewUrl: 'https://storage.test/view/photo-1',
    viewExpiresAt: '2099-01-01T00:00:00Z',
  },
  {
    id: 'photo-2',
    filename: 'sign.jpg',
    width: 600,
    height: 800,
    viewUrl: 'https://storage.test/view/photo-2',
    viewExpiresAt: '2099-01-01T00:00:00Z',
  },
]

/** A photo observation test in review: every contestant holds one observation per frozen photo,
 *  and, as on the wire before the champion is confirmed, no post and no model name. */
export function observationFixture(count: 2 | 4 | 8 | 16 = 2): WritingTest {
  return {
    id: 'observation-1',
    revision: 1,
    output: 'observation',
    factor: 'model',
    modelStage: 'observe',
    count,
    status: 'review',
    sourcePostSlug: '',
    jobId: 'job-1',
    photos: OBSERVATION_PHOTOS.map((photo) => ({ ...photo })),
    candidates: Array.from({ length: count }, (_, index) => ({
      id: `candidate-${index}`,
      status: 'succeeded',
      displayLabel: `hidden-supplier-${index}`,
      // Present on purpose: a blind view must not print it even if a payload carried it.
      identity: {
        label: `Hidden provider ${index}`,
        source: {
          type: 'model',
          model: { providerId: 'private-provider', modelId: `private-model-${index}` },
        },
        synthetic: false,
      },
      usage: { promptTokens: 777, completionTokens: 888, latencyMs: 999 },
      observations: OBSERVATION_PHOTOS.map((photo) => ({
        file: photo.filename,
        scene: `Scene ${index} of ${photo.filename}`,
        mood: `calm ${index}`,
        visibleText: index === 0 ? 'OPEN' : '',
        objects: ['table', `cup ${index}`],
        peoplePresent: index === 0,
      })),
    })),
    matches: [
      {
        id: 'match-1',
        round: 1,
        index: 0,
        leftCandidateId: 'candidate-0',
        rightCandidateId: 'candidate-1',
        winnerCandidateId: '',
      },
    ],
    winnerCandidateId: '',
    publications: [],
    revealed: false,
    createdAt: '2026-10-07T00:00:00Z',
    updatedAt: '2026-10-07T00:00:00Z',
    contentExpiresAt: '2099-10-07T00:00:00Z',
    fictional: false,
    confirmedCredits: 0,
    reservedCredits: 10,
    targetLanguage: 'ko',
  }
}

/** The same test after its only match is decided: revealed, with the winner named. */
export function completedObservationFixture(): WritingTest {
  const test = observationFixture()
  test.status = 'completed'
  test.revealed = true
  test.winnerCandidateId = 'candidate-0'
  test.matches[0]!.winnerCandidateId = 'candidate-0'
  test.candidates.forEach((candidate, index) => {
    candidate.identity = {
      label: `Frozen model ${index + 1}`,
      source: { type: 'model', model: { providerId: 'openrouter', modelId: `model-${index}` } },
      synthetic: false,
    }
  })
  return test
}
