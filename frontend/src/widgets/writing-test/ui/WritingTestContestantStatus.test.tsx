import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { writingTestI18n } from '@/features/writing-test'
import { observationFixture } from './observation-fixture.test-support'
import { WritingTestContestantStatus } from './WritingTestContestantStatus'

beforeEach(() => {
  initializeI18n('ko')
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
})
afterEach(cleanup)

/** A partly failed photo test: A is done, B failed, C still observes. */
function partial() {
  const test = observationFixture(4)
  test.status = 'partial'
  test.matches = []
  test.candidates[1] = {
    ...test.candidates[1]!,
    status: 'failed',
    observations: undefined,
    failure: { reason: 'MODEL_RATE_LIMITED', params: {} },
  }
  test.candidates[2] = { ...test.candidates[2]!, status: 'running', observations: undefined }
  return test
}

it("reads every contestant in the review's observation viewer, each named with its status, without naming the model", async () => {
  const user = userEvent.setup()
  render(<WritingTestContestantStatus test={partial()} />)
  const list = screen.getByRole('region', { name: '후보별 진행' })
  // No row to open per contestant: one held photo beside one contestant's card.
  expect(within(list).queryAllByRole('heading', { level: 4 })).toHaveLength(0)
  expect(
    within(list)
      .getAllByRole('img')
      .map((image) => image.getAttribute('alt')),
  ).toEqual(['비교용 사진 dish.jpg'])
  expect(within(list).getByRole('region', { name: '후보 A · 사진 1' })).toHaveTextContent(
    'Scene 0 of dish.jpg',
  )

  // The contestant switcher names every letter with where it is.
  await user.click(within(list).getByRole('button', { name: '후보 A' }))
  expect(screen.getAllByRole('menuitemradio').map((item) => item.textContent)).toEqual([
    '후보 A · 관찰 완료',
    '후보 B · 실패',
    '후보 C · 관찰 중',
    '후보 D · 관찰 완료',
  ])
  await user.click(screen.getByRole('menuitemradio', { name: '후보 B · 실패' }))
  expect(within(list).getByRole('button', { name: '후보 B' })).toBeVisible()
  // A contestant without observations states its status, and its failure reason, in its card.
  const failed = within(list).getByRole('region', { name: '후보 B · 사진 1' })
  expect(failed).toHaveTextContent(/실패 · \S/)
  expect(failed).not.toHaveTextContent(/Scene/)
  expect(within(list).getByRole('region', { name: '후보 C · 사진 1' })).toHaveTextContent('관찰 중')

  // The photo stays the switcher's own: the next photo keeps the contestant.
  await user.click(within(list).getByRole('button', { name: '다음 사진' }))
  expect(within(list).getByRole('region', { name: '후보 D · 사진 2' })).toHaveTextContent(
    'Scene 3 of sign.jpg',
  )
  expect(within(list).getByRole('button', { name: '후보 B' })).toBeVisible()
  // Blind until the reveal (MODEL-32).
  expect(screen.queryByText(/Hidden provider/)).toBeNull()
  expect(screen.queryByText(/hidden-supplier/)).toBeNull()
})

it('opens on the first contestant with observations', () => {
  const test = partial()
  test.candidates[0] = { ...test.candidates[0]!, status: 'running', observations: undefined }
  render(<WritingTestContestantStatus test={test} />)
  expect(screen.getByRole('button', { name: '후보 D' })).toBeVisible()
  expect(screen.getByRole('button', { name: '다음 후보' })).toBeDisabled()
})

it('offers nothing to open once the payload is purged', () => {
  const test = partial()
  test.contentExpiresAt = '2000-01-01T00:00:00Z'
  render(<WritingTestContestantStatus test={test} />)
  expect(screen.queryByRole('button')).toBeNull()
  expect(screen.queryByRole('img')).toBeNull()
  expect(screen.getAllByText('관찰 완료')).toHaveLength(2)
})
