import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BlockType, PostContentSchema, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { finalizedPostRow } from '@/test/posts'
import { openTemplateDirect } from '@/test/authoring-ui'
import type { FakeAuthoringOptions } from '@/test/authoring'
const source = finalizedPostRow({
  slug: 'owned',
  title: 'private title',
  memo: 'private memo',
  content: create(PostContentSchema, {
    title: 'private title',
    tags: ['private tag'],
    summary: 'private summary',
    blocks: [
      { type: BlockType.HEADING, content: '첫인상과 공간' },
      { type: BlockType.TEXT, content: 'private body'.repeat(2000) },
      { type: BlockType.IMAGE, file: 'private.jpg', caption: 'private caption' },
      { type: BlockType.HEADING, content: '아쉬움과 추천' },
    ],
  }),
})
const providers = {
  models: [{ providerId: 'stub', modelId: 'writer', label: '작가AI', stages: [Stage.WRITE] }],
  selections: [{ stage: Stage.WRITE, providerId: 'stub', modelId: 'writer' }],
}
// 이 글 형식으로 템플릿 만들기 (TMPL-64): the post is handed to the conversation as its one bounded
// reference text (EDIT-16) — its headings marked — and none of it is shown on the screen.
describe('template creation from a post', () => {
  it('hands the post to the conversation as its reference, showing none of it', async () => {
    const calls: string[] = []
    const creates: NonNullable<FakeAuthoringOptions['creates']> = []
    const user = userEvent.setup()
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      calls,
      posts: { posts: [source] },
      authoring: { creates },
      providers,
    })
    await screen.findByRole('log', { name: '템플릿 대화' })
    for (const text of [
      'private title',
      'private memo',
      'private body',
      'private caption',
      'private summary',
      'private tag',
      '첫인상과 공간',
    ])
      expect(screen.queryByText(new RegExp(text))).not.toBeInTheDocument()
    expect(calls.filter((name) => /Start/.test(name))).toEqual([])
    await user.type(screen.getByRole('textbox', { name: '무엇을 올리는지' }), '이 글처럼 써 줘')
    const send = screen.getByRole('button', { name: '초안 만들기' })
    await waitFor(() => expect(send).toBeEnabled())
    await user.click(send)
    // The session (and its reference) is made when the owner confirms the quoted draft.
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '이대로 요청하기' }))
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0].referencePost).toContain('## 첫인상과 공간')
    // Bounded: this post's 24,000-character body is cut to the reference limit.
    expect(Array.from(creates[0].referencePost)).toHaveLength(12000)
  })
  it('opens the conversation without a reference when the post cannot be read', async () => {
    renderAppAt('/templates/new?from=missing', { user: { id: 'alice' } })
    expect(await screen.findByRole('log', { name: '템플릿 대화' })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: '어떤 체험을 글로 남기나요?' }),
    ).not.toBeInTheDocument()
  })
  it('never exposes the retired request/correction/undo form in saved direct editing', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/saved', {
      user: { id: 'alice' },
      templates: { templates: [{ id: 'saved', name: '내 구성', body: '<write>인트로</write>' }] },
    })
    await screen.findByRole('log', { name: '템플릿 대화' })
    await openTemplateDirect(user)
    for (const name of ['AI에게 요청', '요청 보내기', '요청 전으로 되돌리기'])
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
  })
})
