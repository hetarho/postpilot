import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BlockType, PostContentSchema } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { finalizedPostRow } from '@/test/posts'
import type { FakeAuthoringOptions } from '@/test/authoring'
const source = finalizedPostRow({
  slug: 'owned',
  title: '저장 제목',
  memo: 'private memo',
  content: create(PostContentSchema, {
    title: '선택 제목',
    tags: ['private tag'],
    summary: 'private summary',
    blocks: [
      { type: BlockType.TEXT, content: '방문 순서' },
      { type: BlockType.IMAGE, file: 'private.jpg', caption: 'private caption' },
    ],
  }),
})
describe('explicit template reference and retired helper surface', () => {
  it('opening from an owned post starts no model work and explicit direct creation captures only selected form material', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const creates: NonNullable<FakeAuthoringOptions['creates']> = []
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      calls,
      posts: { posts: [source] },
      authoring: { creates },
    })
    expect(await screen.findByText('참고 글: 선택 제목')).toBeInTheDocument()
    expect(calls).not.toContain('CreateAuthoringSession')
    expect(screen.queryByRole('region', { name: 'AI에게 템플릿 요청' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '직접 편집' }))
    await screen.findByLabelText('이름')
    expect(creates[0].referencePost).toBe('선택 제목\n\n방문 순서\n\n[photo position]')
    expect(calls).not.toContain('StartAuthoringOperation')
    expect(calls).not.toContain('StartTemplateRequest')
  })
  it('removing a post reference permits ordinary seed-free creation', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeAuthoringOptions['creates']> = []
    renderAppAt('/templates/new?from=owned', {
      user: { id: 'alice' },
      posts: { posts: [source] },
      authoring: { creates },
    })
    await user.click(await screen.findByRole('button', { name: '참고 글 빼기' }))
    expect(screen.queryByText('참고 글: 선택 제목')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '직접 편집' }))
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0].referencePost).toBe('')
  })
  it('reports unavailable reference content and retains the seed-free exit', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/new?from=missing', { user: { id: 'alice' } })
    expect(await screen.findByText(/참고 글을 읽지 못했어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '참고 글 빼기' }))
    expect(screen.getByRole('button', { name: 'AI와 템플릿 만들기' })).toBeEnabled()
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
