import { afterEach, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { rememberPostEntry, readPostReturnContext } from '@/entities/post'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'
import { renderAppAt } from '@/test/app'
import { USER, resetEditorTest, openBrief } from '@/test/editor'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'

afterEach(() => {
  resetEditorTest()
  clearCaret()
  sessionStorage.clear()
})

it('keeps the creation parent after the first mint and after reload, then deletes safely to home', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const first = renderAppAt('/posts/new', { user: USER, calls })
  await user.type(await screen.findByLabelText('제목'), '새 여행')
  await waitFor(
    () => expect(first.router.state.location.pathname).toBe('/posts/20260828-새-여행'),
    { timeout: 4000 },
  )
  expect(screen.getByRole('link', { name: '만들기로 돌아가기' })).toHaveAttribute('href', '/')
  expect(readPostReturnContext(USER.id, '20260828-새-여행')?.path).toBe('/')
  first.unmount()
  const restored = renderAppAt('/posts/20260828-새-여행', { transport: first.transport })
  await user.click(await screen.findByRole('button', { name: '글 삭제하기' }))
  await user.click(screen.getByRole('button', { name: '삭제', hidden: true }))
  await waitFor(() => expect(calls).toContain('DeletePost'))
  await waitFor(() => expect(restored.router.state.location.pathname).toBe('/'), { timeout: 5000 })
})

// A post opened from filtered history lands on the step its status gives (POST-44) and returns to
// that same filtered history.
it('opens on its status step and retains the actual filtered history parent', async () => {
  rememberPostEntry(USER.id, {
    path: '/posts',
    section: 'posts',
    filters: { q: '제주', status: 'review' },
    scrollY: 840,
    targetId: 'saved',
  })
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    posts: { posts: [{ slug: 'saved', status: 'review', content: POST_CONTENT_FIXTURE }] },
  })
  expect(await screen.findByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  const back = screen.getByRole('link', { name: '작업 내역으로 돌아가기' })
  expect(back).toHaveAttribute('href', '/posts?q=%EC%A0%9C%EC%A3%BC&status=review')
  await userEvent.click(back)
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
  expect(router.state.location.search).toMatchObject({ q: '제주', status: 'review' })
})

it('flushes the newest memo before contextual return without starting AI', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const draftMaterials: Array<{ slug: string; title: string; memo: string }> = []
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    calls,
    posts: { posts: [{ slug: 'saved' }], draftMaterials },
  })
  await user.type(await screen.findByLabelText('메모'), '떠나기 직전 입력')
  await user.click(screen.getByRole('link', { name: '작업 내역으로 돌아가기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
  expect(draftMaterials.at(-1)?.memo).toBe('떠나기 직전 입력')
  expect(calls).not.toContain('StartGeneration')
})

it('mints an unsaved draft once and opens tests with its newest material and retained creation origin', async () => {
  const user = userEvent.setup()
  const draftMaterials: Array<{ slug: string; title: string; memo: string }> = []
  const { router } = renderAppAt('/posts/new', { user: USER, posts: { draftMaterials } })
  await screen.findByLabelText('제목')
  const brief = await openBrief(user)
  fireEvent.change(screen.getByLabelText('제목'), { target: { value: '테스트 초안' } })
  fireEvent.change(screen.getByLabelText('메모'), { target: { value: '방금 쓴 초안 재료' } })
  await user.click(within(brief).getByRole('link', { name: '글 설정 비교 테스트' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/tests'), { timeout: 5000 })
  expect(router.state.location.search).toMatchObject({ sourcePost: '20260828-테스트-초안' })
  expect(draftMaterials).toEqual([{ slug: '', title: '테스트 초안', memo: '방금 쓴 초안 재료' }])
  expect(readPostReturnContext(USER.id, '20260828-테스트-초안')?.path).toBe('/')
})
