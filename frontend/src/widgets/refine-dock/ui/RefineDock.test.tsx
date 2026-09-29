import { cleanup, render, screen, waitFor } from '@testing-library/react'
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
  template: { id: '', name: '' },
  images: [],
  observations: [],
  content: POST_CONTENT_FIXTURE,
  contentRevision: 1n,
  machineBaselineRevision: 1n,
  finalizedRevision: 0n,
  canFinalize: true,
  targetLanguage: 'ko',
} as unknown as PostDraft

function renderDock({
  post = POST,
  beforeFinalize = vi.fn().mockResolvedValue(1n),
}: { post?: PostDraft; beforeFinalize?: () => Promise<bigint> } = {}) {
  const calls: string[] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'writer' }],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    },
    // The server's copy of the post on screen, so FinalizePost answers the way it would.
    posts: {
      posts: [
        {
          slug: POST.slug,
          title: POST.title,
          status: 'review',
          content: POST_CONTENT_FIXTURE,
          // Every photo the content's IMAGE blocks name, or the finalize is refused (POST-13).
          images: [
            { id: 'image-1', filename: 'IMG_1.jpg' },
            { id: 'image-2', filename: 'IMG_2.jpg' },
          ],
          contentRevision: 1n,
          machineBaselineRevision: 1n,
          canFinalize: true,
        },
      ],
    },
  })
  const onFinalized = vi.fn()
  const view = render(
    <RefineDock
      ownerId="alice"
      post={post}
      jobPending={false}
      onRevisionStarted={vi.fn()}
      beforeStart={vi.fn().mockResolvedValue(undefined)}
      beforeFinalize={beforeFinalize}
      onFinalized={onFinalized}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { beforeFinalize, onFinalized, calls, container: view.container }
}

describe('RefineDock', () => {
  // The dock is ONE surface: the revision row, whose heading names the field and carries the
  // step's one way out at its right. The confirming pair used to stand as a second row of
  // full-width buttons under the field, which read as a second, competing interface.
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
    expect(open).toHaveClass('flex-1')
    expect(open).toBeEnabled()

    expect(heading.compareDocumentPosition(open) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(open.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(field.compareDocumentPosition(send) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    for (const gone of ['확정', '확정하고 말투 학습']) {
      expect(screen.queryByRole('button', { name: gone })).not.toBeInTheDocument()
    }
  })

  // POST-56 and POST-57: no popover or modal stands between the press and the run. 확정하기
  // flushes the pending block edit, finalizes the exact revision that flush named, and carries the
  // title the server now holds onward — and nothing about a voice is asked or started.
  it('flushes and finalizes at once on 확정하기, with no surface in between', async () => {
    const user = userEvent.setup()
    const { beforeFinalize, onFinalized, calls } = renderDock()

    await user.click(screen.getByRole('button', { name: '확정하기' }))

    await waitFor(() => expect(onFinalized).toHaveBeenCalledWith('비 온 뒤의 제주'))
    expect(beforeFinalize).toHaveBeenCalledOnce()
    expect(calls.filter((call) => call === 'FinalizePost')).toHaveLength(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정하고 말투 학습' })).not.toBeInTheDocument()
  })

  // THEME-31: the control sits in a heading row too narrow for a sentence, so a refused finalize
  // is said across the dock, ABOVE the row holding the control it explains, and the step holds.
  it('says a refused finalize above the row and does not move the step', async () => {
    const user = userEvent.setup()
    // The flush names a revision the server has already moved past.
    const { onFinalized } = renderDock({ beforeFinalize: vi.fn().mockResolvedValue(2n) })

    await user.click(screen.getByRole('button', { name: '확정하기' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('다른 화면에서 글이 바뀌었어요.')
    const heading = screen.getByText('수정 요청을 입력하세요')
    expect(alert.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(onFinalized).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: '확정하기' })).toBeEnabled()
  })

  it('holds 확정하기 while the draft cannot be finalized', () => {
    renderDock({ post: { ...POST, canFinalize: false } as PostDraft })
    expect(screen.getByRole('button', { name: '확정하기' })).toBeDisabled()
  })

  // A published post takes no revision and no finalize (POST-86): the dock is the road onward to
  // 글 완성, where its address lives, and nothing else — no field, no send, no 확정하기.
  it('gives a published post only the road onward', async () => {
    const user = userEvent.setup()
    const { container, onFinalized, beforeFinalize } = renderDock({
      post: { ...POST, status: 'published', finalizedRevision: 1n } as PostDraft,
    })

    const onward = screen.getByRole('button', { name: '글 완성으로 가기' })
    expect(screen.getAllByRole('button')).toEqual([onward])
    expect(screen.queryByLabelText('수정 요청을 입력하세요')).not.toBeInTheDocument()
    expect(container.querySelector('textarea, input')).toBeNull()

    await user.click(onward)
    expect(onFinalized).toHaveBeenCalledWith('가제')
    expect(beforeFinalize).not.toHaveBeenCalled()
  })

  // A post that is already finalized keeps the road onward and NOTHING standing beside it: the
  // editor's own status line says 확정, and the first changed content save returns the post to
  // `review`, which brings the way out back by itself.
  it('replaces the way out with the road onward once the post is finalized', async () => {
    const user = userEvent.setup()
    const { onFinalized, beforeFinalize, calls } = renderDock({
      post: { ...POST, status: 'finalized', finalizedRevision: 1n } as PostDraft,
    })

    const onward = screen.getByRole('button', { name: '글 완성으로 가기' })
    expect(screen.queryByText('이 revision을 확정했어요.')).not.toBeInTheDocument()
    for (const gone of ['확정하기', '확정', '확정하고 말투 학습']) {
      expect(screen.queryByRole('button', { name: gone })).not.toBeInTheDocument()
    }

    await user.click(onward)
    expect(onFinalized).toHaveBeenCalledWith('가제')
    expect(beforeFinalize).not.toHaveBeenCalled()
    expect(calls).not.toContain('FinalizePost')
  })
})
