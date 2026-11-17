import { afterEach, expect, it } from 'vitest'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
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
  expect(screen.getByRole('link', { name: '글로 돌아가기' })).toHaveAttribute('href', '/posts')
  expect(readPostReturnContext(USER.id, '20260828-새-여행')?.path).toBe('/posts')
  first.unmount()
  const restored = renderAppAt('/posts/20260828-새-여행', { transport: first.transport })
  await user.click(await screen.findByRole('button', { name: '글 삭제하기' }))
  await user.click(screen.getByRole('button', { name: '삭제', hidden: true }))
  await waitFor(() => expect(calls).toContain('DeletePost'))
  await waitFor(() => expect(restored.router.state.location.pathname).toBe('/posts'), {
    timeout: 5000,
  })
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
  expect(await screen.findByRole('tab', { name: '다듬기' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  const back = screen.getByRole('link', { name: '글로 돌아가기' })
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
  await user.click(screen.getByRole('link', { name: '글로 돌아가기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
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
  await userEvent.click(screen.getByRole('link', { name: '글로 돌아가기' }))
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
  expect(readPostReturnContext(USER.id, '20260828-테스트-초안')?.path).toBe('/posts')
})

it('preserves channel scroll through first draft mount and address minting', async () => {
  rememberPostEntry(USER.id, {
    path: '/posts',
    section: 'post',
    filters: {},
    scrollY: 480,
  })
  const view = renderAppAt('/posts/new', { user: USER })
  await userEvent.type(await screen.findByLabelText('제목'), 'Channel draft')
  await waitFor(() => expect(view.router.state.location.pathname).not.toBe('/posts/new'), {
    timeout: 5000,
  })
  const slug = view.router.state.location.pathname.split('/').at(-1)!
  expect(readPostReturnContext(USER.id, slug)).toMatchObject({ path: '/posts', scrollY: 480 })
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

it('changes account guidelines over the saved writing review without leaving the draft', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    calls,
    posts: { posts: [{ slug: 'saved', title: '작성 중인 글', memo: '남겨 둔 경험' }] },
    guidelines: {
      defaults: [{ key: 'gentle', name: '반복 줄이기', text: '같은 문장을 반복하지 않아요.' }],
    },
  })
  await guideTo(user, GUIDE.review)
  const opener = await screen.findByRole('button', { name: '지침 바꾸기' })
  await user.click(opener)
  const sheet = await screen.findByRole('dialog', { name: '지침' })
  expect(router.state.location.pathname).toBe('/posts/saved')
  await user.click(await within(sheet).findByRole('button', { name: /반복 줄이기/ }))
  await user.click(within(sheet).getByRole('button', { name: '반복 줄이기 적용 안함' }))
  await waitFor(() => expect(calls).toContain('SetDefaultGuidelineEnabled'))
  expect(router.state.location.pathname).toBe('/posts/saved')
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog', { name: '지침' })).toBeNull()
  expect(opener).toHaveFocus()
  expect(screen.getByLabelText('제목')).toHaveValue('작성 중인 글')
  expect(screen.getByLabelText('메모')).toHaveValue('남겨 둔 경험')
  expect(screen.getByRole('heading', { name: '이대로 써 볼까요?' })).toBeVisible()
})

it('opens guidelines for an empty unsaved draft without creating a post or starting AI', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const { router } = renderAppAt('/posts/new', { user: USER, calls })
  await guideTo(user, GUIDE.review)
  await user.click(await screen.findByRole('button', { name: '지침 바꾸기' }))
  const sheet = await screen.findByRole('dialog', { name: '지침' })
  await within(sheet).findByRole('button', { name: '기본 지침' })
  expect(router.state.location.pathname).toBe('/posts/new')
  await user.click(within(sheet).getByRole('button', { name: '닫기' }))
  expect(screen.queryByRole('dialog', { name: '지침' })).toBeNull()
  expect(router.state.location.pathname).toBe('/posts/new')
  expect(screen.getByRole('heading', { name: '이대로 써 볼까요?' })).toBeVisible()
  expect(calls).not.toContain('SavePostDraft')
  expect(calls.filter((call) => /^(Start|Analyze|Generate|Create)/.test(call))).toEqual([])
})

it('opens guidelines after a pending first save and restores focus to the saved editor', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const { router } = renderAppAt('/posts/new', { user: USER, calls })
  fireEvent.change(await screen.findByLabelText('제목'), {
    target: { value: '지침을 고치는 동안' },
  })
  await guideTo(user, GUIDE.review)
  await user.click(await screen.findByRole('button', { name: '지침 바꾸기' }))
  const sheet = await screen.findByRole('dialog', { name: '지침' })
  await waitFor(() => expect(calls).toContain('SavePostDraft'))
  expect(sheet).toBeVisible()
  expect(router.state.location.pathname).toBe('/posts/20260828-지침을-고치는-동안')
  const opener = screen.getByRole('button', { name: '지침 바꾸기' })
  await user.click(within(sheet).getByRole('button', { name: '닫기' }))
  await waitFor(() => expect(opener).toHaveFocus())
  expect(await screen.findByRole('heading', { name: '이대로 써 볼까요?' })).toBeVisible()
  expect(screen.getByLabelText('제목')).toHaveValue('지침을 고치는 동안')
})

it('returns from a guideline source link to the saved draft instead of a fresh new editor', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts/new', {
    user: USER,
    posts: { posts: [{ slug: 'source', memo: '기존 글의 경험' }] },
    guidelines: { candidates: [{ id: 'candidate', text: '과장하지 않기', postSlug: 'source' }] },
  })
  fireEvent.change(await screen.findByLabelText('제목'), { target: { value: '돌아올 작성 글' } })
  await guideTo(user, GUIDE.review)
  await user.click(await screen.findByRole('button', { name: '지침 바꾸기' }))
  const sheet = await screen.findByRole('dialog', { name: '지침' })
  const savedPath = router.state.location.pathname
  expect(savedPath).toBe('/posts/20260828-돌아올-작성-글')
  await user.click(await within(sheet).findByRole('button', { name: '하나씩 보기' }))
  const review = await screen.findByRole('dialog', { name: '새 지침 후보' })
  await user.click(within(review).getByRole('link', { name: '요청한 글 보기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/source'))
  router.history.back()
  await waitFor(() => expect(router.state.location.pathname).toBe(savedPath))
  expect(await screen.findByRole('heading', { name: '이대로 써 볼까요?' })).toBeVisible()
  expect(screen.getByLabelText('제목')).toHaveValue('돌아올 작성 글')
})

it.each(['/posts/new', '/posts/saved'])(
  'does not open delayed guidelines after leaving review on %s',
  async (path) => {
    const user = userEvent.setup()
    const calls: string[] = []
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const { router } = renderAppAt(path, {
      user: USER,
      calls,
      posts: { posts: [{ slug: 'saved' }], draftSaveGate: gate },
    })
    fireEvent.change(await screen.findByLabelText('제목'), { target: { value: '느린 저장' } })
    await guideTo(user, GUIDE.review)
    await user.click(await screen.findByRole('button', { name: '지침 바꾸기' }))
    await waitFor(() => expect(calls).toContain('SavePostDraft'))
    await guideTo(user, GUIDE.photos)
    await act(async () => {
      release()
      await gate
    })
    const savedPath = path === '/posts/new' ? '/posts/20260828-느린-저장' : path
    await waitFor(() => {
      expect(router.state.status).toBe('idle')
      expect(router.state.location.pathname).toBe(savedPath)
    })
    expect(screen.queryByRole('dialog', { name: '지침' })).toBeNull()
    expect(await screen.findByRole('heading', { name: '사진을 골라 주세요' })).toBeVisible()
  },
)

it('preserves the post return of a standalone guidelines URL', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/guidelines?post=saved', {
    user: USER,
    posts: { posts: [{ slug: 'saved' }] },
  })
  const back = await screen.findByRole('link', { name: '글 작성' })
  expect(back).toHaveAttribute('href', '/posts/saved')
  expect(screen.getAllByRole('navigation', { name: '돌아가기' })).toHaveLength(1)
  await user.click(back)
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/saved'))
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

// THEME-39: the five stations are panels of the one mounted page — selecting one changes neither
// the address nor the post, and starts nothing.
it('moves between the five stations without changing the address or saving', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  const { router } = renderAppAt('/posts/saved', {
    user: USER,
    calls,
    posts: { posts: [{ slug: 'saved', status: 'review', content: POST_CONTENT_FIXTURE }] },
  })
  await screen.findByRole('tab', { name: '다듬기' })
  calls.length = 0
  for (const station of ['생성', '관찰', '스토리라인', '다듬기', '완성']) {
    await user.click(screen.getByRole('tab', { name: station }))
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: station })).toHaveAttribute('aria-selected', 'true'),
    )
    expect(router.state.location.pathname).toBe('/posts/saved')
  }
  expect(calls.filter((call) => /^(Save|Start|Finalize)/.test(call))).toEqual([])
})
