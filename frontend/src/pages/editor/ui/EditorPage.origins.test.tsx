import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { afterEach, expect, it } from 'vitest'
import { usePost } from '@/entities/post'
import { discardContentQueues, type BlockEditorHandle } from '@/features/edit-post-content'
import {
  BlockType,
  PostContentSchema,
  GetPostResponseSchema,
  OriginFieldKind,
  OriginReviewState,
  PostSchema,
  PostService,
  SavePostContentResponseSchema,
  SemanticOriginCategory,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { EditorRefinePanel } from './EditorRefinePanel'

afterEach(() => {
  cleanup()
  discardContentQueues()
})

function setup() {
  let row = create(PostSchema, {
    slug: 'origins',
    status: 'review',
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    contentHash: 'first',
    targetLanguage: 1,
    content: {
      title: '서울 😀',
      summary: '내가 간 카페',
      tags: ['카페'],
      blocks: [{ type: BlockType.TEXT, content: '서울 😀에서 서울 😀를 만났어요.' }],
    },
    contentOrigins: {
      version: 1,
      result: { contentRevision: 1n, contentHash: 'first' },
      sources: [{ id: 'memo', kind: 'memo', text: '서울에 갔어요.', available: true }],
      spans: [
        {
          field: { kind: OriginFieldKind.BLOCK_CONTENT, blockIndex: 0 },
          start: 0,
          end: 4,
          quote: '서울 😀',
          category: SemanticOriginCategory.OWNER_INPUT,
          sourceRefs: ['memo'],
          reviewState: OriginReviewState.UNREVIEWED,
        },
      ],
    },
  })
  let finish!: () => void
  const waiting = new Promise<void>((resolve) => {
    finish = resolve
  })
  const calls: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.getPost, () => create(GetPostResponseSchema, { post: row }))
    rpc(PostService.method.savePostContent, async (request) => {
      calls.push('save')
      await waiting
      row = create(PostSchema, {
        ...row,
        content: request.content,
        contentRevision: 2n,
        contentHash: 'saved',
        contentOrigins: undefined,
      })
      return create(SavePostContentResponseSchema, { post: row })
    })
  })
  const client = createTestQueryClient()
  const editorRef = createRef<BlockEditorHandle>()
  function Harness() {
    const { post, refetch } = usePost('origins')
    return post?.content ? (
      <>
        <button onClick={refetch}>새 결과 읽기</button>
        <EditorRefinePanel
          post={post}
          ownerId="alice"
          result={post.content}
          editorRef={editorRef}
          onContentChange={() => undefined}
          onGoGenerate={() => undefined}
        />
      </>
    ) : null
  }
  render(<Harness />, { wrapper: withProviders(transport, client) })
  return {
    calls,
    finish: () => finish(),
    replace: () => {
      row = create(PostSchema, {
        ...row,
        content: create(PostContentSchema, {
          title: row.content?.title,
          summary: row.content?.summary,
          tags: row.content?.tags,
          blocks: [{ type: BlockType.TEXT, content: '새 AI 결과' }],
        }),
        contentRevision: 3n,
        machineBaselineRevision: 3n,
        contentHash: 'replacement',
        contentOrigins: undefined,
      })
    },
  }
}

it('keeps the same Korean/emoji input and caret through origin toggle, composition and an older save', async () => {
  const user = userEvent.setup()
  const harness = setup()
  await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
  const input = screen.getByRole('textbox', { name: '1번째 블록 내용' }) as HTMLTextAreaElement
  input.focus()
  input.setSelectionRange(5, 5)
  fireEvent.compositionStart(input)
  fireEvent.change(input, { target: { value: '서울 😀에 입력 중' } })
  fireEvent.compositionEnd(input)
  input.setSelectionRange(7, 7)
  const toggle = screen.getByRole('checkbox', { name: '출처 보기' })
  await user.click(toggle)
  expect(screen.getByRole('textbox', { name: '1번째 블록 내용' })).toBe(input)
  expect(input.value).toBe('서울 😀에 입력 중')
  expect(input.selectionStart).toBe(7)
  await user.click(toggle)
  // Current pending text remains inspectable, with no old confirmed source color.
  const draft = screen.getByRole('article', { name: '생성된 글' })
  expect(draft.querySelector('.text-origin-owner-foreground')).toBeNull()
  await waitFor(() => expect(harness.calls).toEqual(['save']), { timeout: 4000 })
  fireEvent.change(input, { target: { value: '더 최신 입력 😀' } })
  input.setSelectionRange(3, 3)
  harness.finish()
  await waitFor(() => expect(screen.getByText('저장 대기 중')).toBeInTheDocument())
  expect(screen.getByRole('textbox', { name: '1번째 블록 내용' })).toBe(input)
  expect(input.value).toBe('더 최신 입력 😀')
  expect(input.selectionStart).toBe(3)
  expect(draft.querySelector('.text-origin-owner-foreground')).toBeNull()
})

it('restores the known cancel snapshot and remounts only for a new AI baseline', async () => {
  const user = userEvent.setup()
  setup()
  const draft = await screen.findByRole('article', { name: '생성된 글' })
  expect(draft.querySelector('.text-origin-owner-foreground')).not.toBeNull()
  await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
  await user.clear(screen.getByRole('textbox', { name: '1번째 블록 내용' }))
  await user.type(screen.getByRole('textbox', { name: '1번째 블록 내용' }), '바꾼 문구')
  expect(draft.querySelector('.text-origin-owner-foreground')).toBeNull()
  await user.click(screen.getByRole('button', { name: '취소' }))
  expect(
    within(draft).getByRole('button', { name: '직접 입력 기반 출처 보기: 서울 😀' }),
  ).toBeInTheDocument()
  expect(draft.querySelector('.text-origin-owner-foreground')).not.toBeNull()
  cleanup()
  discardContentQueues()
  const fresh = setup()
  await screen.findByRole('article', { name: '생성된 글' })
  fresh.replace()
  await user.click(screen.getByRole('button', { name: '새 결과 읽기' }))
  expect(await screen.findByText('새 AI 결과')).toBeInTheDocument()
  expect(screen.queryByText('서울 😀를 만났어요.', { exact: false })).not.toBeInTheDocument()
  expect(
    screen
      .getByRole('article', { name: '생성된 글' })
      .querySelector('.text-origin-owner-foreground'),
  ).toBeNull()
})
