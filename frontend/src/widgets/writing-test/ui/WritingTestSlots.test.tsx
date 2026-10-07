import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoGuidelineScope } from '@/shared/api'
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
afterEach(cleanup)
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
async function select(label: string, option: string) {
  await userEvent.click(screen.getByRole('combobox', { name: new RegExp(`^${label}(?:\\s|$)`) }))
  await userEvent.click(await screen.findByRole('option', { name: option }))
}
async function enter(fixture: Fixture, label: string) {
  await userEvent.click(await screen.findByRole('button', { name: label }))
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
  await screen.findByRole('textbox', { name: '공통 글감' })
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
    await screen.findByRole('textbox', { name: '공통 글감' })
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
  first.unmount()
  mount(fixture)
  // Cost quotes do not survive reload as approval; the frozen contestants and material do.
  await screen.findByRole('textbox', { name: '공통 글감' })
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
  await screen.findByRole('textbox', { name: '공통 글감' })
  const shortened = await postQuote(fixture)
  expect(shortened.entrants).toEqual(references)
  expect(fixture.preparations.map((batch) => batch.count)).toEqual([1, 2])
})

it('retains required fields from a previous prepared template after a second batch and reload', async () => {
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
  first.unmount()
  mount(fixture)
  const firstField = await screen.findByRole('textbox', { name: /^첫 장소/ })
  const secondField = await screen.findByRole('textbox', { name: /^두 번째 장소/ })
  expect(fixture.preparations.map((batch) => batch.count)).toEqual([1, 1])
  await userEvent.click(screen.getByRole('button', { name: '글 2편 생성 비용 확인' }))
  expect(fixture.estimates).toHaveLength(0)
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
