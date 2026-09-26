import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { usePost } from '@/entities/post'
import {
  POST_CONTENT_FIXTURE,
  POST_CONTENT_WITH_VIDEO_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import { createFakePostsTransport, type FakePostRow } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { clearCaret } from '../model/caret-handoff'
import { discardContentQueues } from '../model/content-queue'
import { BlockEditor } from './BlockEditor'

afterEach(() => {
  cleanup()
  discardContentQueues()
  clearCaret()
})

/** The editor over the post as the cache holds it, keyed by its machine write as ② keys it. */
function Editor({ slug }: { slug: string }) {
  const { post } = usePost(slug)
  return post?.content ? (
    <BlockEditor key={`${post.slug}:${post.machineBaselineRevision}`} post={post} />
  ) : null
}

function renderEditor(row: FakePostRow) {
  const calls: string[] = []
  const transport = createFakePostsTransport({ calls, posts: [row] })
  render(<Editor slug={row.slug} />, { wrapper: withProviders(transport, createTestQueryClient()) })
  return { calls }
}

const AUTOSAVED = { timeout: 4_000 }

describe('the draft read-first', () => {
  const reviewPost: FakePostRow = {
    slug: '20260820-jeju',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
  }

  // Change 05 A7 / A10.
  it('renders the draft as prose with no form control until a block is opened', async () => {
    const user = userEvent.setup()
    renderEditor(reviewPost)

    const draft = await screen.findByRole('article', { name: '생성된 글' })
    expect(within(draft).queryByRole('textbox')).not.toBeInTheDocument()
    expect(within(draft).getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    expect(screen.getByLabelText('1번째 블록 내용')).toHaveValue(
      POST_CONTENT_FIXTURE.blocks[0].content,
    )
  })

  // Change 05 A8: cancel restores the value the block had when its editor opened.
  it('restores a cancelled block and keeps a saved one as prose', async () => {
    const user = userEvent.setup()
    renderEditor(reviewPost)

    await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '고쳐 쓴 문단')
    await user.click(screen.getByRole('button', { name: '취소' }))

    expect(screen.queryByLabelText('1번째 블록 내용')).not.toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const reopened = screen.getByLabelText('1번째 블록 내용')
    await user.clear(reopened)
    await user.type(reopened, '확정한 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    expect(screen.queryByLabelText('1번째 블록 내용')).not.toBeInTheDocument()
    expect(screen.getByText('확정한 문단')).toBeInTheDocument()
  })

  // Change 05 A9: the editing UI keeps every capability it had.
  it('keeps add, delete and move available from a block editor', async () => {
    const user = userEvent.setup()
    const { calls } = renderEditor(reviewPost)

    await user.click(await screen.findByRole('button', { name: '2번째 블록 수정' }))
    expect(screen.getByRole('button', { name: '2번째 블록 위로' })).toBeEnabled()
    expect(screen.getByLabelText('2번째 블록 종류')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '삭제' }))
    expect(screen.queryByLabelText('2번째 블록 종류')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '문단 추가' }))
    expect(await screen.findByText('새 문단')).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('SavePostContent'), AUTOSAVED)
  })

  // VIDEO-2: a VIDEO block carries the IMAGE fields and none of its own, so the same three
  // controls edit it — only the list it picks from differs, and it is offered only when the post
  // actually has a clip.
  it('edits a VIDEO block with the attached-video list, alt and caption', async () => {
    const user = userEvent.setup()
    renderEditor({
      slug: '20260820-video',
      status: 'review',
      content: POST_CONTENT_WITH_VIDEO_FIXTURE,
      images: POST_IMAGES_FIXTURE,
      videos: [{ id: 'video-1', filename: 'clip.mp4' }],
      contentRevision: 1n,
      machineBaselineRevision: 1n,
    })

    const blockIndex = POST_CONTENT_WITH_VIDEO_FIXTURE.blocks.length - 1
    await user.click(await screen.findByRole('button', { name: `${blockIndex + 1}번째 블록 수정` }))
    expect(screen.getByText('첨부 영상')).toBeInTheDocument()
    await user.type(screen.getByPlaceholderText('캡션 (선택)'), '파도')
    await user.click(screen.getByRole('button', { name: '저장' }))
    expect(await screen.findByText('파도')).toBeInTheDocument()
  })
})

describe('the block editor and the header', () => {
  const reviewPost: FakePostRow = {
    slug: '20260820-final',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }

  // 취소 restores the block it opened on, so moving must close the editor rather than leave a
  // snapshot pointed at whichever block shifted into that slot.
  it('closes a block editor when the block moves', async () => {
    const user = userEvent.setup()
    renderEditor(reviewPost)

    await user.click(await screen.findByRole('button', { name: '2번째 블록 수정' }))
    await user.click(screen.getByRole('button', { name: '2번째 블록 위로' }))

    expect(screen.queryByRole('button', { name: '취소' })).not.toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[1].content)).toBeInTheDocument()
  })

  // The header editor owns the title, summary and tags — cancelling it must not revert a block.
  it('keeps a block edit made while the header editor was open', async () => {
    const user = userEvent.setup()
    renderEditor(reviewPost)

    await user.click(await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }))
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const block = screen.getByLabelText('1번째 블록 내용')
    await user.clear(block)
    await user.type(block, '유지되어야 하는 문단')
    await user.click(within(block.closest('article')!).getByRole('button', { name: '저장' }))

    // 취소 on the header restores its three fields only.
    await user.click(screen.getAllByRole('button', { name: '취소' })[0])

    expect(screen.getByText('유지되어야 하는 문단')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: POST_CONTENT_FIXTURE.title })).toBeInTheDocument()
  })
})
