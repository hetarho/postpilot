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
describe('template creation from a post (TMPL-64)', () => {
  it('opens the AI conversation without showing or sending anything from the post yet', async () => {
    const calls: string[] = []
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      calls,
      posts: { posts: [source] },
    })
    expect(
      await screen.findByRole('textbox', { name: '어떤 글인지 말해 주세요' }),
    ).toBeInTheDocument()
    for (const text of [
      'private memo',
      'private body',
      'private caption',
      'private summary',
      'private tag',
    ])
      expect(screen.queryByText(new RegExp(text))).not.toBeInTheDocument()
    // Opening the conversation creates no session and starts no AI work: that waits for 초안 만들기
    // and its estimate.
    expect(calls.filter((name) => /Create|Start/.test(name))).toEqual([])
  })
  it('keeps an unavailable post out and offers the conversation without it', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/new?from=missing', { user: { id: 'alice' } })
    expect(await screen.findByText(/참고 글을 읽지 못했어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '참고 글 빼기' }))
    expect(
      await screen.findByRole('textbox', { name: '어떤 글인지 말해 주세요' }),
    ).toBeInTheDocument()
  })
  it('never exposes the retired request/correction/undo form in saved direct editing', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/saved', {
      user: { id: 'alice' },
      templates: { templates: [{ id: 'saved', name: '내 구성', body: '<write>인트로</write>' }] },
    })
    await user.click(await screen.findByRole('button', { name: /직접 편집하기$/ }))
    await screen.findByLabelText('이름')
    for (const name of ['AI에게 요청', '요청 보내기', '요청 전으로 되돌리기'])
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
  })
})
