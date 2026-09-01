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

function renderDock(beforeStart = vi.fn().mockResolvedValue(undefined)) {
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
      post={POST}
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
  // A10: row 1 is the instruction with its icon send button, row 2 is the pair that ends the step.
  it('lays out the revision row above the two confirming actions', () => {
    renderDock()

    const field = screen.getByLabelText('수정 요청')
    const send = screen.getByRole('button', { name: '수정' })
    const finalize = screen.getByRole('button', { name: '확정' })
    const learn = screen.getByRole('button', { name: '확정하고 말투 학습' })

    expect(field.compareDocumentPosition(send) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(send.compareDocumentPosition(finalize) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(finalize.compareDocumentPosition(learn) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  // A13/§8.3: the keyboard covers the bottom ~40%, so it may hide a control but never the reason
  // that control is disabled.
  it('renders every reason above the control it explains', async () => {
    renderDock()

    const blocker = await screen.findByText('수정 요청을 입력하세요.')
    const learnBlocked = screen.getByText('새 결과가 필요해요.')
    expect(
      blocker.compareDocumentPosition(screen.getByLabelText('수정 요청')) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    expect(
      learnBlocked.compareDocumentPosition(
        screen.getByRole('button', { name: '확정하고 말투 학습' }),
      ) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
  })

  // A13: the dock is mounted OUTSIDE the step panel, so a finalize may never name a revision that
  // omits a block edit the user has already made.
  it('flushes a pending block edit before naming the revision it confirms', async () => {
    const user = userEvent.setup()
    const { beforeFinalize } = renderDock()

    await user.click(screen.getByRole('button', { name: '확정' }))
    const dialog = await screen.findByRole('dialog', { name: '이 revision을 확정할까요?' })
    await user.click(within(dialog).getByRole('button', { name: '확정' }))

    await waitFor(() => expect(beforeFinalize).toHaveBeenCalled())
  })
})
