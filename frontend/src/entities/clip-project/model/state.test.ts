import { describe, expect, it } from 'vitest'
import { clipFinished, clipState } from './state'

const result = { contentType: 'video/mp4', bytes: 1, durationMs: 1000, createdAt: '2026-09-11' }

describe('clipState', () => {
  it('is a draft while no plan has been written', () => {
    expect(clipState({ editPlanRevision: 0, renderedPlanRevision: 0, result: undefined })).toBe(
      'draft',
    )
  })

  it('never hides a rendered result behind the draft step', () => {
    expect(clipState({ editPlanRevision: 0, renderedPlanRevision: 0, result })).toBe('refining')
  })

  it('keeps a matching unconfirmed render editable', () => {
    expect(clipState({ editPlanRevision: 3, renderedPlanRevision: 3, result })).toBe('refining')
  })

  it('is refining when the plan is newer than its render', () => {
    expect(clipState({ editPlanRevision: 4, renderedPlanRevision: 3, result })).toBe('refining')
  })

  it('is refining when a plan exists but nothing has rendered', () => {
    expect(clipState({ editPlanRevision: 1, renderedPlanRevision: 0, result: undefined })).toBe(
      'refining',
    )
  })
})

const finalized = { at: '2026-09-13', planRevision: 3, resultId: 'result' }
const confirmed = { planRevision: 3, at: '2026-09-12' }

// CLIP-229: 확정 holds exactly while the plan is the confirmed revision, with no reset write.
it('is confirmed only while the plan is the revision 확정하기 recorded', () => {
  expect(clipState({ editPlanRevision: 3, renderedPlanRevision: 3, result, confirmed })).toBe(
    'confirmed',
  )
  expect(clipState({ editPlanRevision: 4, renderedPlanRevision: 3, result, confirmed })).toBe(
    'refining',
  )
  // A zero plan revision never reads as confirmed, whatever a stray record says.
  expect(
    clipState({
      editPlanRevision: 0,
      renderedPlanRevision: 0,
      confirmed: { planRevision: 0, at: '2026-09-12' },
    }),
  ).toBe('draft')
})

// CLIP-76, CLIP-232: finalization outranks confirmation; a saved URL makes it 게시됨.
it('reads a finalized project as completed, or published with its posted URL', () => {
  const done = { editPlanRevision: 3, renderedPlanRevision: 3, result, finalized, confirmed }
  expect(clipState(done)).toBe('completed')
  expect(clipState({ ...done, publishedUrl: 'https://clip.naver.com/x' })).toBe('published')
  // Finalized before confirmation existed: no confirmed revision, still 완료.
  expect(clipState({ editPlanRevision: 3, renderedPlanRevision: 3, result, finalized })).toBe(
    'completed',
  )
})

it('counts confirmed, completed and published as finished', () => {
  expect(
    ['draft', 'refining', 'confirmed', 'completed', 'published'].map((s) =>
      clipFinished(s as never),
    ),
  ).toEqual([false, false, true, true, true])
})

// CLIP-36, CLIP-177: a storyline written before any plan is already ②'s work.
it('derives refining from a storyline with no plan', () => {
  const storyline = {
    paragraphs: [{ text: '앞', observationIds: [] }],
    editedByHand: false,
    addedSourceIds: [],
    takenOutObservationIds: [],
  }
  expect(clipState({ editPlanRevision: 0, renderedPlanRevision: 0, storyline })).toBe('refining')
  expect(clipState({ editPlanRevision: 0, renderedPlanRevision: 0 })).toBe('draft')
})
