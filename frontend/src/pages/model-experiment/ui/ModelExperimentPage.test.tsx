import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const history = [{ id: 'review-1', stage: Stage.WRITE, postSlug: 'draft-1' }]
const posts = [
  { slug: 'draft-1', title: 'Draft', status: 'draft', pendingExperimentId: 'review-1' },
]

afterEach(() => initializeI18n('ko'))

const entries = [
  {
    path: '/ai-models/experiments/review-1?stage=write&from=compare',
    back: '/tests',
    ko: '← 글쓰기 테스트로 돌아가기',
    en: '← Back to writing tests',
    primary: '/ai-models',
    group: '/ai-models/compare',
  },
  {
    path: '/ai-models/experiments/review-1?stage=write',
    back: '/tests/history',
    ko: '← 글쓰기 테스트 기록',
    en: '← Writing test history',
    primary: '/ai-models',
    group: '/ai-models/experiments',
  },
  {
    path: '/posts/experiments/review-1',
    back: '/posts/draft-1',
    ko: '← 글로 돌아가기',
    en: '← Back to post',
    primary: '/posts',
    group: '/posts',
  },
  {
    path: '/posts/experiments/review-1?from=posts&q=Draft&status=draft',
    back: '/posts?q=Draft&status=draft',
    ko: '← 내 글 목록으로 돌아가기',
    en: '← Back to my posts',
    primary: '/posts',
    group: '/posts',
  },
] as const

it.each(
  entries.flatMap((entry) => (['ko', 'en'] as const).map((locale) => ({ ...entry, locale }))),
)('returns to the actual entry for $path in $locale', async ({ path, back, ko, en, locale }) => {
  initializeI18n(locale)
  const user = userEvent.setup()
  const { router } = renderAppAt(path, {
    user: { id: 'alice' },
    posts: { posts },
    experiments: { history },
  })
  await screen.findByRole('heading', {
    name: locale === 'ko' ? '이전 유료 비교 기록' : 'Earlier paid comparison',
  })
  const link = screen.getByRole('link', { name: locale === 'ko' ? ko : en })
  expect(link).toHaveAttribute('href', back)
  expect(document.querySelector('aside')).toBeNull()
  if (back.startsWith('/posts')) {
    await user.click(link)
    await waitFor(() => expect(router.state.location.href).toBe(back))
  }
})

it.each(['', '?from=https://example.com', '?from=posts&stage=invalid'])(
  'keeps legacy and invalid model origins in model history: %s',
  async (search) => {
    renderAppAt(`/ai-models/experiments/review-1${search}`, {
      user: { id: 'alice' },
      experiments: { history },
    })
    await screen.findByRole('heading', { name: '이전 유료 비교 기록' })
    expect(screen.getByRole('link', { name: '← 글쓰기 테스트 기록' })).toHaveAttribute(
      'href',
      '/tests/history',
    )
    expect(screen.queryByRole('link', { name: '← 글로 돌아가기' })).not.toBeInTheDocument()
  },
)

it('opens a post-backed history record and returns to the same history stage', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/ai-models/experiments?stage=write', {
    user: { id: 'alice' },
    experiments: { history },
  })
  await user.click(await screen.findByRole('link', { name: /draft-1/ }))
  expect(await screen.findByRole('link', { name: '← 글쓰기 테스트 기록' })).toHaveAttribute(
    'href',
    '/tests/history',
  )
  expect(router.state.location.pathname).toBe('/ai-models/experiments/review-1')
})

it('carries the narrowed post list through a pending result and back', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts?q=Draft&status=draft', {
    user: { id: 'alice' },
    posts: { posts },
    experiments: { history },
  })
  await user.click(await screen.findByRole('link', { name: /Draft/ }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/experiments/review-1'))
  await user.click(await screen.findByRole('link', { name: '← 내 글 목록으로 돌아가기' }))
  await waitFor(() => expect(router.state.location.href).toBe('/posts?q=Draft&status=draft'))
  expect(await screen.findByRole('link', { name: /Draft/ })).toBeInTheDocument()
})

it('opens the editor result in writing navigation and returns to its post', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts/draft-1', {
    user: { id: 'alice' },
    posts: { posts },
    experiments: { history },
  })
  await user.click(await screen.findByRole('link', { name: 'AI 결과 확인 →' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/experiments/review-1'))
  await user.click(await screen.findByRole('link', { name: '← 글로 돌아가기' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/draft-1'))
})

it.each(['', '?from=compare', '?from=https://example.com'])(
  'falls back to the post list when the writing result has no source: %s',
  async (search) => {
    renderAppAt(`/posts/experiments/review-1${search}`, {
      user: { id: 'alice' },
      experiments: { history: [{ id: 'review-1', stage: Stage.WRITE }] },
    })
    await screen.findByRole('heading', { name: '이전 유료 비교 기록' })
    expect(screen.getByRole('link', { name: '← 내 글 목록으로 돌아가기' })).toHaveAttribute(
      'href',
      '/posts',
    )
  },
)

it.each([
  [
    '/ai-models/experiments/review-1?from=compare&stage=write',
    '← 글쓰기 테스트로 돌아가기',
    '/tests',
  ],
  ['/posts/experiments/review-1?from=posts&q=Draft', '← 내 글 목록으로 돌아가기', '/posts?q=Draft'],
])('retains an exit through loading and failed reads at %s', async (path, name, href) => {
  let release!: () => void
  const readGate = new Promise<void>((resolve) => {
    release = resolve
  })
  renderAppAt(path, { user: { id: 'alice' }, experiments: { readGate, detailFails: true } })
  const main = within(await screen.findByRole('main'))
  expect(main.getByRole('status')).toHaveTextContent('비교 결과를 불러오는 중')
  expect(main.getByRole('link', { name })).toHaveAttribute('href', href)
  await act(async () => release())
  expect(await screen.findByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  expect(main.getByRole('link', { name })).toHaveAttribute('href', href)
})

it('keeps the writing entry and list context through the sign-in guard', async () => {
  const path = '/posts/experiments/review-1?from=posts&q=Draft&status=draft'
  const { router } = renderAppAt(path)
  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(router.state.location.search.redirect).toBe(path)
})
