import { afterEach, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { rememberPostEntry, readPostReturnContext } from '@/entities/post'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'
import { renderAppAt } from '@/test/app'
import { GUIDE, USER, guideTo, resetEditorTest, openBrief } from '@/test/editor'
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
  expect(screen.getByRole('link', { name: '네이버 블로그로 돌아가기' })).toHaveAttribute(
    'href',
    '/naver-blog',
  )
  expect(readPostReturnContext(USER.id, '20260828-새-여행')?.path).toBe('/naver-blog')
  first.unmount()
  const restored = renderAppAt('/posts/20260828-새-여행', { transport: first.transport })
  await user.click(await screen.findByRole('button', { name: '글 삭제하기' }))
  await user.click(screen.getByRole('button', { name: '삭제', hidden: true }))
  await waitFor(() => expect(calls).toContain('DeletePost'))
  await waitFor(() => expect(restored.router.state.location.pathname).toBe('/naver-blog'), {
    timeout: 5000,
  })
})

// A post opened from filtered history lands on the step its status gives (POST-44) and returns to
// that same filtered history.
it('opens on its status step and retains the actual filtered history parent', async () => {
  rememberPostEntry(USER.id, {
    path: '/naver-blog',
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
  const back = screen.getByRole('link', { name: '네이버 블로그로 돌아가기' })
  expect(back).toHaveAttribute('href', '/naver-blog?q=%EC%A0%9C%EC%A3%BC&status=review')
  await userEvent.click(back)
  await waitFor(() => expect(router.state.location.pathname).toBe('/naver-blog'))
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
  await user.click(screen.getByRole('link', { name: '네이버 블로그로 돌아가기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/naver-blog'))
  expect(draftMaterials.at(-1)?.memo).toBe('떠나기 직전 입력')
  expect(calls).not.toContain('StartGeneration')
})

it('keeps the latest memo and its status when the header return cannot flush', async () => {
  const calls: string[] = []
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    calls,
    posts: { posts: [{ slug: 'saved' }], failSaves: 20 },
  })
  const memo = await screen.findByLabelText('메모')
  fireEvent.change(memo, { target: { value: '아직 저장하지 못한 마지막 메모' } })
  await userEvent.click(screen.getByRole('link', { name: '네이버 블로그로 돌아가기' }))
  await waitFor(() =>
    expect(screen.getByRole('status', { name: '글 상태' })).toHaveTextContent('저장하지 못했어요'),
  )
  expect(router.state.location.pathname).toBe('/posts/saved')
  expect(memo).toHaveValue('아직 저장하지 못한 마지막 메모')
  expect(calls).toContain('SavePostDraft')
  expect(calls.some((call) => /^(Start|Generate|Cancel|Publish)/.test(call))).toBe(false)
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
  expect(readPostReturnContext(USER.id, '20260828-테스트-초안')?.path).toBe('/naver-blog')
})

it('preserves channel scroll through first draft mount and address minting', async () => {
  rememberPostEntry(USER.id, {
    path: '/naver-blog',
    section: 'naverBlog',
    filters: {},
    scrollY: 480,
  })
  const view = renderAppAt('/posts/new', { user: USER })
  await userEvent.type(await screen.findByLabelText('제목'), 'Channel draft')
  await waitFor(() => expect(view.router.state.location.pathname).not.toBe('/posts/new'), {
    timeout: 5000,
  })
  const slug = view.router.state.location.pathname.split('/').at(-1)!
  expect(readPostReturnContext(USER.id, slug)).toMatchObject({ path: '/naver-blog', scrollY: 480 })
})

// From the template question, 새 템플릿 만들기 left the owner on a wizard whose two back links both
// led to the template list, and a return to the post opened on the summary instead of the
// question they had left.
it('returns from the template wizard to the template question of the post', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    posts: { posts: [{ slug: 'saved', memo: '바다를 보고 왔다' }] },
    templates: { templates: [] },
  })
  await guideTo(user, GUIDE.template)
  await user.click(screen.getByRole('button', { name: '새 템플릿 만들기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/templates/new'))

  const back = await screen.findByRole('link', { name: '글 작성' })
  expect(back).toHaveAttribute('href', '/posts/saved')
  expect(screen.queryByRole('link', { name: /템플릿/ })).not.toBeInTheDocument()
  await user.click(back)
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/saved'))
  expect(await screen.findByRole('heading', { name: '어떤 틀로 써 볼까요?' })).toBeVisible()
})

// POST-123 →THEME-58: 확인's 지침 row opens the guidelines page with the post as its one way back,
// and coming back — by that link or the browser's Back — lands on 확인 again. The post holds
// nothing that would otherwise open it on 확인.
it('returns from the guidelines page to the post, on 확인', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    posts: { posts: [{ slug: 'saved' }] },
  })
  expect(await screen.findByRole('heading', { name: '어떤 글을 쓰시나요?' })).toBeVisible()
  await guideTo(user, GUIDE.review)
  await user.click(await screen.findByRole('button', { name: '지침 바꾸기' }))
  await waitFor(() => expect(router.state.location.href).toBe('/guidelines?post=saved'))

  const main = await screen.findByRole('main')
  const back = await screen.findByRole('link', { name: '글 작성' })
  expect(back).toHaveAttribute('href', '/posts/saved')
  // It heads the page, over the hub's title and chips, and it is the page's only way back.
  expect(back.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(screen.getAllByRole('navigation', { name: '돌아가기' })).toHaveLength(1)
  await user.click(back)
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/saved'))
  expect(await screen.findByRole('heading', { name: '이대로 써 볼까요?' })).toBeVisible()

  await user.click(await screen.findByRole('button', { name: '지침 바꾸기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/guidelines'))
  router.history.back()
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/saved'))
  expect(await screen.findByRole('heading', { name: '이대로 써 볼까요?' })).toBeVisible()
})

// The guidelines page reached from its own tab is a hub section with no way back (THEME-58).
it('shows no back link on the guidelines page opened from its tab', async () => {
  renderAppAt('/guidelines', { user: USER })
  await screen.findByRole('main')
  expect(screen.queryByRole('link', { name: '글 작성' })).not.toBeInTheDocument()
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).not.toBeInTheDocument()
})

// 내용 opened on the title as bare display-size text with its name hidden: a second, larger
// heading under the screen's question, with nothing saying it could be typed in.
it('draws the title on 내용 as a named field like the answers under it', async () => {
  const user = userEvent.setup()
  renderAppAt('/posts/saved', {
    user: USER,
    posts: { posts: [{ slug: 'saved', title: '' }] },
  })
  await guideTo(user, GUIDE.details)

  const title = await screen.findByRole('textbox', { name: '제목' })
  expect(title).toHaveAttribute('placeholder', '예: 손님이 다시 찾는 이유')
  expect(title).toHaveClass('bg-field-bg')
  expect(title).not.toHaveClass('text-3xl')
  const label = document.querySelector('label[for="post-title"]')
  expect(label).toHaveTextContent('제목')
  expect(label).not.toHaveClass('sr-only')
})
