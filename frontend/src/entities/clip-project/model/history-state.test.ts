import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { clipState } from './state'
import { clipHistoryState, clipHistoryStateLabel, clipHistoryStatus } from './history-state'

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
  ).toBe('finished')
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
