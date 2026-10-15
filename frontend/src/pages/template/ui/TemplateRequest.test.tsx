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
async function target(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: /^맛집/ }))
  await user.click(await screen.findByRole('button', { name: /^식사/ }))
  await screen.findByRole('heading', { name: '이런 글은 보통 이렇게 구성돼요' })
}
describe('template creation from post headings', () => {
  it('seeds the wizard with the post’s headings', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      calls,
      posts: { posts: [source] },
    })
    await target(user)
    const boxes = screen.getAllByRole('checkbox')
    expect(boxes.slice(-2).map((box) => box.closest('label')?.textContent)).toEqual([
      '첫인상과 공간',
      '아쉬움과 추천',
    ])
    expect(boxes.slice(-2).every((box) => (box as HTMLInputElement).checked)).toBe(true)
    for (const text of [
      'private title',
      'private memo',
      'private body',
      'private caption',
      'private summary',
      'private tag',
    ])
      expect(screen.queryByText(new RegExp(text))).not.toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: '이전' })[0]!)
    await user.click(await screen.findByRole('button', { name: /^식사/ }))
    expect(screen.getAllByRole('checkbox', { name: '첫인상과 공간' })).toHaveLength(1)
    expect(calls.filter((name) => /Create|Start/.test(name))).toEqual([])
  })
  it('offers the target roles alone when the post has no headings', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      posts: {
        posts: [
          {
            ...source,
            content: create(PostContentSchema, {
              title: 'private title',
              blocks: [{ type: BlockType.TEXT, content: 'private body' }],
            }),
          },
        ],
      },
    })
    await target(user)
    expect(screen.getAllByRole('checkbox')).toHaveLength(11)
    expect(screen.queryByRole('button', { name: /부분 빼기/ })).not.toBeInTheDocument()
  })
  it('keeps an unavailable post out and offers a fresh wizard', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/new?from=missing', { user: { id: 'alice' } })
    expect(await screen.findByText(/참고 글을 읽지 못했어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '참고 글 빼기' }))
    expect(
      await screen.findByRole('heading', { name: '어떤 체험을 글로 남기나요?' }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'AI와 템플릿 만들기' })).not.toBeInTheDocument()
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
