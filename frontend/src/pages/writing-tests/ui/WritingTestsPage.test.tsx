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
import { contentLanguageToProto, Stage } from '@/shared/api'
import { writingTestI18n } from '@/features/writing-test'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { createTestQueryClient, withProviders } from '@/test/session'
import { chooseOption } from '@/test/listbox'
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
function savedPair(stage: Stage, a = 'model-0', b = 'model-1') {
  return {
    stage,
    candidateA: { providerId: 'openrouter', modelId: a },
    candidateB: { providerId: 'openrouter', modelId: b },
    extraCandidates: [{ providerId: 'openrouter', modelId: 'model-15' }],
  }
}
it.each([
  ['write', Stage.WRITE],
  ['observe', Stage.OBSERVE],
] as const)(
  'prefills exactly the eligible saved A/B pair for a new two-model %s test without spending',
  async (stage, wireStage) => {
    const fixture = createWritingTestStudioFixture({ comparisonPairs: [savedPair(wireStage)] })
    mount(`/tests?stage=${stage}`, fixture)
    await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'AI 모델' }))
    expect(screen.getByRole('combobox', { name: /^후보 1/ })).toHaveTextContent('Model 1')
    expect(screen.getByRole('combobox', { name: /^후보 2/ })).toHaveTextContent('Model 2')
    expect(screen.queryByRole('combobox', { name: /^후보 3/ })).toBeNull()
    expect(fixture.calls).toContain('GetComparisonPairs')
    expect(fixture.estimates).toHaveLength(0)
    expect(fixture.admissions).toHaveLength(0)
    expect(fixture.publications).toHaveLength(0)
  },
)
it('keeps an owner/entry recovered test draft ahead of a newly changed saved pair', async () => {
  const options = { comparisonPairs: [savedPair(Stage.WRITE)] }
  const fixture = createWritingTestStudioFixture(options)
  const user = userEvent.setup()
  mount('/tests?draft=retained-work', fixture)
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  await user.click(screen.getByRole('button', { name: 'AI 모델' }))
  await chooseOption(user, screen.getByRole('combobox', { name: /^후보 1/ }), 'Model 3')
  await chooseOption(user, screen.getByRole('combobox', { name: /^후보 2/ }), 'Model 4')
  cleanup()
  options.comparisonPairs = [savedPair(Stage.WRITE, 'model-4', 'model-5')]
  mount('/tests?draft=retained-work', fixture)
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expect(await screen.findByRole('combobox', { name: /^후보 1/ })).toHaveTextContent('Model 3')
  expect(screen.getByRole('combobox', { name: /^후보 2/ })).toHaveTextContent('Model 4')
  expect(fixture.estimates).toHaveLength(0)
  expect(fixture.admissions).toHaveLength(0)
})
it.each([4, 8, 16] as const)(
  'does not load or prefill stored pair/extras in a new %i-entry test',
  async (count) => {
    const fixture = createWritingTestStudioFixture({
      count,
      comparisonPairs: [savedPair(Stage.WRITE)],
    })
    mount(`/tests?count=${count}`, fixture)
    await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'AI 모델' }))
    for (let index = 1; index <= count; index++)
      expect(
        screen.getByRole('combobox', { name: new RegExp(`^후보 ${index}(?:\\s|$)`) }),
      ).toHaveTextContent('선택해 주세요')
    expect(fixture.calls).not.toContain('GetComparisonPairs')
    expect(fixture.estimates).toHaveLength(0)
    expect(fixture.admissions).toHaveLength(0)
  },
)
it.each([
  ['voice', '말투'],
  ['template', '글 템플릿'],
  ['guideline', '글 지침'],
] as const)(
  'keeps saved model pairs outside a two-entry %s setting test',
  async (factor, label) => {
    const fixture = createWritingTestStudioFixture({
      factor,
      comparisonPairs: [savedPair(Stage.WRITE)],
    })
    mount(`/tests?factor=${factor}`, fixture)
    await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
    await userEvent.click(screen.getByRole('button', { name: label }))
    expect(await screen.findByRole('combobox', { name: /^후보 1/ })).toHaveTextContent(
      '선택해 주세요',
    )
    expect(screen.getByRole('combobox', { name: /^후보 2/ })).toHaveTextContent('선택해 주세요')
    expect(fixture.calls).not.toContain('GetComparisonPairs')
    expect(fixture.estimates).toHaveLength(0)
    expect(fixture.admissions).toHaveLength(0)
    expect(fixture.preparations).toHaveLength(0)
  },
)
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
