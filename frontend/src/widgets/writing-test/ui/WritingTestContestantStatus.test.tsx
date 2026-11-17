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

it('opens a done contestant to its observation, photo by photo, without naming the model', async () => {
  render(<WritingTestContestantStatus test={partial()} />)
  const list = screen.getByRole('region', { name: '후보별 진행' })
  // Only a contestant with a stored result opens; the others stay a status line.
  expect(within(list).queryByRole('button', { name: '후보 B' })).toBeNull()
  expect(within(list).queryByRole('button', { name: '후보 C' })).toBeNull()
  expect(within(list).getByText('관찰 중')).toBeInTheDocument()
  expect(within(list).getByText(/^실패 · /)).toBeInTheDocument()

  const open = within(list).getByRole('button', { name: '후보 A' })
  expect(open).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByText('Scene 0 of dish.jpg')).toBeNull()
  await userEvent.click(open)
  expect(open).toHaveAttribute('aria-expanded', 'true')
  expect(screen.getByRole('region', { name: '후보 A · 사진 1' })).toHaveTextContent(
    'Scene 0 of dish.jpg',
  )
  expect(screen.getByRole('img', { name: '비교용 사진 dish.jpg' })).toBeInTheDocument()
  // The review's own viewer, one column: a photo switcher and no candidate switcher.
  expect(screen.queryByRole('button', { name: '다음 후보' })).toBeNull()
  await userEvent.click(screen.getByRole('button', { name: '다음 사진' }))
  expect(screen.getByRole('region', { name: '후보 A · 사진 2' })).toHaveTextContent(
    'Scene 0 of sign.jpg',
  )
  // Blind until the reveal (MODEL-32).
  expect(screen.queryByText(/Hidden provider/)).toBeNull()
  expect(screen.queryByText(/hidden-supplier/)).toBeNull()
})

it('offers nothing to open once the payload is purged', () => {
  const test = partial()
  test.contentExpiresAt = '2000-01-01T00:00:00Z'
  render(<WritingTestContestantStatus test={test} />)
  expect(screen.queryByRole('button', { name: '후보 A' })).toBeNull()
  expect(screen.getAllByText('관찰 완료')).toHaveLength(2)
})
