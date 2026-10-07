import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { contentLanguageToProto } from '@/shared/api'
import { writingTestI18n } from '@/features/writing-test'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { createTestQueryClient, withProviders } from '@/test/session'
import { WritingTestHistoryPage, WritingTestPage, WritingTestsPage } from './WritingTestsPage'

beforeEach(() => {
  sessionStorage.clear()
  initializeI18n('ko')
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
})
afterEach(cleanup)
function mount(path: string, fixture: ReturnType<typeof createWritingTestStudioFixture>) {
  const root = createRootRoute({ component: Outlet })
  const newTest = createRoute({
    getParentRoute: () => root,
    path: '/tests',
    component: WritingTestsPage,
  })
  const detail = createRoute({
    getParentRoute: () => root,
    path: '/tests/$testId',
    component: WritingTestPage,
  })
  const history = createRoute({
    getParentRoute: () => root,
    path: '/tests/history',
    component: WritingTestHistoryPage,
  })
  const router = createRouter({
    routeTree: root.addChildren([newTest, detail, history]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  render(<RouterProvider router={router} />, {
    wrapper: withProviders(fixture.transport, createTestQueryClient()),
  })
  return router
}
it('preloads the actual source context before authoring and quotes its real revisions and language without auto generation', async () => {
  const fixture = createWritingTestStudioFixture({ count: 4, source: true })
  mount('/tests?count=4&source=owned-source', fixture)
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  await userEvent.click(screen.getByRole('button', { name: 'AI 모델' }))
  for (let index = 0; index < 4; index++) {
    await userEvent.click(
      screen.getByRole('combobox', { name: new RegExp(`^후보 ${index + 1}(?:\\s|$)`) }),
    )
    await userEvent.click(screen.getByRole('option', { name: `Model ${index + 1}` }))
  }
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(await screen.findByRole('textbox', { name: '공통 글감' })).toHaveValue(
    'Real recorded material',
  )
  expect(screen.getByRole('combobox', { name: /^글 언어 영어$/ })).toBeVisible()
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.estimates).toHaveLength(0)
  await userEvent.click(screen.getByRole('button', { name: '글 4편 생성 비용 확인' }))
  await screen.findByRole('button', { name: '글 4편 만들기' })
  expect(fixture.estimates[0]).toMatchObject({
    count: 4,
    context: {
      sourcePostSlug: 'owned-source',
      expectedInputRevision: 9007199254740993n,
      expectedContentRevision: 7n,
      targetLanguage: contentLanguageToProto('en'),
      material: {
        text: 'Real recorded material',
        fictional: false,
        attachmentIds: ['owned-photo'],
      },
    },
  })
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})
it('opens a deep test link with its real actor and private server match without any new paid action', async () => {
  const fixture = createWritingTestStudioFixture({ readableTest: true })
  mount('/tests/test', fixture)
  await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
  expect(fixture.calls).toContain('GetWritingTest')
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.votes).toHaveLength(0)
  expect(fixture.preparations).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})
it('mounts common and one selected voice history as read-only and offers an explicit fresh draft action', async () => {
  const fixture = createWritingTestStudioFixture({ readableTest: true })
  mount('/tests/history?voiceId=voice-history', fixture)
  await waitFor(() => expect(fixture.calls).toContain('ListWritingTests'))
  expect(screen.getByRole('button', { name: '새 테스트' })).toBeEnabled()
  await waitFor(() =>
    expect(fixture.calls.filter((name) => name === 'ListVoiceChecks')).toHaveLength(1),
  )
  // Blind model records carry no proof that they used the requested voice.
  expect(screen.queryByRole('link', { name: '테스트 이어보기' })).not.toBeInTheDocument()
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.votes).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})

it('applies stage and source history URL filters to the actual displayed records', async () => {
  const fixture = createWritingTestStudioFixture({ readableTest: true })
  const record = fixture.getTest()!
  record.sourcePostSlug = 'recorded-source'
  mount('/tests/history?stage=observe&source=another-source', fixture)
  await waitFor(() => expect(fixture.calls).toContain('ListWritingTests'))
  expect(screen.queryByRole('link', { name: '테스트 이어보기' })).not.toBeInTheDocument()
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})
