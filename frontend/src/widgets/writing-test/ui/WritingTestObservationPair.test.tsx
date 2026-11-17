import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { WritingTest } from '@/entities/writing-test'
import { writingTestI18n } from '@/features/writing-test'
import { observationFixture } from './observation-fixture.test-support'
import { WritingTestPair } from './WritingTestPair'

beforeEach(() => {
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
  i18next.addResourceBundle('en', 'writingTests', writingTestI18n.en, true, true)
})
afterEach(() => vi.restoreAllMocks())

function setup(test: WritingTest, pending = false) {
  const actions = { onWinner: vi.fn(), onShowCandidate: vi.fn(), onReading: vi.fn() }
  render(<WritingTestPair test={test} match={test.matches[0]!} pending={pending} {...actions} />)
  return actions
}
const rows = () => within(screen.getByRole('list')).getAllByRole('listitem')

it('shows every frozen photo with both blind observations beside each other from sm: up and decides with the existing match decision', async () => {
  const actions = setup(observationFixture(16))
  const images = screen.getAllByRole('img')
  expect(images.map((image) => image.getAttribute('src'))).toEqual([
    'https://storage.test/view/photo-1',
    'https://storage.test/view/photo-2',
  ])
  expect(images[0]).toHaveAttribute('width', '800')
  expect(images[0]).toHaveAttribute('height', '600')
  expect(images[0]).toHaveAccessibleName('비교용 사진 dish.jpg')
  expect(rows()).toHaveLength(2)
  for (const [index, row] of rows().entries()) {
    const file = ['dish.jpg', 'sign.jpg'][index]!
    const left = within(row).getByRole('region', { name: `후보 A · ${file}` })
    const right = within(row).getByRole('region', { name: `후보 B · ${file}` })
    expect(left).toHaveTextContent(`Scene 0 of ${file}`)
    expect(left).toHaveTextContent('calm 0')
    expect(left).toHaveTextContent('OPEN')
    expect(left).toHaveTextContent('table, cup 0')
    expect(within(left).getByText('사람').nextElementSibling).toHaveTextContent('있음')
    expect(right).toHaveTextContent(`Scene 1 of ${file}`)
    expect(within(right).getByText('보이는 글자').nextElementSibling).toHaveTextContent('없음')
    expect(within(right).getByText('사람').nextElementSibling).toHaveTextContent('없음')
    // One shared container puts the pair side by side from sm: and stacks it on phones.
    expect(left.parentElement).toBe(right.parentElement)
    expect(left.parentElement).toHaveClass('sm:grid-cols-2')
  }
  // Only the current pair is shown, and no contestant identity or accounting before the reveal.
  expect(screen.queryByText(/Scene 2/)).not.toBeInTheDocument()
  expect(
    screen.queryByText(/Hidden provider|hidden-supplier|private-provider|777|888|999/),
  ).not.toBeInTheDocument()
  const choices = screen.getAllByRole('button', { name: /승자로 선택/ })
  expect(choices.map((choice) => choice.textContent)).toEqual([
    '후보 A를 승자로 선택',
    '후보 B를 승자로 선택',
  ])
  expect(actions.onWinner).not.toHaveBeenCalled()
  await userEvent.setup().click(choices[1]!)
  expect(actions.onWinner).toHaveBeenCalledExactlyOnceWith('candidate-1')
  // Both observations stay on screen, so there is no candidate switch to remember a position for.
  expect(actions.onShowCandidate).not.toHaveBeenCalled()
  expect(actions.onReading).not.toHaveBeenCalled()
})

it('states a missing observation and a photo that cannot be shown without hiding the rest', () => {
  const test = observationFixture()
  test.candidates[1]!.observations = test.candidates[1]!.observations!.filter(
    (entry) => entry.file === 'dish.jpg',
  )
  test.photos![1]!.viewUrl = undefined
  setup(test)
  const second = rows()[1]!
  expect(within(second).getByText('이 사진은 관찰하지 못했어요.')).toBeVisible()
  expect(within(second).getByText('사진을 불러오지 못했어요')).toBeVisible()
  expect(within(second).queryByRole('img')).not.toBeInTheDocument()
  expect(within(second).getByRole('region', { name: '후보 A · sign.jpg' })).toHaveTextContent(
    'Scene 0 of sign.jpg',
  )
  fireEvent.error(screen.getByRole('img'))
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.getAllByText('사진을 불러오지 못했어요')).toHaveLength(2)
})

it.each(['partial', 'expired', 'stale', 'invalid-pair', 'pending'] as const)(
  'refuses a %s match decision without inventing progress',
  async (kind) => {
    const test = observationFixture(4)
    if (kind === 'partial') {
      test.candidates[3]!.status = 'failed'
      test.candidates[3]!.observations = []
    }
    if (kind === 'expired') test.contentExpiresAt = '2020-01-01T00:00:00Z'
    if (kind === 'stale') test.matches[0]!.winnerCandidateId = 'candidate-0'
    if (kind === 'invalid-pair') test.matches[0]!.rightCandidateId = 'candidate-0'
    const actions = setup(test, kind === 'pending')
    if (kind === 'pending') {
      const choices = screen.getAllByRole('button', { name: /승자로 선택/ })
      expect(choices).toHaveLength(2)
      for (const choice of choices) {
        expect(choice).toBeDisabled()
        await userEvent.setup().click(choice)
      }
    } else {
      expect(screen.queryByRole('button', { name: /승자로 선택/ })).not.toBeInTheDocument()
      expect(screen.queryByRole('img')).not.toBeInTheDocument()
      expect(screen.getByRole('status')).toBeVisible()
    }
    if (kind === 'partial')
      expect(screen.getByRole('status')).toHaveTextContent('3 / 4개 관찰 완료')
    if (kind === 'expired')
      expect(screen.getByRole('status')).toHaveTextContent(
        '사진과 관찰 결과의 보관 기간이 지났어요',
      )
    expect(actions.onWinner).not.toHaveBeenCalled()
  },
)
