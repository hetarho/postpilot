import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { usePost } from '@/entities/post'
import {
  BlockSchema,
  BlockType,
  GalleryLayout,
  PostContentSchema,
  type PostContent,
} from '@/shared/api'
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
  const { post, refetch } = usePost(slug)
  return (
    <>
      <button onClick={refetch}>서버 내용 다시 읽기</button>
      {post?.content ? (
        <BlockEditor key={`${post.slug}:${post.machineBaselineRevision}`} post={post} />
      ) : null}
    </>
  )
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

  // POST-55.
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

  // POST-55: cancel restores the value the block had when its editor opened.
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

  // The editing UI keeps every capability it had.
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

  it('saves an edit against the new revision after an AI result remounts the editor', async () => {
    const user = userEvent.setup()
    const contentSaves: Array<{
      slug: string
      expectedRevision: bigint
      content: typeof POST_CONTENT_FIXTURE
    }> = []
    const first: FakePostRow = {
      ...reviewPost,
      slug: '20261002-ai-rewrite',
    }
    const rewritten: FakePostRow = {
      ...first,
      content: {
        ...POST_CONTENT_FIXTURE,
        blocks: POST_CONTENT_FIXTURE.blocks.map((block, index) =>
          index === 0 ? { ...block, content: 'AI가 다시 쓴 문단' } : block,
        ),
      },
      contentRevision: 2n,
      machineBaselineRevision: 2n,
    }
    const transport = createFakePostsTransport({
      posts: [first],
      getSequence: [first, rewritten],
      contentSaves,
    })
    render(<Editor slug={first.slug} />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await screen.findByText(POST_CONTENT_FIXTURE.blocks[0].content)

    await user.click(screen.getByRole('button', { name: '서버 내용 다시 읽기' }))
    await screen.findByText('AI가 다시 쓴 문단')
    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '내가 고친 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(contentSaves).toHaveLength(1), AUTOSAVED)
    expect(contentSaves[0]?.expectedRevision).toBe(2n)
    expect(contentSaves[0]?.content.blocks[0].content).toBe('내가 고친 문단')
    expect(screen.queryByText(/다른 화면에서 글이 바뀌었어요/)).not.toBeInTheDocument()
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

// POST-106: a photo group is one card — made from the type picker, its layout, photos, alt and
// caption edited in place, and turned back into a single photo — and the editor refuses what the
// server refuses.
describe('a photo group', () => {
  const third = { ...POST_IMAGES_FIXTURE[0]!, id: 'image-3', filename: 'IMG_3.jpg' }
  const photoBlockIndex = POST_CONTENT_FIXTURE.blocks.findIndex(
    (block) => block.type === BlockType.IMAGE,
  )

  function renderWithSaves(row: FakePostRow) {
    const contentSaves: Array<{ slug: string; expectedRevision: bigint; content: PostContent }> = []
    const transport = createFakePostsTransport({ calls: [], posts: [row], contentSaves })
    render(<Editor slug={row.slug} />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    return contentSaves
  }

  const row = (images = [...POST_IMAGES_FIXTURE, third], content = POST_CONTENT_FIXTURE) =>
    ({
      slug: '20260820-group',
      status: 'review',
      content,
      images,
      contentRevision: 1n,
      machineBaselineRevision: 1n,
    }) satisfies FakePostRow

  async function openTypePicker(user: ReturnType<typeof userEvent.setup>, index: number) {
    await user.click(await screen.findByRole('button', { name: `${index + 1}번째 블록 수정` }))
    await user.click(screen.getByLabelText(`${index + 1}번째 블록 종류`))
  }

  it('is offered only when the post has two or more photos', async () => {
    const user = userEvent.setup()
    renderEditor(row([POST_IMAGES_FIXTURE[0]!]))
    await openTypePicker(user, 0)
    expect(screen.getByRole('option', { name: '사진' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: '사진 묶음' })).not.toBeInTheDocument()
    cleanup()
    discardContentQueues()

    renderEditor(row())
    await openTypePicker(user, 0)
    expect(screen.getByRole('option', { name: '사진 묶음' })).toBeInTheDocument()
  })

  it('turns a photo into a group that keeps it first, edits it, and turns it back', async () => {
    const user = userEvent.setup()
    const saves = renderWithSaves(row())
    await openTypePicker(user, photoBlockIndex)
    await user.click(screen.getByRole('option', { name: '사진 묶음' }))

    // The photo stays first and the next attached photo joins it; the caption is kept.
    const remove = (filename: string) => screen.getByRole('button', { name: `${filename} 빼기` })
    expect(remove('IMG_1.jpg')).toBeDisabled()
    expect(remove('IMG_2.jpg')).toBeDisabled()
    expect(screen.getByPlaceholderText('캡션 (선택)')).toHaveValue('비 뒤의 바다')

    await user.click(screen.getByLabelText('사진 추가'))
    await user.click(screen.getByRole('option', { name: 'IMG_3.jpg' }))
    expect(remove('IMG_2.jpg')).toBeEnabled()
    expect(screen.getByLabelText('사진 추가')).toBeDisabled()
    await user.click(remove('IMG_2.jpg'))
    await user.click(screen.getByRole('tab', { name: '슬라이드' }))
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => {
      const block = saves.at(-1)?.content.blocks[photoBlockIndex]
      expect(block?.type).toBe(BlockType.GALLERY)
      expect(block?.files).toEqual(['IMG_1.jpg', 'IMG_3.jpg'])
      expect(block?.layout).toBe(GalleryLayout.SLIDE)
      expect(block?.caption).toBe('비 뒤의 바다')
    }, AUTOSAVED)

    await openTypePicker(user, photoBlockIndex)
    await user.click(screen.getByRole('option', { name: '사진' }))
    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => {
      const block = saves.at(-1)?.content.blocks[photoBlockIndex]
      expect(block?.type).toBe(BlockType.IMAGE)
      expect(block?.file).toBe('IMG_1.jpg')
      expect(block?.caption).toBe('비 뒤의 바다')
    }, AUTOSAVED)
  })

  it('counts every detached photo of a group before finalize and saves nothing', async () => {
    const content = create(PostContentSchema, {
      title: '묶음',
      blocks: [
        create(BlockSchema, { type: BlockType.TEXT, content: '도착' }),
        create(BlockSchema, {
          type: BlockType.GALLERY,
          files: ['IMG_1.jpg', 'gone.jpg'],
          layout: GalleryLayout.COLLAGE,
        }),
        create(BlockSchema, { type: BlockType.IMAGE, file: 'gone-too.jpg' }),
      ],
    })
    const saves = renderWithSaves(row(POST_IMAGES_FIXTURE, content))
    expect(await screen.findByText(/사진이 없는 자리가 2곳/)).toBeInTheDocument()
    expect(saves).toHaveLength(0)
  })
})
