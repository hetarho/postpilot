import { act, fireEvent, render, screen, within } from '@testing-library/react'
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
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function setup(test: WritingTest, pending = false) {
  const actions = { onWinner: vi.fn(), onShowCandidate: vi.fn(), onReading: vi.fn() }
  render(<WritingTestPair test={test} match={test.matches[0]!} pending={pending} {...actions} />)
  return actions
}
const visibleImages = () => screen.queryAllByRole('img')

/** jsdom has no IntersectionObserver; this one records its callback so a test can report the card a
 *  swipe settled on, which is what the card strip listens to. */
function stubObserver() {
  const observer = {
    report: undefined as ((entries: Array<Partial<IntersectionObserverEntry>>) => void) | undefined,
    observed: [] as Element[],
  }
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(callback: (entries: Array<Partial<IntersectionObserverEntry>>) => void) {
        observer.report = callback
      }
      observe(target: Element) {
        observer.observed.push(target)
      }
      disconnect() {}
    },
  )
  return observer
}

it('holds one photo beside one blind card, switches photo and candidate by step, menu and swipe, and decides with the existing match decision', async () => {
  const observer = stubObserver()
  const scrollTo = vi.fn()
  Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: scrollTo })
  const user = userEvent.setup()
  const actions = setup(observationFixture(16))

  // One photo on screen; every photo stays mounted so a switch never waits on a fetch.
  expect(visibleImages().map((image) => image.getAttribute('src'))).toEqual([
    'https://storage.test/view/photo-1',
  ])
  expect(screen.getAllByRole('img', { hidden: true })).toHaveLength(2)
  const [image] = visibleImages()
  expect(image).toHaveAttribute('width', '800')
  expect(image).toHaveAttribute('height', '600')
  expect(image).toHaveAccessibleName('비교용 사진 dish.jpg')
  const left = screen.getByRole('region', { name: '후보 A · 사진 1' })
  expect(left).toHaveTextContent('Scene 0 of dish.jpg')
  expect(left).toHaveTextContent('calm 0')
  expect(left).toHaveTextContent('OPEN')
  expect(left).toHaveTextContent('table, cup 0')
  expect(within(left).getByText('사람').nextElementSibling).toHaveTextContent('있음')
  // The other candidate's card is the next slide of the same strip, already rendered.
  const right = screen.getByRole('region', { name: '후보 B · 사진 1' })
  expect(right).toHaveTextContent('Scene 1 of dish.jpg')
  expect(within(right).getByText('보이는 글자').nextElementSibling).toHaveTextContent('없음')
  expect(within(right).getByText('사람').nextElementSibling).toHaveTextContent('없음')
  expect(observer.observed).toHaveLength(2)

  // The photo switcher moves the photo and keeps the candidate.
  expect(screen.getByRole('button', { name: '이전 사진' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: '다음 사진' }))
  expect(visibleImages().map((each) => each.getAttribute('src'))).toEqual([
    'https://storage.test/view/photo-2',
  ])
  expect(screen.getByRole('button', { name: '사진 2 / 2' })).toBeVisible()
  expect(screen.getByRole('button', { name: '다음 사진' })).toBeDisabled()
  expect(screen.getByRole('region', { name: '후보 A · 사진 2' })).toHaveTextContent(
    'Scene 0 of sign.jpg',
  )
  await user.click(screen.getByRole('button', { name: '사진 2 / 2' }))
  await user.click(screen.getByRole('menuitemradio', { name: '사진 1 · dish.jpg' }))
  expect(screen.getByRole('button', { name: '사진 1 / 2' })).toBeVisible()
  expect(screen.getByRole('region', { name: '후보 A · 사진 1' })).toBeInTheDocument()

  // The candidate switcher scrolls the strip to the other card on the same photo.
  expect(screen.getByRole('button', { name: '이전 후보' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: '다음 후보' }))
  expect(screen.getByRole('button', { name: '후보 B' })).toBeVisible()
  expect(scrollTo).toHaveBeenCalledTimes(1)
  await user.click(screen.getByRole('button', { name: '후보 B' }))
  await user.click(screen.getByRole('menuitemradio', { name: '후보 A' }))
  expect(screen.getByRole('button', { name: '후보 A' })).toBeVisible()
  expect(scrollTo).toHaveBeenCalledTimes(2)
  // A swipe that settles on B moves the switcher without scrolling the strip again.
  act(() =>
    observer.report?.([
      { target: observer.observed[1]!, isIntersecting: true, intersectionRatio: 0.9 },
    ]),
  )
  expect(screen.getByRole('button', { name: '후보 B' })).toBeVisible()
  expect(scrollTo).toHaveBeenCalledTimes(2)

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
  await user.click(choices[1]!)
  expect(actions.onWinner).toHaveBeenCalledExactlyOnceWith('candidate-1')
  // The cards switch inside the viewer, so there is no post candidate switch to remember.
  expect(actions.onShowCandidate).not.toHaveBeenCalled()
  expect(actions.onReading).not.toHaveBeenCalled()
})

it('states a missing observation and a photo that cannot be shown without hiding the rest', async () => {
  const test = observationFixture()
  test.candidates[1]!.observations = test.candidates[1]!.observations!.filter(
    (entry) => entry.file === 'dish.jpg',
  )
  test.photos![1]!.viewUrl = undefined
  setup(test)
  await userEvent.click(screen.getByRole('button', { name: '다음 사진' }))
  expect(screen.getByRole('region', { name: '후보 B · 사진 2' })).toHaveTextContent(
    '이 사진은 관찰하지 못했어요.',
  )
  expect(screen.getByRole('region', { name: '후보 A · 사진 2' })).toHaveTextContent(
    'Scene 0 of sign.jpg',
  )
  expect(visibleImages()).toHaveLength(0)
  const unavailable = () =>
    screen.getAllByText('사진을 불러오지 못했어요').filter((each) => !each.closest('[hidden]'))
  expect(unavailable()).toHaveLength(1)

  await userEvent.click(screen.getByRole('button', { name: '이전 사진' }))
  expect(unavailable()).toHaveLength(0)
  fireEvent.error(visibleImages()[0]!)
  expect(visibleImages()).toHaveLength(0)
  expect(unavailable()).toHaveLength(1)
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
