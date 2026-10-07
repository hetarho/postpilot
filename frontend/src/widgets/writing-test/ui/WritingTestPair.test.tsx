import { create } from '@bufbuild/protobuf'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { useState } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { WritingTest } from '@/entities/writing-test'
import { toNaver } from '@/features/export-naver'
import { toMarkdown } from '@/features/export-markdown'
import { writingTestI18n } from '@/features/writing-test'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { WritingTestPair } from './WritingTestPair'

function fixture(count: 2 | 4 | 8 | 16 = 2): WritingTest {
  return {
    id: 'test-1',
    revision: 1,
    factor: 'model',
    modelStage: 'write',
    count,
    status: 'review',
    sourcePostSlug: '',
    jobId: 'job-1',
    candidates: Array.from({ length: count }, (_, index) => ({
      id: `candidate-${index}`,
      status: 'succeeded',
      displayLabel: `hidden-supplier-${index}`,
      identity: {
        label: `Hidden provider ${index}`,
        source: {
          type: 'model',
          model: { providerId: 'private-provider', modelId: `private-model-${index}` },
        },
        synthetic: false,
      },
      usage: { promptTokens: 777, completionTokens: 888, latencyMs: 999 },
      output: create(PostContentSchema, {
        title: `Complete title ${index}`,
        summary: `Summary ${index}`,
        tags: ['stored'],
        blocks: [
          create(BlockSchema, { type: BlockType.TEXT, content: `Opening ${index}` }),
          create(BlockSchema, { type: BlockType.HEADING, level: 2, content: `Heading ${index}` }),
          create(BlockSchema, {
            type: BlockType.IMAGE,
            file: 'missing.jpg',
            caption: 'Photo caption',
          }),
          create(BlockSchema, {
            type: BlockType.GALLERY,
            files: ['two.jpg', '[bad](https://evil.test).jpg'],
            caption: 'Gallery caption',
          }),
          create(BlockSchema, {
            type: BlockType.VIDEO,
            file: 'movie.mp4',
            caption: 'Video caption',
          }),
          create(BlockSchema, { type: BlockType.LIST, items: ['List one', 'List two'] }),
          create(BlockSchema, { type: BlockType.QUOTE, content: 'Complete quote' }),
          create(BlockSchema, { type: BlockType.TEXT, content: `Last paragraph ${index}` }),
        ],
      }),
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
    targetLanguage: 'en',
  }
}
const callbacks = () => ({ onWinner: vi.fn(), onShowCandidate: vi.fn(), onReading: vi.fn() })
beforeEach(() => {
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
  i18next.addResourceBundle('en', 'writingTests', writingTestI18n.en, true, true)
})
afterEach(() => vi.restoreAllMocks())

it('shows exactly the two current complete posts with canonical last blocks and safe missing-media markers', async () => {
  const test = fixture(16)
  const actions = callbacks()
  render(<WritingTestPair test={test} match={test.matches[0]!} {...actions} />)
  expect(screen.getAllByRole('article')).toHaveLength(2)
  expect(screen.getByText('Last paragraph 0')).toBeInTheDocument()
  expect(screen.getByText('Last paragraph 1')).toBeInTheDocument()
  expect(screen.queryByText('Last paragraph 2')).not.toBeInTheDocument()
  expect(screen.getAllByText('첨부 자료 · missing.jpg')).toHaveLength(2)
  expect(screen.getAllByText('[bad](https://evil.test).jpg')).toHaveLength(2)
  expect(screen.queryByRole('link')).not.toBeInTheDocument()
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.getAllByText('movie.mp4')).toHaveLength(2)
  expect(
    screen.queryByText(/Hidden provider|hidden-supplier|private-provider|777|888|999/),
  ).not.toBeInTheDocument()
  const choices = screen.getAllByRole('button', { name: /승자로 선택/ })
  expect(choices).toHaveLength(2)
  expect(actions.onWinner).not.toHaveBeenCalled()
  await userEvent.setup().click(choices[1]!)
  expect(actions.onWinner).toHaveBeenCalledExactlyOnceWith('candidate-1')
})

it.each(['partial', 'missing', 'expired', 'stale', 'invalid-pair', 'pending'] as const)(
  'refuses a %s match decision without inventing progress',
  async (kind) => {
    const test = fixture(4)
    if (kind === 'partial') test.candidates[3]!.status = 'failed'
    if (kind === 'missing') test.candidates[3]!.output = undefined
    if (kind === 'expired') test.contentExpiresAt = '2000-01-01T00:00:00Z'
    const match = { ...test.matches[0]! }
    if (kind === 'stale') test.matches[0]!.winnerCandidateId = 'candidate-0'
    if (kind === 'invalid-pair') match.rightCandidateId = ''
    const actions = callbacks()
    render(<WritingTestPair test={test} match={match} pending={kind === 'pending'} {...actions} />)
    const choices = screen.queryAllByRole('button', { name: /승자로 선택/ })
    if (kind === 'pending') {
      expect(choices).toHaveLength(2)
      for (const button of choices) {
        expect(button).toBeDisabled()
        await userEvent.setup().click(button)
      }
    } else expect(choices).toHaveLength(0)
    expect(actions.onWinner).not.toHaveBeenCalled()
  },
)

it('copies the full canonical Naver and Markdown outputs in the frozen language after an explicit action', async () => {
  const user = userEvent.setup()
  const clipboard = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  const test = fixture()
  render(<WritingTestPair test={test} match={test.matches[0]!} {...callbacks()} />)
  const candidate = within(screen.getByRole('region', { name: '후보 A' }))
  expect(clipboard).not.toHaveBeenCalled()
  await user.click(candidate.getByRole('button', { name: '네이버용 글 복사' }))
  const content = test.candidates[0]!.output!
  await waitFor(() => expect(clipboard).toHaveBeenCalledWith(toNaver(content, [], 'en')))
  expect(clipboard.mock.calls[0]![0]).toContain('Last paragraph 0')
  expect(clipboard.mock.calls[0]![0]).toContain('photo_1_Photo_caption_photo')
  expect(clipboard.mock.calls[0]![0]).not.toContain('사진_')
  await user.click(candidate.getByRole('button', { name: 'Markdown 글 복사' }))
  await waitFor(() =>
    expect(clipboard).toHaveBeenCalledWith(toMarkdown(content, [], test.createdAt, 'en')),
  )
  expect(clipboard.mock.calls[1]![0]).toContain('language: en')
  expect(clipboard.mock.calls[1]![0]).toContain('Last paragraph 0')
  expect(clipboard.mock.calls[1]![0]).not.toContain('https://evil.test')
})

it('selects the complete copy fallback without trapping post reading in a nested scroll panel', async () => {
  const user = userEvent.setup()
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
  const test = fixture()
  render(<WritingTestPair test={test} match={test.matches[0]!} {...callbacks()} />)
  await user.click(
    within(screen.getByRole('region', { name: '후보 A' })).getByRole('button', {
      name: 'Markdown 글 복사',
    }),
  )
  const fallback = (await screen.findByRole('textbox', {
    name: '글 복사 · A',
  })) as HTMLTextAreaElement
  expect(fallback).toHaveValue(toMarkdown(test.candidates[0]!.output!, [], test.createdAt, 'en'))
  expect(fallback).toHaveFocus()
  expect(fallback.selectionStart).toBe(0)
  expect(fallback.selectionEnd).toBe(fallback.value.length)
  expect(screen.getByRole('article', { name: '후보 A' }).className).not.toMatch(/overflow-y|max-h-/)
})

it('keeps each phone reading position through keyboard candidate switches and a remount', async () => {
  const original = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    ...original(query),
    matches: query.includes('max-width: 767px') || original(query).matches,
  }))
  Object.defineProperty(window, 'scrollY', { configurable: true, writable: true, value: 0 })
  const scroll = vi
    .spyOn(window, 'scrollTo')
    .mockImplementation((x: number | ScrollToOptions = 0, y?: number) => {
      if (typeof x === 'object' && typeof x.top === 'number')
        Object.defineProperty(window, 'scrollY', { configurable: true, value: x.top })
      else if (typeof y === 'number')
        Object.defineProperty(window, 'scrollY', { configurable: true, value: y })
    })
  const reading: Record<string, number> = { 'candidate-0': 120, 'candidate-1': 240 }
  const onReading = vi.fn((id: string, position: number) => {
    reading[id] = position
  })
  const test = fixture()
  const onWinner = vi.fn()
  function Host() {
    const [active, setActive] = useState('candidate-0')
    return (
      <WritingTestPair
        test={test}
        match={test.matches[0]!}
        visibleCandidateId={active}
        onShowCandidate={setActive}
        onReading={onReading}
        onWinner={onWinner}
        reading={reading}
      />
    )
  }
  const mounted = render(<Host />)
  expect(scroll).toHaveBeenLastCalledWith(0, 120)
  const user = userEvent.setup()
  act(() => {
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 360 })
    fireEvent.scroll(window)
  })
  await user.click(screen.getByRole('tab', { name: '후보 A' }))
  await user.keyboard('{ArrowRight}')
  expect(screen.getByRole('tab', { name: '후보 B' })).toHaveFocus()
  expect(scroll).toHaveBeenLastCalledWith(0, 240)
  act(() => {
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 480 })
    fireEvent.scroll(window)
  })
  await user.keyboard('{ArrowLeft}')
  expect(scroll).toHaveBeenLastCalledWith(0, 360)
  mounted.unmount()
  render(<Host />)
  expect(scroll).toHaveBeenLastCalledWith(0, 360)
  expect(onWinner).not.toHaveBeenCalled()
})

it('rechecks output expiry before an explicit vote or export, without a fresh provider request', async () => {
  const now = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-07T00:00:00Z'))
  const user = userEvent.setup()
  const clipboard = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  const test = fixture()
  test.contentExpiresAt = '2026-10-07T00:01:00Z'
  const actions = callbacks()
  render(<WritingTestPair test={test} match={test.matches[0]!} {...actions} />)
  now.mockReturnValue(Date.parse('2026-10-07T00:02:00Z'))
  await user.click(
    within(screen.getByRole('region', { name: '후보 A' })).getByRole('button', {
      name: '네이버용 글 복사',
    }),
  )
  await user.click(screen.getByRole('button', { name: '후보 A를 승자로 선택' }))
  expect(clipboard).not.toHaveBeenCalled()
  expect(actions.onWinner).not.toHaveBeenCalled()
})

it('shows preparation progress rather than expiration when no paid output has completed yet', () => {
  const test = fixture()
  test.status = 'running'
  test.candidates = test.candidates.map((candidate) => ({
    ...candidate,
    status: 'running',
    output: undefined,
  }))
  render(<WritingTestPair test={test} match={test.matches[0]!} {...callbacks()} />)
  expect(screen.getByRole('status')).toHaveTextContent('0 / 2편 준비')
  expect(screen.queryByText('글 결과의 보관 기간이 지났어요')).not.toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})

it('refuses an undecided future server match while the preceding match still awaits a human decision', () => {
  const test = fixture(16)
  const future = {
    id: 'later-match',
    round: 1,
    index: 1,
    leftCandidateId: 'candidate-2',
    rightCandidateId: 'candidate-3',
    winnerCandidateId: '',
  }
  test.matches.push(future)
  const actions = callbacks()
  render(<WritingTestPair test={test} match={future} {...actions} />)
  expect(screen.queryByRole('button', { name: /승자로 선택/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('article')).not.toBeInTheDocument()
  expect(actions.onWinner).not.toHaveBeenCalled()
})

it('restores the phone reading offset after desktop scroll anchoring changes the document position', async () => {
  const original = window.matchMedia
  const events = new EventTarget()
  let phone = true
  const media = {
    ...original('(max-width: 767px)'),
    get matches() {
      return phone
    },
    addEventListener: events.addEventListener.bind(events),
    removeEventListener: events.removeEventListener.bind(events),
  } as MediaQueryList
  vi.spyOn(window, 'matchMedia').mockImplementation((query) =>
    query.includes('max-width: 767px') ? media : original(query),
  )
  Object.defineProperty(window, 'scrollY', { configurable: true, value: 0 })
  const scroll = vi
    .spyOn(window, 'scrollTo')
    .mockImplementation((x: number | ScrollToOptions = 0, y?: number) => {
      if (typeof x === 'object' && typeof x.top === 'number')
        Object.defineProperty(window, 'scrollY', { configurable: true, value: x.top })
      else if (typeof y === 'number')
        Object.defineProperty(window, 'scrollY', { configurable: true, value: y })
    })
  const test = fixture()
  const reading: Record<string, number> = { 'candidate-0': 1200 }
  render(
    <WritingTestPair
      test={test}
      match={test.matches[0]!}
      visibleCandidateId="candidate-0"
      onShowCandidate={vi.fn()}
      onWinner={vi.fn()}
      reading={reading}
      onReading={(id, value) => {
        reading[id] = value
      }}
    />,
  )
  expect(scroll).toHaveBeenLastCalledWith(0, 1200)
  act(() => {
    phone = false
    events.dispatchEvent(new Event('change'))
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 1900 })
    fireEvent.scroll(window)
  })
  expect(reading['candidate-0']).toBe(1900)
  act(() => {
    phone = true
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 2679 })
    fireEvent.scroll(window)
    events.dispatchEvent(new Event('change'))
  })
  await waitFor(() => expect(scroll).toHaveBeenLastCalledWith(0, 1200))
})
