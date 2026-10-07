import { create } from '@bufbuild/protobuf'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { WritingTest } from '@/entities/writing-test'
import { toMarkdown } from '@/features/export-markdown'
import { writingTestI18n } from '@/features/writing-test'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { WritingTestChampionOutput } from './WritingTestChampionOutput'

function fixture(): WritingTest {
  return {
    id: 'confirmed-test',
    revision: 2,
    factor: 'voice',
    modelStage: 'write',
    count: 2,
    status: 'completed',
    sourcePostSlug: '',
    jobId: 'job',
    candidates: ['a', 'b'].map((id) => ({
      id,
      status: 'succeeded',
      displayLabel: id,
      output: create(PostContentSchema, {
        title: `Retained ${id} title`,
        summary: 'Retained summary',
        tags: ['champion'],
        blocks: [
          create(BlockSchema, {
            type: BlockType.IMAGE,
            file: 'missing.jpg',
            caption: 'Retained caption',
          }),
          create(BlockSchema, { type: BlockType.TEXT, content: `${id} full final paragraph` }),
        ],
      }),
    })),
    matches: [
      {
        id: 'final',
        round: 1,
        index: 0,
        leftCandidateId: 'a',
        rightCandidateId: 'b',
        winnerCandidateId: 'b',
      },
    ],
    winnerCandidateId: 'b',
    publications: [],
    revealed: true,
    createdAt: '2026-10-07T00:00:00Z',
    updatedAt: '2026-10-07T01:00:00Z',
    contentExpiresAt: '2099-10-07T00:00:00Z',
    fictional: false,
    confirmedCredits: 10,
    reservedCredits: 0,
    targetLanguage: 'en',
  }
}
beforeEach(() => i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true))
afterEach(() => vi.restoreAllMocks())

it('reads and copies the declared champion’s complete output without winner or publication controls', async () => {
  const user = userEvent.setup()
  const copy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  const test = fixture()
  render(<WritingTestChampionOutput test={test} />)
  expect(screen.getByRole('heading', { name: '우승 글 결과' })).toBeInTheDocument()
  expect(screen.getAllByRole('article')).toHaveLength(1)
  expect(screen.getByText('b full final paragraph')).toBeInTheDocument()
  expect(screen.queryByText('a full final paragraph')).not.toBeInTheDocument()
  expect(screen.getByText('첨부 자료 · missing.jpg')).toBeInTheDocument()
  expect(screen.getByText('영어로 작성한 결과')).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /승자로 선택|저장|활성 모델/ }),
  ).not.toBeInTheDocument()
  expect(copy).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: 'Markdown 글 복사' }))
  await waitFor(() =>
    expect(copy).toHaveBeenCalledExactlyOnceWith(
      toMarkdown(test.candidates[1]!.output!, [], test.createdAt, 'en'),
    ),
  )
})

it.each(['expired', 'purged'] as const)(
  'shows %s champion metadata without exporting or substituting another result',
  (kind) => {
    const test = fixture()
    if (kind === 'expired') test.contentExpiresAt = '2000-01-01T00:00:00Z'
    else test.candidates[1]!.output = undefined
    render(<WritingTestChampionOutput test={test} />)
    expect(screen.getByRole('status')).toHaveTextContent('글 결과의 보관 기간이 지났어요')
    expect(screen.queryByRole('article')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  },
)

it.each(['review', 'hidden', 'undeclared'] as const)(
  'does not infer a champion from %s output',
  (kind) => {
    const test = fixture()
    if (kind === 'review') test.status = 'review'
    if (kind === 'hidden') test.revealed = false
    if (kind === 'undeclared') test.winnerCandidateId = ''
    render(<WritingTestChampionOutput test={test} />)
    expect(screen.queryByRole('article')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  },
)

it('retains an exact selectable champion copy fallback when clipboard permission is denied', async () => {
  const user = userEvent.setup()
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
  const test = fixture()
  render(<WritingTestChampionOutput test={test} />)
  await user.click(screen.getByRole('button', { name: 'Markdown 글 복사' }))
  const fallback = (await screen.findByRole('textbox', {
    name: '글 복사 · B',
  })) as HTMLTextAreaElement
  expect(fallback).toHaveValue(toMarkdown(test.candidates[1]!.output!, [], test.createdAt, 'en'))
  expect(fallback.selectionEnd).toBe(fallback.value.length)
  expect(fallback).toHaveFocus()
})
