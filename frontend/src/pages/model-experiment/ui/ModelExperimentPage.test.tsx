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
    back: '/ai-models/compare?stage=write',
    ko: '← 모델 비교로 돌아가기',
    en: '← Back to model comparison',
    primary: '/ai-models',
    group: '/ai-models/compare',
  },
  {
    path: '/ai-models/experiments/review-1?stage=write',
    back: '/ai-models/experiments?stage=write',
    ko: '← 비교 기록',
    en: '← Comparison history',
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
)(
  'returns to the actual entry for $path in $locale',
  async ({ path, back, ko, en, locale, primary, group }) => {
    initializeI18n(locale)
    const user = userEvent.setup()
    const { router, container } = renderAppAt(path, {
      user: { id: 'alice' },
      posts: { posts },
      experiments: { history },
    })
    await screen.findByRole('heading', {
      name: locale === 'ko' ? '블라인드 비교' : 'Blind comparison',
    })
    const link = screen.getByRole('link', { name: locale === 'ko' ? ko : en })
    expect(link).toHaveAttribute('href', back)
    expect(
      [...container.querySelectorAll('aside a[aria-current="page"]')].map(
        (a) => a.getAttribute('href')?.split('?')[0],
      ),
    ).toEqual([primary, group])
    await user.click(link)
    await waitFor(() => expect(router.state.location.href).toBe(back))
  },
)

it.each(['', '?from=https://example.com', '?from=posts&stage=invalid'])(
  'keeps legacy and invalid model origins in model history: %s',
  async (search) => {
    renderAppAt(`/ai-models/experiments/review-1${search}`, {
      user: { id: 'alice' },
      experiments: { history },
    })
    await screen.findByRole('heading', { name: '블라인드 비교' })
    expect(screen.getByRole('link', { name: '← 비교 기록' })).toHaveAttribute(
      'href',
      '/ai-models/experiments?stage=write',
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
  await user.click(await screen.findByRole('link', { name: '← 비교 기록' }))
  await waitFor(() => expect(router.state.location.href).toBe('/ai-models/experiments?stage=write'))
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
    await screen.findByRole('heading', { name: '블라인드 비교' })
    expect(screen.getByRole('link', { name: '← 내 글 목록으로 돌아가기' })).toHaveAttribute(
      'href',
      '/posts',
    )
  },
)

it.each([
  [
    '/ai-models/experiments/review-1?from=compare&stage=analyze',
    '← 모델 비교로 돌아가기',
    '/ai-models/compare?stage=analyze',
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
