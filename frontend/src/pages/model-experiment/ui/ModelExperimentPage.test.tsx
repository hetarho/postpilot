import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { paidActions, renderLegacyModelRoute } from '@/test/legacy-model-routes'

const history = [{ id: 'review-1', stage: Stage.WRITE, postSlug: 'draft-1' }]
const posts = [
  { slug: 'draft-1', title: 'Draft', status: 'draft', pendingExperimentId: 'review-1' },
]

afterEach(() => initializeI18n('ko'))

const entries = [
  {
    path: '/tests/records/review-1',
    back: '/tests/history',
    ko: '테스트 기록',
    en: 'Test history',
  },
  {
    path: '/ai-models/experiments/review-1?stage=write&from=compare',
    back: '/tests/history',
    ko: '테스트 기록',
    en: 'Test history',
  },
  {
    path: '/ai-models/experiments/review-1?stage=write',
    back: '/tests/history',
    ko: '테스트 기록',
    en: 'Test history',
  },
  {
    path: '/posts/experiments/review-1',
    back: '/tests/history',
    ko: '테스트 기록',
    en: 'Test history',
  },
  {
    path: '/posts/experiments/review-1?from=posts&q=Draft&status=draft',
    back: '/posts?q=Draft&status=draft',
    ko: '글 작업 내역',
    en: 'Post work history',
  },
] as const

it.each(
  entries.flatMap((entry) => (['ko', 'en'] as const).map((locale) => ({ ...entry, locale }))),
)('returns to the actual entry for $path in $locale', async ({ path, back, ko, en, locale }) => {
  initializeI18n(locale)
  const user = userEvent.setup()
  const { router, procedures } = renderLegacyModelRoute(path, {
    user: { id: 'alice' },
    posts: { posts },
    experiments: { history },
  })
  await screen.findByRole('heading', {
    name: locale === 'ko' ? '이전 유료 비교 기록' : 'Earlier paid comparison',
  })
  const link = within(screen.getByRole('main')).getByRole('link', {
    name: locale === 'ko' ? ko : en,
  })
  expect(router.state.location.pathname).toBe('/tests/records/review-1')
  expect(link).toHaveAttribute('href', back)
  expect(document.querySelector('aside')).toBeNull()
  const breadcrumb = screen.getByRole('navigation', {
    name: locale === 'ko' ? '현재 위치' : 'Current location',
  })
  expect(within(breadcrumb).queryByRole('link')).not.toBeInTheDocument()
  await user.click(within(breadcrumb).getByRole('button'))
  const location = await screen.findByRole('dialog', {
    name: locale === 'ko' ? /^현재 위치:/ : /^Current location:/,
  })
  expect(
    within(within(location).getByRole('list'))
      .getAllByRole('link')
      .map((item) => item.getAttribute('href')),
  ).toEqual(['/tests', '/tests/history'])
  await user.keyboard('{Escape}')
  expect(location).not.toBeInTheDocument()
  if (back.startsWith('/posts')) {
    await user.click(link)
    await waitFor(() => expect(router.state.location.href).toBe(back))
  }
  expect(paidActions(procedures)).toEqual([])
})

it.each(['', '?from=https://example.com', '?from=posts&stage=invalid'])(
  'keeps legacy and invalid model origins in model history: %s',
  async (search) => {
    renderAppAt(`/ai-models/experiments/review-1${search}`, {
      user: { id: 'alice' },
      experiments: { history },
    })
    await screen.findByRole('heading', { name: '이전 유료 비교 기록' })
    expect(
      within(screen.getByRole('main')).getByRole('link', { name: '테스트 기록' }),
    ).toHaveAttribute('href', '/tests/history')
    expect(screen.queryByRole('link', { name: '글 작성' })).not.toBeInTheDocument()
  },
)

it('opens a retained paid record from canonical history and returns to the same owner stage/source', async () => {
  const user = userEvent.setup()
  const { router, procedures } = renderLegacyModelRoute(
    '/ai-models/experiments?stage=write&source=draft-1',
    { user: { id: 'alice' }, experiments: { history } },
  )
  await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
  const record = await within(
    screen.getByRole('region', { name: '이전 유료 비교 기록' }),
  ).findByRole('link', { name: '계속 보기' })
  expect(new URL(record.getAttribute('href')!, 'https://fixture').pathname).toBe(
    '/tests/records/review-1',
  )
  await act(() => router.navigate({ href: record.getAttribute('href')! }))
  await screen.findByRole('heading', { name: '이전 유료 비교 기록' })
  const back = within(screen.getByRole('main')).getByRole('link', { name: '테스트 기록' })
  expect(back).toHaveAttribute('href', '/tests/history?stage=write&source=draft-1')
  await user.click(back)
  await waitFor(() => expect(router.state.location.pathname).toBe('/tests/history'))
  expect(router.state.location.search).toMatchObject({ stage: 'write', source: 'draft-1' })
  expect(paidActions(procedures)).toEqual([])
})

it('carries the narrowed post list through a pending result and back', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts?q=Draft&status=draft', {
    user: { id: 'alice' },
    posts: { posts },
    experiments: { history },
  })
  const record = await screen.findByRole('link', { name: /Draft/ })
  const recordUrl = new URL(record.getAttribute('href')!, 'https://fixture')
  expect(recordUrl.pathname).toBe('/tests/records/review-1')
  expect(recordUrl.searchParams.get('entry')).toBe('/posts?q=Draft&status=draft')
  await user.click(record)
  await waitFor(() => expect(router.state.location.pathname).toBe('/tests/records/review-1'))
  await user.click(await screen.findByRole('link', { name: '글 작업 내역' }))
  await waitFor(() => expect(router.state.location.href).toBe('/posts?q=Draft&status=draft'))
  expect(await screen.findByRole('link', { name: /Draft/ })).toBeInTheDocument()
})

it('opens the editor result in canonical test navigation and returns to its owned post', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts/draft-1', {
    user: { id: 'alice' },
    posts: { posts },
    experiments: { history },
  })
  const record = await screen.findByRole('link', { name: 'A/B 결과 확인' })
  expect(record).toHaveAttribute('href', '/tests/records/review-1?source=draft-1')
  await user.click(record)
  await waitFor(() => expect(router.state.location.pathname).toBe('/tests/records/review-1'))
  await user.click(await screen.findByRole('link', { name: '글 작성' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/posts/draft-1'))
})

it.each(['', '?from=compare', '?from=https://example.com'])(
  'falls back to named test history when a retained record has no explicit source: %s',
  async (search) => {
    renderAppAt(`/posts/experiments/review-1${search}`, {
      user: { id: 'alice' },
      experiments: { history: [{ id: 'review-1', stage: Stage.WRITE }] },
    })
    await screen.findByRole('heading', { name: '이전 유료 비교 기록' })
    expect(
      within(screen.getByRole('main')).getByRole('link', { name: '테스트 기록' }),
    ).toHaveAttribute('href', '/tests/history')
  },
)

it.each([
  ['/ai-models/experiments/review-1?from=compare&stage=write', '테스트 기록', '/tests/history'],
  ['/posts/experiments/review-1?from=posts&q=Draft', '글 작업 내역', '/posts?q=Draft'],
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
