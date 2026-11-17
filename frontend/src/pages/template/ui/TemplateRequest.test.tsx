import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BlockType, PostContentSchema } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { finalizedPostRow } from '@/test/posts'
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
describe('template creation from a post', () => {
  // Guide: a template is made by talking, so 이 글 형식으로 템플릿 만들기 opens the conversation
  // with the post attached as its reference — no wizard, and nothing is sent until the owner asks.
  it('opens the conversation for a post-based template without sending anything', async () => {
    const calls: string[] = []
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      calls,
      posts: { posts: [source] },
    })
    expect(await screen.findByText(/어떤 글을 올리세요\?/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^맛집/ })).not.toBeInTheDocument()
    expect(calls.filter((call) => /^(Start|Create|SaveAuthoring)/.test(call))).toEqual([])
  })
  it('never exposes the retired request/correction/undo form in saved direct editing', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/saved', {
      user: { id: 'alice' },
      templates: { templates: [{ id: 'saved', name: '내 구성', body: '<write>인트로</write>' }] },
    })
    await user.click(await screen.findByRole('button', { name: '직접 고치기' }))
    await screen.findByLabelText('이름')
    for (const name of ['AI에게 요청', '요청 보내기', '요청 전으로 되돌리기'])
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
  })
})
