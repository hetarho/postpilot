import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import type { ExperimentCandidate, ModelExperiment } from '@/entities/model-experiment'
import { ExperimentReview } from './ExperimentReview'

function written(title: string): ExperimentCandidate['output'] {
  return {
    kind: 'write',
    content: { $typeName: 'postpilot.v1.PostContent', title, summary: '', tags: [], blocks: [] },
  }
}

const mocks = vi.hoisted(() => ({ useExperiment: vi.fn(), useSession: vi.fn() }))

// Partial: only the read is faked. `candidateSides` is the entity's own pure ordering, and a
// test that replaced it would stop checking that A and B mean the same candidate everywhere.
vi.mock('@/entities/model-experiment', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/model-experiment')>()),
  useExperiment: mocks.useExperiment,
}))

vi.mock('@/entities/session', () => ({ useSession: mocks.useSession }))
vi.mock('@/entities/voice', () => ({
  useVoices: () => ({
    voices: [{ id: 'voice-default', name: '기본 말투', isDefault: true, deleted: false }],
  }),
  voiceRefLabel: (voice: { name: string; deleted: boolean }) =>
    voice.deleted ? `삭제된 말투 · ${voice.name}` : voice.name,
}))

const experiment: ModelExperiment = {
  id: 'experiment-1',
  stage: 'write',
  origin: 'lab',
  status: 'review',
  postSlug: 'post-1',
  voiceId: 'voice-default',
  templateName: '',
  jobId: 'job-1',
  candidates: [
    {
      id: 'candidate-b',
      displaySide: 'right',
      status: 'succeeded',
      output: written('B 결과'),
      badges: [],
      otherNote: '',
      failure: undefined,
      modelLabel: '',
    },
    {
      id: 'candidate-a',
      displaySide: 'left',
      status: 'succeeded',
      output: written('A 결과'),
      badges: [],
      otherNote: '',
      failure: undefined,
      modelLabel: '',
    },
  ],
  winnerCandidateId: '',
  outcome: '',
  applyFailure: undefined,
  appliedAt: '',
  adoptionRequested: false,
  adoptionFailure: undefined,
  adoptedAt: '',
  createdAt: '',
  finishedAt: '',
  decidedAt: '',
  revealed: false,
  targetLanguage: undefined,
  source: 'post',
  voicePromptKey: '',
  voicePromptText: '',
  voiceAnswer: '',
}

beforeEach(() => {
  mocks.useSession.mockReturnValue({ user: { id: 'alice' } })
  mocks.useExperiment.mockReturnValue({
    experiment,
    isPending: false,
    isError: false,
    refetch: vi.fn(),
  })
})

it('retains full paid outputs and source names while removing ranking and winner controls', async () => {
  const user = userEvent.setup()
  render(
    <ExperimentReview id="experiment-1" backLink={() => <a href="/tests/history">기록으로</a>} />,
  )
  expect(screen.getByRole('heading', { name: '이전 유료 비교 기록' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '기록으로' })).toHaveAttribute('href', '/tests/history')
  const selector = screen.getByRole('tablist', { name: '읽을 결과' })
  expect(selector).toBeInTheDocument()
  expect(screen.getByText('말투 · 기본 말투')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'A 결과' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'B 결과' })).toBeInTheDocument()
  await user.click(screen.getByRole('tab', { name: 'B' }))
  expect(screen.getByRole('article', { name: '후보 B' })).toHaveClass('block')
  expect(screen.getByRole('article', { name: '후보 A' })).toHaveClass('hidden')
  expect(
    screen.queryByRole('button', { name: /이 결과로 선택|순위 정하기|결과 적용/ }),
  ).not.toBeInTheDocument()
})

it('copies retained complete content only after the explicit copy action', async () => {
  const user = userEvent.setup()
  const copy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  render(<ExperimentReview id="experiment-1" backLink={() => null} />)
  expect(copy).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: '결과 A 복사' }))
  await waitFor(() => expect(copy).toHaveBeenCalledWith('A 결과'))
  expect(await screen.findByText('결과 A를 복사했어요.')).toBeInTheDocument()
})

it('keeps a selectable exact copy fallback when clipboard access is unavailable', async () => {
  const user = userEvent.setup()
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
  render(<ExperimentReview id="experiment-1" backLink={() => null} />)
  await user.click(screen.getByRole('button', { name: '결과 A 복사' }))
  const fallback = await screen.findByRole('textbox', { name: '결과 A 복사' })
  expect(fallback).toHaveValue('A 결과')
  expect(fallback).toHaveAttribute('readonly')
  expect(fallback).toHaveFocus()
})

it('does not offer copying when a retained payload expired', () => {
  mocks.useExperiment.mockReturnValue({
    experiment: {
      ...experiment,
      candidates: experiment.candidates.map((candidate) => ({ ...candidate, output: undefined })),
    },
    isPending: false,
    isError: false,
  })
  render(<ExperimentReview id="experiment-1" backLink={() => null} />)
  expect(screen.getByText(/보관 기간이 지나 삭제된 결과/)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /결과 . 복사/ })).not.toBeInTheDocument()
})

it('leaves admitted paid work running without replay and keeps the return entry', () => {
  mocks.useExperiment.mockReturnValue({
    experiment: { ...experiment, status: 'running' },
    isPending: false,
    isError: false,
  })
  render(
    <ExperimentReview id="experiment-1" backLink={() => <a href="/tests/history">기록으로</a>} />,
  )
  expect(screen.getByText(/이 화면을 닫아도 취소되지 않아요/)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '기록으로' })).toHaveAttribute('href', '/tests/history')
  expect(screen.queryByRole('button', { name: /재시도|다시 생성/ })).not.toBeInTheDocument()
})

// MODEL-67: a 말투 반영 비교's review shows the owner's answer and, under each piece, its
// fingerprint comparison labelled 이 후보.
it('reads a 말투 반영 비교 as the answer and each piece with its comparison', () => {
  const share = (voice: number, text: number) => ({
    key: '해요',
    unit: 'share' as const,
    voice,
    text,
  })
  mocks.useExperiment.mockReturnValue({
    experiment: {
      ...experiment,
      source: 'voice',
      postSlug: '',
      voicePromptKey: 'opening_greeting',
      voicePromptText: '첫인사를 써 보세요.',
      voiceAnswer: '안녕하세요, 동네 빵집이에요.',
      candidates: experiment.candidates.map((candidate, index) => ({
        ...candidate,
        output: {
          kind: 'voice' as const,
          text: `조각 ${index}`,
          comparison: [
            {
              item: 'endings' as const,
              unknown: false,
              distance: 0.5,
              headline: '해요',
              facets: [share(0.9, index === 0 ? 0.4 : 0.8)],
            },
          ],
        },
      })),
    },
    isPending: false,
    isError: false,
    refetch: vi.fn(),
  })
  render(<ExperimentReview id="experiment-1" backLink={() => null} />)

  expect(screen.getByRole('region', { name: '내 답' })).toHaveTextContent(
    '안녕하세요, 동네 빵집이에요.',
  )
  expect(screen.getByText('조각 0')).toBeInTheDocument()
  expect(screen.getAllByText(/내 말투 90% · 이 후보/).map((line) => line.textContent)).toEqual(
    expect.arrayContaining(['내 말투 90% · 이 후보 40%', '내 말투 90% · 이 후보 80%']),
  )
})

it('forwards the current owner namespace and removes previous-owner results while the next read is pending', () => {
  mocks.useExperiment.mockImplementation((_id: string, owner: string) =>
    owner === 'alice'
      ? { experiment, isPending: false, isError: false }
      : { experiment: undefined, isPending: true, isError: false },
  )
  const view = render(
    <ExperimentReview id="experiment-1" backLink={() => <a href="/tests/history">기록으로</a>} />,
  )
  expect(screen.getByText('A 결과')).toBeInTheDocument()
  expect(mocks.useExperiment).toHaveBeenLastCalledWith('experiment-1', 'alice')
  mocks.useSession.mockReturnValue({ user: { id: 'bob' } })
  view.rerender(
    <ExperimentReview id="experiment-1" backLink={() => <a href="/tests/history">기록으로</a>} />,
  )
  expect(mocks.useExperiment).toHaveBeenLastCalledWith('experiment-1', 'bob')
  expect(screen.queryByText('A 결과')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /결과 . 복사/ })).not.toBeInTheDocument()
  expect(screen.getByRole('status')).toHaveTextContent('비교 결과를 불러오는 중')
})
