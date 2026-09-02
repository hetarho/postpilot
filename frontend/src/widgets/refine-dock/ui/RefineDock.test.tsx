import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PostDraft } from '@/entities/post'
import { Stage } from '@/shared/api'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { RefineDock } from './RefineDock'

afterEach(cleanup)

const POST = {
  slug: 'post-a',
  title: '가제',
  memo: '',
  status: 'review',
  voice: { id: 'voice-a', name: '일상 말투', deleted: false, sourceLanguage: 'ko' },
  purpose: { id: '', name: '' },
  images: [],
  observations: [],
  content: POST_CONTENT_FIXTURE,
  contentRevision: 1n,
  machineBaselineRevision: 1n,
  finalizedRevision: 0n,
  canFinalize: true,
  targetLanguage: 'ko',
} as unknown as PostDraft

const LEARNING = {
  canLearn: false,
  blocked: '새 결과가 필요해요.',
  active: false,
  needsAnalyzeModel: false,
  learn: vi.fn(),
} as unknown as Parameters<typeof RefineDock>[0]['learning']

function renderDock(beforeStart = vi.fn().mockResolvedValue(undefined), post: PostDraft = POST) {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'writer' }],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    },
  })
  const beforeFinalize = vi.fn().mockResolvedValue(1n)
  render(
    <RefineDock
      ownerId="alice"
      post={post}
      ruleLanguageMismatch={false}
      learning={LEARNING}
      jobPending={false}
      onRevisionStarted={vi.fn()}
      beforeStart={beforeStart}
      beforeFinalize={beforeFinalize}
      onFinalized={vi.fn()}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { beforeStart, beforeFinalize }
}

describe('RefineDock', () => {
  // The dock is ONE surface now: the revision row, whose heading names the field and carries the
  // step's way out at its right. The confirming pair used to stand as a second row of full-width
  // buttons under the field, which read as a second, competing interface.
  it('puts the way out in the revision row heading and nothing else beside the field', () => {
    renderDock()

    const heading = screen.getByText('수정 요청을 입력하세요')
    const open = screen.getByRole('button', { name: '확정하기' })
    const field = screen.getByLabelText('수정 요청을 입력하세요')
    const send = screen.getByRole('button', { name: '수정' })

    // The label is the field's own name at the `fieldTitle` role — smaller and heavier than the
    // step title it used to borrow — and 확정하기 fills what is left of the row (A9).
    expect(heading.tagName).toBe('LABEL')
    expect(heading).toHaveClass('text-base', 'font-bold')
    expect(open.parentElement).toHaveClass('flex-1')

    expect(heading.compareDocumentPosition(open) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(open.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(field.compareDocumentPosition(send) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    for (const gone of ['확정', '확정하고 말투 학습']) {
      expect(screen.queryByRole('button', { name: gone })).not.toBeInTheDocument()
    }
  })

  // A13/§8.3: the keyboard covers the bottom ~40%, so it may hide a control but never the reason
  // that control is disabled — inside the surface the choice is made on, as well as in the row.
  it('offers both ways out inside 확정하기, each under the reason it is refused for', async () => {
    const user = userEvent.setup()
    renderDock()

    await user.click(screen.getByRole('button', { name: '확정하기' }))
    const panel = await screen.findByRole('dialog', { name: '확정하기' })
    const finalize = within(panel).getByRole('button', { name: '확정' })
    const learn = within(panel).getByRole('button', { name: '확정하고 말투 학습' })
    const learnBlocked = within(panel).getByText('새 결과가 필요해요.')

    expect(finalize).toBeEnabled()
    expect(learn).toBeDisabled()
    expect(finalize.compareDocumentPosition(learn) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(
      learnBlocked.compareDocumentPosition(learn) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
  })

  // A post that is already finalized keeps the road onward and NOTHING standing beside it: the
  // editor's own status badge says 확정, and the first changed content save returns the post to
  // `review`, which brings the way out back by itself.
  it('replaces the way out with the road onward once the post is finalized', () => {
    renderDock(undefined, { ...POST, status: 'finalized' } as PostDraft)

    expect(screen.getByRole('button', { name: '글 완성으로 가기' })).toBeInTheDocument()
    expect(screen.queryByText('이 revision을 확정했어요.')).not.toBeInTheDocument()
    for (const gone of ['확정하기', '확정', '확정하고 말투 학습']) {
      expect(screen.queryByRole('button', { name: gone })).not.toBeInTheDocument()
    }
  })

  // A13: the dock is mounted OUTSIDE the step panel, so a finalize may never name a revision that
  // omits a block edit the user has already made. The panel that offers the choice IS the
  // confirmation — each action carries the sentence saying what it does — so there is no modal
  // between the press and the run.
  it('flushes a pending block edit before naming the revision it confirms', async () => {
    const user = userEvent.setup()
    const { beforeFinalize } = renderDock()

    await user.click(screen.getByRole('button', { name: '확정하기' }))
    const panel = await screen.findByRole('dialog', { name: '확정하기' })
    await user.click(within(panel).getByRole('button', { name: '확정' }))

    await waitFor(() => expect(beforeFinalize).toHaveBeenCalled())
  })
})
