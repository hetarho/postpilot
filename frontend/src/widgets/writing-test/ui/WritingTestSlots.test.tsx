import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoGuidelineScope } from '@/shared/api'
import { SM_MEDIA_QUERY } from '@/shared/ui'
import { writingTestI18n } from '@/features/writing-test'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { createTestQueryClient, withProviders } from '@/test/session'
import { WritingTestStudio } from './WritingTestStudio'

beforeEach(() => {
  sessionStorage.clear()
  initializeI18n('ko')
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
  i18next.addResourceBundle('en', 'writingTests', writingTestI18n.en, true, true)
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})
type Fixture = ReturnType<typeof createWritingTestStudioFixture>
const revision = '2026-10-08T00:00:00Z'
const owned = {
  ownedTemplates: [1, 2].map((index) => ({
    id: `saved-${index}`,
    name: `Saved ${index}`,
    body: '<write>Saved structure</write>',
    updatedAt: revision,
  })),
  ownedVoices: [1, 2].map((index) => ({
    id: `saved-${index}`,
    name: `Saved ${index}`,
    made: true,
    updatedAt: revision,
  })),
  ownedGuidelines: [1, 2].map((index) => ({
    id: `saved-${index}`,
    title: `Saved ${index}`,
    text: `Saved rule ${index}`,
    scope: ProtoGuidelineScope.GLOBAL,
    updatedAt: revision,
  })),
}
function mount(fixture: Fixture, ownerId = 'alice') {
  return render(
    <WritingTestStudio ownerId={ownerId} seedKey="mixed-slots" initialPlan={fixture.plan} />,
    {
      wrapper: withProviders(fixture.transport, createTestQueryClient()),
    },
  )
}
/** A reload never runs React's cleanup, while leaving the setup does and forgets it: keep the
 *  session's storage exactly as the page left it, then open the page again. */
function reload(mounted: ReturnType<typeof mount>, fixture: Fixture) {
  const kept = Array.from({ length: sessionStorage.length }, (_, index) => {
    const key = sessionStorage.key(index)!
    return [key, sessionStorage.getItem(key)!] as const
  })
  const recovery = kept.filter(([key]) => key.startsWith('postpilot:writing-test:'))
  expect(recovery).toHaveLength(1)
  mounted.unmount()
  expect(sessionStorage.getItem(recovery[0][0])).toBeNull()
  for (const [key, value] of kept) sessionStorage.setItem(key, value)
  return mount(fixture)
}
async function select(label: string, option: string) {
  await userEvent.click(screen.getByRole('combobox', { name: new RegExp(`^${label}(?:\\s|$)`) }))
  await userEvent.click(await screen.findByRole('option', { name: option }))
}
async function enter(fixture: Fixture, label: string) {
  // A tile only chooses the factor; the step bar's 다음 moves on to the candidates.
  await userEvent.click(await screen.findByRole('button', { name: label }))
  expect(screen.queryByRole('textbox', { name: '후보의 방향 (선택)' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await waitFor(() => expect(fixture.calls).toContain('GetSelections'))
  await waitFor(() => expect(screen.getByRole('button', { name: '다음' })).toBeEnabled())
}
function expectNoPreparation(fixture: Fixture) {
  expect(fixture.preparationEstimates).toEqual([])
  expect(fixture.preparationCreates).toEqual([])
  expect(fixture.preparations).toEqual([])
}
async function prepare(fixture: Fixture, missing: number) {
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  const confirmation = await screen.findByRole('button', {
    name: `AI로 ${missing}칸 채우고 계속`,
  })
  expect(fixture.preparationEstimates.at(-1)?.count).toBe(missing)
  expect(fixture.preparations).toHaveLength(fixture.preparationCreates.length)
  await userEvent.click(confirmation)
  await screen.findByRole('textbox', { name: /^공통 글감/ })
}
async function postQuote(fixture: Fixture, count = fixture.plan.count) {
  await userEvent.click(screen.getByRole('button', { name: `글 ${count}편 생성 비용 확인` }))
  await screen.findByRole('button', { name: `글 ${count}편 만들기` })
  expect(fixture.admissions).toEqual([])
  return fixture.estimates.at(-1)!
}

it.each([2, 4, 16] as const)(
  'prepares exactly %i minus one empty slots and preserves the saved slot',
  async (count) => {
    const fixture = createWritingTestStudioFixture({ ...owned, factor: 'template', count })
    mount(fixture)
    await enter(fixture, '글 템플릿')
    expect(screen.queryByRole('tab', { name: 'AI와 새 후보 만들기' })).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /^후보 1 AI가 준비해요$/ })).toBeVisible()
    expect(screen.getByRole('textbox', { name: '후보의 방향 (선택)' })).toHaveValue('')
    await select(`후보 ${count}`, 'Saved 1')
    expectNoPreparation(fixture)
    await prepare(fixture, count - 1)
    expect(fixture.preparationCreates).toHaveLength(1)
    expect(fixture.preparations).toHaveLength(1)
    expect(fixture.preparations[0].count).toBe(count - 1)
    expect(fixture.preparations[0].prompt).toBe(writingTestI18n.ko.defaultDirection.template)
    const quote = await postQuote(fixture)
    expect(quote.count).toBe(count)
    expect(quote.entrants).toHaveLength(count)
    expect(quote.entrants[count - 1].source).toMatchObject({
      case: 'setting',
      value: { id: 'saved-1', revision },
    })
    expect(
      quote.entrants.slice(0, -1).every((entrant) => entrant.source.case === 'authoringCandidate'),
    ).toBe(true)
    expect(fixture.preparationEstimates.map((estimate) => estimate.count)).toEqual([count - 1])
  },
)

it.each([
  ['voice', '말투'],
  ['template', '글 템플릿'],
  ['guideline', '글 지침'],
] as const)(
  'skips all preparation when both %s slots select saved settings',
  async (factor, label) => {
    const fixture = createWritingTestStudioFixture({ ...owned, factor })
    mount(fixture)
    await enter(fixture, label)
    await select('후보 2', 'Saved 2')
    await select('후보 1', 'Saved 1')
    expectNoPreparation(fixture)
    await userEvent.click(screen.getByRole('button', { name: '다음' }))
    await screen.findByRole('textbox', { name: /^공통 글감/ })
    expectNoPreparation(fixture)
    const quote = await postQuote(fixture)
    expect(quote.entrants.map((entrant) => entrant.source.case)).toEqual(['setting', 'setting'])
    expectNoPreparation(fixture)
  },
)

it.each(['voice', 'guideline'] as const)(
  'uses the kind-specific default for a blank %s direction after explicit approval',
  async (factor) => {
    const fixture = createWritingTestStudioFixture({ ...owned, factor })
    mount(fixture)
    await enter(fixture, writingTestI18n.ko.factor[factor])
    await select('후보 1', 'Saved 1')
    expect(screen.getByRole('textbox', { name: '후보의 방향 (선택)' })).toHaveValue('')
    expectNoPreparation(fixture)
    await prepare(fixture, 1)
    expect(fixture.preparations[0].prompt).toBe(writingTestI18n.ko.defaultDirection[factor])
    expect(fixture.preparations).toHaveLength(1)
    expect(fixture.estimates).toEqual([])
  },
)

it('locks a quoted assignment and requotes changed missing slots only after Back', async () => {
  const fixture = createWritingTestStudioFixture({ ...owned, factor: 'template', count: 4 })
  mount(fixture)
  await enter(fixture, '글 템플릿')
  await select('후보 4', 'Saved 1')
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await screen.findByRole('button', { name: 'AI로 3칸 채우고 계속' })
  expect(screen.getByRole('combobox', { name: /^후보 1 AI가 준비해요$/ })).toBeDisabled()
  expect(screen.getByRole('combobox', { name: /^후보 4 Saved 1$/ })).toBeDisabled()
  expect(screen.getByRole('combobox', { name: '비교 형식 4강전' })).toBeDisabled()
  expect(screen.getByRole('textbox', { name: '후보의 방향 (선택)' })).toBeDisabled()
  expect(fixture.preparationCreates).toEqual([])
  expect(fixture.preparations).toEqual([])
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  expect(screen.queryByRole('button', { name: 'AI로 3칸 채우고 계속' })).not.toBeInTheDocument()
  expect(screen.getByRole('combobox', { name: /^후보 1 AI가 준비해요$/ })).toBeEnabled()
  await select('후보 1', 'Saved 2')
  expect(fixture.preparationCreates).toEqual([])
  expect(fixture.preparations).toEqual([])
  await prepare(fixture, 2)
  const quote = await postQuote(fixture)
  expect(fixture.preparationEstimates.map((estimate) => estimate.count)).toEqual([3, 2])
  expect(fixture.preparations.map((batch) => batch.count)).toEqual([2])
  expect(quote.entrants[0].source).toMatchObject({ case: 'setting', value: { id: 'saved-2' } })
  expect(quote.entrants[3].source).toMatchObject({ case: 'setting', value: { id: 'saved-1' } })
})

it('retains saved and prepared references through reload, format changes and another exact batch', async () => {
  const fixture = createWritingTestStudioFixture({ ...owned, factor: 'template' })
  const first = mount(fixture)
  await enter(fixture, '글 템플릿')
  await select('후보 1', 'Saved 1')
  await prepare(fixture, 1)
  const firstQuote = await postQuote(fixture)
  const references = firstQuote.entrants.slice()
  reload(first, fixture)
  // Cost quotes do not survive reload as approval; the frozen contestants and material do.
  await screen.findByRole('textbox', { name: /^공통 글감/ })
  expect(fixture.preparations).toHaveLength(1)
  expect(fixture.estimates).toHaveLength(1)
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  expect(screen.getByRole('combobox', { name: /^후보 1 Saved 1$/ })).toBeVisible()
  expect(screen.getByRole('combobox', { name: /^후보 2 AI가 만든 후보/ })).toBeVisible()
  await select('비교 형식', '4강전')
  expect(screen.getByRole('combobox', { name: /^후보 1 Saved 1$/ })).toBeVisible()
  expect(screen.getByRole('combobox', { name: /^후보 2 AI가 만든 후보/ })).toBeVisible()
  expect(fixture.preparations).toHaveLength(1)
  await prepare(fixture, 2)
  const second = await postQuote(fixture, 4)
  expect(second.entrants.slice(0, 2)).toEqual(references)
  expect(fixture.preparations.map((batch) => batch.count)).toEqual([1, 2])
  expect(fixture.preparationCreates).toEqual(['prepared-alice', 'prepared-alice-2'])
  expect(second.entrants[2].source).toMatchObject({
    case: 'authoringCandidate',
    value: { sessionId: 'prepared-alice-2' },
  })
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  await select('비교 형식', 'A/B 테스트')
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await screen.findByRole('textbox', { name: /^공통 글감/ })
  const shortened = await postQuote(fixture)
  expect(shortened.entrants).toEqual(references)
  expect(fixture.preparations.map((batch) => batch.count)).toEqual([1, 2])
})

it('retains required fields from a previous prepared template after a second batch and reload', async () => {
  // Both prepared templates ask, so they stand side by side only from `sm:` up.
  stubWidth(true)
  const fixture = createWritingTestStudioFixture({
    ...owned,
    factor: 'template',
    preparationBodies: [
      '<ask label="첫 장소" required="true"/><write>First structure</write>',
      '<ask label="두 번째 장소" required="true"/><write>Second structure</write>',
    ],
  })
  const first = mount(fixture)
  await enter(fixture, '글 템플릿')
  await select('후보 1', 'Saved 1')
  await prepare(fixture, 1)
  expect(screen.getByRole('textbox', { name: /^첫 장소/ })).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  await select('후보 1', 'AI가 준비해요')
  await prepare(fixture, 1)
  expect(screen.getByRole('textbox', { name: /^첫 장소/ })).toBeVisible()
  expect(screen.getByRole('textbox', { name: /^두 번째 장소/ })).toBeVisible()
  reload(first, fixture)
  const firstField = await screen.findByRole('textbox', { name: /^첫 장소/ })
  const secondField = await screen.findByRole('textbox', { name: /^두 번째 장소/ })
  expect(fixture.preparations.map((batch) => batch.count)).toEqual([1, 1])
  await userEvent.click(screen.getByRole('button', { name: '글 2편 생성 비용 확인' }))
  expect(fixture.estimates).toHaveLength(0)
  // The refused press lands on the first missing answer, which the slot order puts in the newer batch.
  await waitFor(() => expect(secondField).toHaveFocus())
  await userEvent.type(firstField, '공원')
  await userEvent.type(secondField, '동네 카페')
  const quote = await postQuote(fixture)
  expect(quote.context?.material?.templateAnswers).toEqual([
    expect.objectContaining({ label: '첫 장소', text: '공원', enabled: true }),
    expect.objectContaining({ label: '두 번째 장소', text: '동네 카페', enabled: true }),
  ])
  expect(
    new Set(
      quote.entrants.map(
        (entrant) => entrant.source.case === 'authoringCandidate' && entrant.source.value.sessionId,
      ),
    ).size,
  ).toBe(2)
})

/** jsdom has no layout: the `sm:` query decides whether the groups stand stacked or behind tabs. */
function stubWidth(wide: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: wide && query === SM_MEDIA_QUERY,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }))
}
const asking = {
  ...owned,
  ownedTemplates: [
    {
      id: 'saved-1',
      name: 'Template A',
      body: '<ask label="A 장소" required="true"/><ask label="한 줄 소개"/><write>A</write>',
      updatedAt: revision,
    },
    {
      id: 'saved-2',
      name: 'Template B',
      body: '<ask label="B 장소" required="true"/><ask label="한 줄 소개"/><write>B</write>',
      updatedAt: revision,
    },
  ],
}
async function materialOfTwoTemplates(fixture: Fixture) {
  mount(fixture)
  await enter(fixture, '글 템플릿')
  await select('후보 1', 'Template A')
  await select('후보 2', 'Template B')
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await screen.findByRole('textbox', { name: /^공통 글감/ })
}
const stepBar = () => document.querySelector<HTMLElement>('[data-action-bar="step"]')!

it('stacks each contestant template under its own heading, shares one answer and marks what is required', async () => {
  stubWidth(true)
  const fixture = createWritingTestStudioFixture({ ...asking, factor: 'template' })
  await materialOfTwoTemplates(fixture)
  expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
  ).toEqual(['Template A', 'Template B'])
  const first = within(screen.getByRole('region', { name: 'Template A' }))
  const second = within(screen.getByRole('region', { name: 'Template B' }))
  expect(first.getByRole('textbox', { name: 'A 장소 필수' })).toBeVisible()
  expect(second.getByRole('textbox', { name: 'B 장소 필수' })).toBeVisible()
  expect(first.queryByRole('textbox', { name: /^B 장소/ })).not.toBeInTheDocument()
  // The question both templates ask stands in each group, marked shared and not required.
  const sharedIn = (group: typeof first) => group.getByRole('textbox', { name: '한 줄 소개 공통' })
  await userEvent.type(sharedIn(first), '한 번에 읽히는 소개')
  expect(sharedIn(second)).toHaveValue('한 번에 읽히는 소개')
  expect(first.getAllByText('필수')).toHaveLength(1)
  expect(first.getAllByText('공통')).toHaveLength(1)
  expect(second.getAllByText('공통')).toHaveLength(1)
  // Every required field of the step wears the mark; the optional ones do not.
  expect(screen.getByRole('textbox', { name: '공통 글감 필수' })).toBeVisible()
  expect(screen.getByRole('spinbutton', { name: '목표 글자 수 필수' })).toBeVisible()
  expect(screen.getByRole('spinbutton', { name: '최소 태그 수 필수' })).toBeVisible()
  expect(screen.getByRole('combobox', { name: /^함께 사용할 글 작성 모델 필수/ })).toBeVisible()
  expect(screen.getByRole('combobox', { name: /^같게 유지할 말투/ })).not.toHaveAccessibleName(
    /필수/,
  )
  expect(screen.queryByRole('combobox', { name: /^함께 사용할 관찰 모델/ })).not.toBeInTheDocument()
  expect(fixture.estimates).toEqual([])
})

it('marks the observer and the guideline slot as required where they stand and drops the material mark under an attachment', async () => {
  stubWidth(true)
  const fixture = createWritingTestStudioFixture({ ...owned, factor: 'guideline', source: true })
  mount(fixture)
  await enter(fixture, '글 지침')
  await select('후보 1', 'Saved 1')
  await select('후보 2', 'Saved 2')
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await screen.findByRole('combobox', { name: /^바꿀 지침 자리 필수/ })
  expect(screen.getByRole('combobox', { name: /^함께 사용할 관찰 모델 필수/ })).toBeVisible()
  await waitFor(() => expect(screen.getByRole('textbox', { name: '공통 글감' })).toBeVisible())
})

it('shows one template at a time behind tabs on a phone, each naming its unanswered required questions', async () => {
  stubWidth(false)
  const fixture = createWritingTestStudioFixture({ ...asking, factor: 'template' })
  await materialOfTwoTemplates(fixture)
  const tabs = screen.getAllByRole('tab')
  expect(tabs.map((tab) => tab.textContent)).toEqual([
    'Template A · 필수 1개 남음',
    'Template B · 필수 1개 남음',
  ])
  expect(screen.getByRole('tab', { name: /^Template A/ })).toHaveAttribute('aria-selected', 'true')
  const panel = screen.getByRole('tabpanel', { name: 'Template A' })
  expect(screen.getByRole('tab', { name: /^Template A/ })).toHaveAttribute(
    'aria-controls',
    panel.id,
  )
  expect(within(panel).getByRole('textbox', { name: 'A 장소 필수' })).toBeVisible()
  expect(screen.queryByRole('textbox', { name: /^B 장소/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('heading', { level: 3 })).not.toBeInTheDocument()
  await userEvent.type(within(panel).getByRole('textbox', { name: /^A 장소/ }), '공원')
  expect(screen.getByRole('tab', { name: /^Template A/ })).toHaveTextContent('필수 0개 남음')
  await userEvent.click(screen.getByRole('tab', { name: /^Template B/ }))
  expect(screen.getByRole('textbox', { name: 'B 장소 필수' })).toBeVisible()
  expect(screen.queryByRole('textbox', { name: /^A 장소/ })).not.toBeInTheDocument()
})

it('refuses a cost check with a missing answer in the second tab, names it in the step bar and lands on its field', async () => {
  stubWidth(false)
  const fixture = createWritingTestStudioFixture({ ...asking, factor: 'template' })
  await materialOfTwoTemplates(fixture)
  await userEvent.type(screen.getByRole('textbox', { name: /^A 장소/ }), '공원')
  const cta = screen.getByRole('button', { name: '글 2편 생성 비용 확인' })
  await userEvent.click(cta)
  expect(fixture.estimates).toEqual([])
  expect(fixture.admissions).toEqual([])
  const reason = await within(stepBar()).findByText('‘B 장소’ 항목을 확인해 주세요.')
  // Above the row, and named by the action it holds.
  expect(reason.compareDocumentPosition(cta) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(cta).toHaveAttribute('aria-describedby', reason.id)
  expect(screen.getByRole('tab', { name: /^Template B/ })).toHaveAttribute('aria-selected', 'true')
  const field = screen.getByRole('textbox', { name: 'B 장소 필수' })
  await waitFor(() => expect(field).toHaveFocus())
  expect(field).toHaveAttribute('aria-invalid', 'true')
  const message = document.getElementById(field.getAttribute('aria-describedby')!)!
  expect(message).toHaveTextContent('필수 입력을 확인해 주세요.')
  expect(stepBar()).not.toContainElement(message)
  // Answering it clears the refusal in the field and in the bar.
  await userEvent.type(field, '카페')
  expect(field).not.toHaveAttribute('aria-invalid')
  expect(within(stepBar()).queryByText(/항목/)).not.toBeInTheDocument()
  await postQuote(fixture)
  expect(fixture.estimates).toHaveLength(1)
})

it('counts every refused field in the bar and keeps a bounds refusal under its own field', async () => {
  stubWidth(false)
  const fixture = createWritingTestStudioFixture({ ...asking, factor: 'template' })
  await materialOfTwoTemplates(fixture)
  const length = screen.getByRole('spinbutton', { name: '목표 글자 수 필수' })
  fireEvent.change(length, { target: { value: '5' } })
  await userEvent.click(screen.getByRole('button', { name: '글 2편 생성 비용 확인' }))
  expect(fixture.estimates).toEqual([])
  // On screen order: the length comes before the questions, so it is named first and focused.
  await within(stepBar()).findByText('‘목표 글자 수’ 항목 등 3곳을 확인해 주세요.')
  await waitFor(() => expect(length).toHaveFocus())
  const bounds = document.getElementById(length.getAttribute('aria-describedby')!)!
  expect(bounds).toHaveTextContent('목표 글자 수는')
  expect(stepBar()).not.toContainElement(bounds)
  expect(length).toHaveAttribute('aria-invalid', 'true')
})
