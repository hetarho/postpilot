import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { usePost } from '@/entities/post'
import {
  POST_CONTENT_FIXTURE,
  POST_CONTENT_WITH_VIDEO_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import {
  createFakePostsTransport,
  finalizedPostRow,
  type FakePostRow,
  type FakePostsOptions,
} from '@/test/posts'
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

function renderEditor(row: FakePostRow, transport?: ReturnType<typeof createFakePostsTransport>) {
  const calls: string[] = []
  const contentSaves: NonNullable<FakePostsOptions['contentSaves']> = []
  const used = transport ?? createFakePostsTransport({ calls, contentSaves, posts: [row] })
  render(<Editor slug={row.slug} />, { wrapper: withProviders(used, createTestQueryClient()) })
  return { calls, contentSaves, transport: used }
}

const article = () => within(screen.getByRole('article', { name: '생성된 글' }))
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

// POST-79, POST-80: the marks stand where the last write's candidates still stand, and taking a
// phrase is an ordinary content save that records nothing about where the words came from.
describe('the replacement marks', () => {
  const SLUG = '20260820-rain'
  const CANDIDATES: NonNullable<FakePostRow['replacementCandidates']> = [
    { surface: 'title', index: 0, source: '제주', phrases: ['제주도', '제주 바다'] },
    // '여행' would duplicate tag 2, so only '산책로' is offered.
    { surface: 'tag', index: 1, source: '산책', phrases: ['산책로', '여행'] },
    {
      surface: 'body',
      index: 0,
      source: '기다렸다',
      phrases: ['기다린다', '기다려 본다', '기다리고 있었다', '기다렸어요'],
    },
    { surface: 'body', index: 0, source: '비가', phrases: ['빗줄기가'] },
    { surface: 'body', index: 2, source: '바닷가로', phrases: ['해변으로'] },
  ]
  const marked = finalizedPostRow({
    slug: SLUG,
    images: POST_IMAGES_FIXTURE,
    replacementCandidates: CANDIDATES,
  })

  async function renderMarks() {
    const view = renderEditor(marked)
    await screen.findByRole('article', { name: '생성된 글' })
    return view
  }

  it('marks the title, a tag and a TEXT block', async () => {
    await renderMarks()

    const heading = article().getByRole('heading', { level: 3 })
    const title = within(heading)
    expect(title.getByRole('button', { name: '제주' })).toHaveAttribute('aria-haspopup', 'dialog')
    // The text around a mark is kept whole.
    expect(heading).toHaveTextContent(/^비 온 뒤의 제주$/)
    expect(article().getByRole('button', { name: '비가' }).parentElement).toHaveTextContent(
      /^비가 그치기를 기다렸다\.$/,
    )
    const tags = within(article().getByRole('list', { name: '태그' }))
    // A chip stands alone, so its mark grows to the pointer floor under a coarse pointer; a mark in
    // a sentence takes WCAG 2.5.8's inline exception (THEME-23).
    expect(tags.getByRole('button', { name: '산책' })).toHaveClass(
      'pointer-coarse:min-h-11',
      'pointer-coarse:min-w-11',
    )
    expect(article().getByRole('button', { name: '기다렸다' })).not.toHaveClass(
      'pointer-coarse:min-h-11',
    )
    // The tag chip with no candidate stays plain text.
    expect(tags.queryByRole('button', { name: '제주' })).toBeNull()
    expect(article().getByRole('button', { name: '기다렸다' })).toBeInTheDocument()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
  })

  it('offers at most three phrases and no duplicate tag', async () => {
    const user = userEvent.setup()
    await renderMarks()

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    const panel = within(await screen.findByRole('dialog', { name: '‘기다렸다’ 바꿔 쓰기' }))
    expect(panel.getAllByRole('button').map((button) => button.textContent)).toEqual([
      '기다린다',
      '기다려 본다',
      '기다리고 있었다',
    ])
    expect(panel.getByRole('button', { name: '‘기다린다’(으)로 바꾸기' })).toBeInTheDocument()
    expect(panel.getByText('‘기다렸다’ 대신 쓸 수 있는 표현')).toBeInTheDocument()
    // What was observed about the phrases, and nothing about what they gain.
    expect(
      panel.getByText('네이버 검색 결과의 제목과 설명에 자주 나온 표현이에요.'),
    ).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await user.click(article().getByRole('button', { name: '산책' }))
    const tagPanel = within(await screen.findByRole('dialog', { name: '‘산책’ 바꿔 쓰기' }))
    expect(tagPanel.getAllByRole('button').map((button) => button.textContent)).toEqual(['산책로'])
  })

  // POST-79: a taken candidate is spent. Its mark never comes back — not even where the phrase
  // holds its source, as 제주도 holds 제주 — and every other mark stays. Focus goes to the header's
  // pencil.
  it('spends a taken candidate, even where its phrase holds the source', async () => {
    const user = userEvent.setup()
    const { contentSaves, transport } = await renderMarks()

    await user.click(article().getByRole('button', { name: '제주' }))
    await user.click(await screen.findByRole('button', { name: '‘제주도’(으)로 바꾸기' }))

    expect(screen.queryByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeNull()
    const spent = () => {
      const heading = article().getByRole('heading', { level: 3 })
      expect(heading).toHaveTextContent(/^비 온 뒤의 제주도$/)
      expect(within(heading).queryByRole('button', { name: /제주/ })).toBeNull()
      for (const neighbour of ['산책', '기다렸다', '비가'])
        expect(article().getByRole('button', { name: neighbour })).toBeInTheDocument()
    }
    spent()
    await waitFor(() => expect(contentSaves).toHaveLength(1), AUTOSAVED)
    expect(contentSaves[0].content.title).toBe('비 온 뒤의 제주도')
    expect(contentSaves[0].takenCandidates).toEqual([0])
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '제목과 요약, 태그 수정' })).toHaveFocus(),
    )
    spent()

    // A fresh mount reads the post again: the server holds the shorter list.
    cleanup()
    discardContentQueues()
    renderEditor(marked, transport)
    await screen.findByRole('article', { name: '생성된 글' })
    spent()
  })

  it('puts focus on the pencil of the block the take changed', async () => {
    const user = userEvent.setup()
    const { contentSaves } = await renderMarks()

    await user.click(article().getByRole('button', { name: '바닷가로' }))
    await user.click(await screen.findByRole('button', { name: '‘해변으로’(으)로 바꾸기' }))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: '3번째 블록 수정' })).toHaveFocus(),
    )
    await waitFor(() => expect(contentSaves).toHaveLength(1), AUTOSAVED)
    expect(contentSaves[0].content.blocks[2].content).toBe('해변으로')
  })

  it('keeps a neighbouring mark after a take', async () => {
    const user = userEvent.setup()
    await renderMarks()

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    await user.click(await screen.findByRole('button', { name: '‘기다린다’(으)로 바꾸기' }))

    await waitFor(() => expect(article().queryByRole('button', { name: '기다렸다' })).toBeNull())
    expect(article().getByText(/기다린다\./)).toBeInTheDocument()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
    expect(article().getByRole('button', { name: '제주' })).toBeInTheDocument()
  })

  // An open editor is plain fields: no mark stands in the block it edits. The other half of the
  // page's old case — a source edited away drops its mark — is visibleSpans' rule, pinned in
  // entities/post/model/replacements.test.ts.
  it('shows no mark while a block editor is open', async () => {
    const user = userEvent.setup()
    await renderMarks()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    expect(article().queryByRole('button', { name: '비가' })).toBeNull()
    expect(article().queryByRole('button', { name: '기다렸다' })).toBeNull()
  })

  // Opening and closing a mark is no edit (POST-79): nothing is queued, so nothing goes out.
  it('sends nothing for an ignored mark', async () => {
    const user = userEvent.setup()
    const { calls } = await renderMarks()

    await user.click(article().getByRole('button', { name: '제주' }))
    expect(await screen.findByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeNull()
    expect(screen.getByText('저장됨')).toBeInTheDocument()
    expect(calls).not.toContain('SavePostContent')
  })
})
