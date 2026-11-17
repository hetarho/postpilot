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
import { wireObservationTest } from '@/entities/writing-test/api/fixtures.test-support'
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
/** A tile only chooses the factor; the step bar's 다음 moves on to the candidates. */
async function chooseFactor(label: string, user: Pick<typeof userEvent, 'click'> = userEvent) {
  await user.click(screen.getByRole('button', { name: label }))
  expect(screen.queryByRole('checkbox', { name: /^Model/ })).toBeNull()
  await user.click(screen.getByRole('button', { name: '다음' }))
}
/** A reload never runs React's cleanup, while leaving the setup does and forgets it: keep the
 *  session's storage exactly as the page left it across `unload`. */
function reloadAfter(unload: () => void) {
  const kept = Array.from({ length: sessionStorage.length }, (_, index) => {
    const key = sessionStorage.key(index)!
    return [key, sessionStorage.getItem(key)!] as const
  })
  const recovery = kept.filter(([key]) => key.startsWith('postpilot:writing-test:'))
  expect(recovery).toHaveLength(1)
  unload()
  expect(sessionStorage.getItem(recovery[0][0])).toBeNull()
  for (const [key, value] of kept) sessionStorage.setItem(key, value)
}
function savedPair(stage: Stage, a = 'model-0', b = 'model-1') {
  return {
    stage,
    candidateA: { providerId: 'openrouter', modelId: a },
    candidateB: { providerId: 'openrouter', modelId: b },
    extraCandidates: [{ providerId: 'openrouter', modelId: 'model-15' }],
  }
}
it('prefills exactly the eligible saved A/B pair for a new two-model test without spending', async () => {
  const fixture = createWritingTestStudioFixture({ comparisonPairs: [savedPair(Stage.WRITE)] })
  mount('/tests?stage=write', fixture)
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  await chooseFactor('AI 모델')
  expect(await screen.findByRole('checkbox', { name: 'Model 1' })).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'Model 2' })).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'Model 3' })).not.toBeChecked()
  expect(screen.getByText('2 / 2개 선택')).toBeVisible()
  expect(fixture.calls).toContain('GetComparisonPairs')
  expect(fixture.estimates).toHaveLength(0)
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})
it('keeps an owner/entry recovered test draft ahead of a newly changed saved pair', async () => {
  const options = { comparisonPairs: [savedPair(Stage.WRITE)] }
  const fixture = createWritingTestStudioFixture(options)
  const user = userEvent.setup()
  mount('/tests?draft=retained-work', fixture)
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  await chooseFactor('AI 모델', user)
  await user.click(await screen.findByRole('checkbox', { name: 'Model 1' }))
  await user.click(screen.getByRole('checkbox', { name: 'Model 2' }))
  await user.click(screen.getByRole('checkbox', { name: 'Model 3' }))
  await user.click(screen.getByRole('checkbox', { name: 'Model 4' }))
  reloadAfter(cleanup)
  options.comparisonPairs = [savedPair(Stage.WRITE, 'model-4', 'model-5')]
  mount('/tests?draft=retained-work', fixture)
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expect(await screen.findByRole('checkbox', { name: 'Model 3' })).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'Model 4' })).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'Model 1' })).not.toBeChecked()
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
    await chooseFactor('AI 모델')
    expect(await screen.findByText(`0 / ${count}개 선택`)).toBeVisible()
    expect(screen.getAllByRole('checkbox').some((box) => (box as HTMLInputElement).checked)).toBe(
      false,
    )
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
    await chooseFactor(label)
    expect(await screen.findByRole('combobox', { name: /^후보 1/ })).toHaveTextContent(
      'AI가 준비해요',
    )
    expect(screen.getByRole('combobox', { name: /^후보 2/ })).toHaveTextContent('AI가 준비해요')
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
  await chooseFactor('AI 모델')
  for (let index = 0; index < 4; index++)
    await userEvent.click(await screen.findByRole('checkbox', { name: `Model ${index + 1}` }))
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(await screen.findByRole('textbox', { name: /^공통 글감/ })).toHaveValue(
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
it('names a photo observation test as such on its deep link (MODEL-97)', async () => {
  const fixture = createWritingTestStudioFixture()
  fixture.restoreTest(wireObservationTest(['photo-1'], 2))
  mount('/tests/test', fixture)
  await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
  expect(screen.getByRole('heading', { level: 1, name: '사진 관찰 테스트' })).toBeInTheDocument()
  expect(screen.queryByRole('heading', { level: 1, name: '글쓰기 테스트' })).toBeNull()
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
