import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { clipState } from './state'
import {
  clipHistoryFilter,
  clipHistoryState,
  clipHistoryStateLabel,
  clipHistoryStatus,
  clipMatchesHistoryFilter,
} from './history-state'

const storyline = {
  paragraphs: [{ text: '직접 본 이야기', observationIds: [] }],
  editedByHand: false,
  addedSourceIds: [],
  takenOutObservationIds: [],
}
const base = { editPlanRevision: 0, renderedPlanRevision: 0 }
const result = { contentType: 'video/mp4', bytes: 1, durationMs: 15000, createdAt: '2026-11-12' }
afterEach(() => initializeI18n('ko'))

it('distinguishes the saved story from a draft, plan, render and explicit finalization', () => {
  expect(clipHistoryState(base)).toBe('draft')
  expect(clipHistoryState({ ...base, storyline })).toBe('storyline_ready')
  expect(clipHistoryState({ ...base, storyline, editPlanRevision: 1 })).toBe('refining')
  expect(clipHistoryState({ ...base, storyline, result })).toBe('refining')
  expect(
    clipHistoryState({
      ...base,
      storyline,
      finalized: { at: 'now', planRevision: 1, resultId: 'result' },
    }),
  ).toBe('completed')
  // A prepared story still opens the editable second lifecycle step.
  expect(clipState({ ...base, storyline })).toBe('refining')
})

it.each(['ko', 'en'] as const)('names only the story as ready in %s', (locale) => {
  initializeI18n(locale)
  expect(clipHistoryStateLabel('storyline_ready')).toBe(
    locale === 'ko' ? '스토리라인 작성 완료' : 'Storyline ready',
  )
  expect(clipHistoryStatus({ ...base, storyline })).toEqual({
    label: clipHistoryStateLabel('storyline_ready'),
    tone: 'accent',
  })
})

it.each([
  ['queued', undefined, '생성 중', 'info'],
  ['running', undefined, '생성 중', 'info'],
  ['running', '2026-11-12', '취소 중', 'info'],
  ['failed', undefined, '실패', 'danger'],
  ['cancelled', undefined, '취소됨', 'neutral'],
  ['done', undefined, '스토리라인 작성 완료', 'accent'],
] as const)(
  'gives a %s attempt precedence over a retained story',
  (status, cancelRequestedAt, label, tone) => {
    initializeI18n('ko')
    const latestJob = { status, cancelRequestedAt }
    expect(clipHistoryStatus({ ...base, storyline, latestJob })).toEqual({ label, tone })
  },
)

const finalized = { at: '2026-11-12', planRevision: 2, resultId: 'result' }
const confirmed = { planRevision: 2, at: '2026-11-12' }
const confirmedProject = { ...base, storyline, editPlanRevision: 2, result, confirmed }
const completed = { ...confirmedProject, finalized }
const published = { ...completed, publishedUrl: 'https://www.instagram.com/reel/x' }
// Finalized before confirmation existed (CLIP-76): no confirmed revision.
const legacy = { ...base, editPlanRevision: 2, result, finalized }

it.each([
  [confirmedProject, 'confirmed', '확정', 'success'],
  [completed, 'completed', '완료', 'done'],
  [published, 'published', '게시됨', 'done'],
  [legacy, 'completed', '완료', 'done'],
] as const)('names the %#th finished project apart', (project, state, label, tone) => {
  initializeI18n('ko')
  expect(clipHistoryState(project)).toBe(state)
  expect(clipHistoryStatus(project)).toEqual({ label, tone })
})

it.each([
  ['running', '생성 중', 'info'],
  ['failed', '실패', 'danger'],
  ['cancelled', '취소됨', 'neutral'],
] as const)('keeps a %s attempt ahead of a published clip', (status, label, tone) => {
  initializeI18n('ko')
  expect(clipHistoryStatus({ ...published, latestJob: { status } })).toEqual({ label, tone })
})

it('lets 완료 hold published rows while 확정 holds only confirmed ones', () => {
  expect(clipMatchesHistoryFilter(completed, 'completed')).toBe(true)
  expect(clipMatchesHistoryFilter(published, 'completed')).toBe(true)
  expect(clipMatchesHistoryFilter(legacy, 'completed')).toBe(true)
  expect(clipMatchesHistoryFilter(confirmedProject, 'completed')).toBe(false)
  expect(clipMatchesHistoryFilter(confirmedProject, 'confirmed')).toBe(true)
  expect(clipMatchesHistoryFilter(published, 'confirmed')).toBe(false)
  // The existing refining URL keeps every editable ② project, a prepared story among them.
  expect(clipMatchesHistoryFilter({ ...base, storyline }, 'refining')).toBe(true)
  expect(clipMatchesHistoryFilter({ ...base, storyline }, 'storyline_ready')).toBe(true)
})

it('reads the former finished filter as 완료 and drops unknown values', () => {
  expect(clipHistoryFilter('finished')).toBe('completed')
  expect(clipHistoryFilter('confirmed')).toBe('confirmed')
  expect(clipHistoryFilter('published')).toBeUndefined()
  expect(clipHistoryFilter(undefined)).toBeUndefined()
})
