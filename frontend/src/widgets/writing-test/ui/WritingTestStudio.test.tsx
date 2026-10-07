import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoConfigurationKind, WritingTestPublicationAction } from '@/shared/api'
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
function mount(fixture: Fixture, ownerId = 'alice') {
  const onOpenTest = vi.fn()
  const seedKey = 'host-test'
  const mounted = render(
    <WritingTestStudio
      ownerId={ownerId}
      seedKey={seedKey}
      initialPlan={fixture.plan}
      onOpenTest={onOpenTest}
    />,
    { wrapper: withProviders(fixture.transport, createTestQueryClient()) },
  )
  return { ...mounted, onOpenTest, seedKey }
}
async function select(label: string, option: string) {
  await userEvent.click(screen.getByRole('combobox', { name: new RegExp(`^${label}(?:\\s|$)`) }))
  await userEvent.click(screen.getByRole('option', { name: option }))
}
async function modelCandidates(fixture: Fixture) {
  await userEvent.click(await screen.findByRole('button', { name: 'AI 모델' }))
  await waitFor(() => expect(fixture.calls).toContain('GetSelections'))
  // Picking independently in reverse order catches sparse or shifted slot representations.
  for (let index = fixture.plan.count - 1; index >= 0; index--)
    await select(`후보 ${index + 1}`, `Model ${index + 1}`)
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await screen.findByRole('textbox', { name: '공통 글감' })
}
async function generate(fixture: Fixture) {
  await userEvent.click(
    screen.getByRole('button', { name: `글 ${fixture.plan.count}편 생성 비용 확인` }),
  )
  await screen.findByRole('button', { name: `글 ${fixture.plan.count}편 만들기` })
  expect(fixture.admissions).toHaveLength(0)
  await userEvent.click(screen.getByRole('button', { name: `글 ${fixture.plan.count}편 만들기` }))
  await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
}
async function decideAll(fixture: Fixture) {
  for (let index = 0; index < fixture.plan.count - 1; index++) {
    await waitFor(() =>
      expect(
        screen
          .getAllByRole('button', { name: /를 승자로 선택/ })
          .every((button) => !button.hasAttribute('disabled')),
      ).toBe(true),
    )
    await userEvent.click(screen.getAllByRole('button', { name: /를 승자로 선택/ })[0])
    await waitFor(() => expect(fixture.votes).toHaveLength(index + 1))
  }
  await screen.findByText('Frozen setting 1')
}

it('requires every chosen slot and preserves unsent material through Back without implicit AI', async () => {
  const fixture = createWritingTestStudioFixture()
  mount(fixture)
  await userEvent.click(await screen.findByRole('button', { name: 'AI 모델' }))
  await select('후보 2', 'Model 2')
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(screen.getByText('서로 다른 후보 2개를 선택해 주세요.')).toBeVisible()
  expect(screen.queryByRole('textbox', { name: '공통 글감' })).not.toBeInTheDocument()
  await select('후보 1', 'Model 1')
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  const material = await screen.findByRole('textbox', { name: '공통 글감' })
  await userEvent.clear(material)
  await userEvent.type(material, 'My unsent working material')
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(screen.getByRole('textbox', { name: '공통 글감' })).toHaveValue(
    'My unsent working material',
  )
  expect(fixture.estimates).toHaveLength(0)
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.preparations).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})
it('returns a known refused quote to editable inputs while preserving both candidates and material without starting work', async () => {
  const fixture = createWritingTestStudioFixture({ estimateFailure: true })
  mount(fixture)
  await modelCandidates(fixture)
  const material = screen.getByRole('textbox', { name: '공통 글감' })
  await userEvent.clear(material)
  await userEvent.type(material, 'Keep this working material after the refused estimate')
  await userEvent.click(screen.getByRole('button', { name: '글 2편 생성 비용 확인' }))
  await userEvent.click(await screen.findByRole('button', { name: '입력 수정하기' }))
  expect(await screen.findByRole('textbox', { name: '공통 글감' })).toHaveValue(
    'Keep this working material after the refused estimate',
  )
  expect(fixture.estimates).toHaveLength(1)
  expect(fixture.admissions).toHaveLength(0)
  expect(screen.queryByRole('button', { name: '같은 요청 다시 확인하기' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '이전 단계' }))
  expect(screen.getByRole('combobox', { name: /^후보 1 Model 1$/ })).toBeVisible()
  expect(screen.getByRole('combobox', { name: /^후보 2 Model 2$/ })).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  expect(fixture.estimates).toHaveLength(1)
  await userEvent.click(screen.getByRole('button', { name: '글 2편 생성 비용 확인' }))
  await screen.findByRole('button', { name: '글 2편 만들기' })
  expect(fixture.estimates).toHaveLength(2)
  expect(fixture.estimates[1]).toMatchObject({
    count: 2,
    context: { material: { text: 'Keep this working material after the refused estimate' } },
  })
  expect(fixture.estimates[1].entrants).toEqual(fixture.estimates[0].entrants)
  expect(fixture.admissions).toHaveLength(0)
  expect(fixture.preparations).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)
})
it('generates sixteen outputs once, sends fifteen human decisions, then adopts only through explicit review and confirmation', async () => {
  const fixture = createWritingTestStudioFixture({ count: 16 })
  const { onOpenTest } = mount(fixture)
  await modelCandidates(fixture)
  await generate(fixture)
  const outputs = fixture
    .getTest()!
    .candidates.map((candidate) => candidate.output?.blocks.map((block) => block.content))
  expect(fixture.estimates[0]).toMatchObject({
    count: 16,
    context: { material: { text: 'Same fictional writing material', fictional: true } },
  })
  expect(fixture.admissions[0].plan?.entrants).toHaveLength(16)
  expect(fixture.publications).toHaveLength(0)
  expect(screen.queryByText('Frozen setting 1')).not.toBeInTheDocument()
  await decideAll(fixture)
  expect(fixture.admissions).toHaveLength(1)
  expect(fixture.preparations).toHaveLength(0)
  expect(
    fixture
      .getTest()!
      .candidates.map((candidate) => candidate.output?.blocks.map((block) => block.content)),
  ).toEqual(outputs)
  expect(fixture.publications).toHaveLength(0)
  expect(onOpenTest).toHaveBeenCalledTimes(1)
  await userEvent.click(screen.getByRole('button', { name: '활성 모델로 변경하기' }))
  expect(fixture.publications).toHaveLength(0)
  await userEvent.click(screen.getByRole('button', { name: '활성 모델로 변경하기' }))
  await waitFor(() => expect(fixture.publications).toHaveLength(1))
  expect(fixture.publications[0]).toMatchObject({
    action: WritingTestPublicationAction.ADOPT_MODEL,
    winnerCandidateId: 'candidate-0',
    makeDefault: false,
  })
  await screen.findByText(/Frozen setting 1.*사용할 수 있어요/)
  await waitFor(() =>
    expect(fixture.calls.filter((name) => name === 'GetSelections').length).toBeGreaterThan(1),
  )
})
it.each([
  {
    factor: 'voice' as const,
    label: '말투',
    count: 16 as const,
    kind: ProtoConfigurationKind.WRITING_VOICE,
  },
  {
    factor: 'template' as const,
    label: '글 템플릿',
    count: 8 as const,
    kind: ProtoConfigurationKind.POST_TEMPLATE,
  },
  {
    factor: 'guideline' as const,
    label: '글 지침',
    count: 4 as const,
    kind: ProtoConfigurationKind.POST_GUIDELINE,
  },
])(
  'prepares $count unsaved $factor candidates and quotes real posts separately without a seed',
  async ({ factor, label, count, kind }) => {
    const fixture = createWritingTestStudioFixture({ count, factor })
    mount(fixture)
    await userEvent.click(await screen.findByRole('button', { name: label }))
    await userEvent.type(
      screen.getByRole('textbox', { name: '후보의 방향 (선택)' }),
      'Distinct approaches to the same shared scenario',
    )
    await waitFor(() => expect(screen.getByRole('button', { name: '다음' })).toBeEnabled())
    expect(fixture.preparations).toHaveLength(0)
    await userEvent.click(screen.getByRole('button', { name: '다음' }))
    await screen.findByRole('button', { name: `AI로 ${count}칸 채우고 계속` })
    expect(fixture.preparationEstimates).toEqual([{ kind, count }])
    expect(fixture.calls).not.toContain('CreateAuthoringSession')
    expect(fixture.admissions).toHaveLength(0)
    await userEvent.click(screen.getByRole('button', { name: `AI로 ${count}칸 채우고 계속` }))
    await screen.findByText(`${count}개 후보가 준비됐어요`)
    expect(fixture.preparations).toHaveLength(1)
    expect(fixture.preparations[0]).toMatchObject({ kind, count })
    expect(fixture.admissions).toHaveLength(0)
    await userEvent.click(
      await screen.findByRole('button', { name: `글 ${count}편 생성 비용 확인` }),
    )
    await screen.findByRole('button', { name: `글 ${count}편 만들기` })
    expect(fixture.estimates[0].entrants).toHaveLength(count)
    expect(
      fixture.estimates[0].entrants.every(
        (entrant) => entrant.source.case === 'authoringCandidate',
      ),
    ).toBe(true)
    expect(fixture.estimates[0].entrants[0].source).toMatchObject({
      case: 'authoringCandidate',
      value: { sessionId: 'prepared-alice', candidateId: 'prepared-candidate-0', revision: 2 },
    })
    expect(fixture.admissions).toHaveLength(0)
    expect(fixture.publications).toHaveLength(0)
  },
)
it('retains real source revisions for explicit compatible output application and does not apply at champion selection', async () => {
  const fixture = createWritingTestStudioFixture({ source: true })
  mount(fixture)
  await modelCandidates(fixture)
  expect(screen.getByRole('textbox', { name: '공통 글감' })).toHaveValue('Real recorded material')
  await generate(fixture)
  await decideAll(fixture)
  expect(fixture.applications).toHaveLength(0)
  await userEvent.click(screen.getByRole('button', { name: '원본 글에 결과 적용' }))
  await waitFor(() => expect(fixture.applications).toHaveLength(1))
  expect(fixture.applications[0]).toMatchObject({
    expectedInputRevision: 9007199254740993n,
    expectedContentRevision: 7n,
    winnerCandidateId: 'candidate-0',
  })
  expect(fixture.publications).toHaveLength(0)
})
it('saves a synthetic style only after an explicit valid name and default choice', async () => {
  const fixture = createWritingTestStudioFixture({ factor: 'voice' })
  mount(fixture)
  await userEvent.click(await screen.findByRole('button', { name: '말투' }))
  await userEvent.type(
    screen.getByRole('textbox', { name: '후보의 방향 (선택)' }),
    'Two distinct reusable styles',
  )
  await waitFor(() => expect(screen.getByRole('button', { name: '다음' })).toBeEnabled())
  await userEvent.click(screen.getByRole('button', { name: '다음' }))
  await userEvent.click(await screen.findByRole('button', { name: 'AI로 2칸 채우고 계속' }))
  await screen.findByText('2개 후보가 준비됐어요')
  await generate(fixture)
  await decideAll(fixture)
  expect(fixture.publications).toHaveLength(0)
  await userEvent.click(screen.getByRole('button', { name: '우승 말투 저장하기' }))
  expect(
    screen.getByText('AI가 만든 말투로 저장해요. 개인 학습 자료로 들어가지 않아요.'),
  ).toBeVisible()
  const name = screen.getByRole('textbox', { name: '저장할 이름' })
  await userEvent.clear(name)
  expect(name).toHaveValue('')
  await userEvent.click(screen.getByRole('button', { name: '저장할 이름을 입력해 주세요' }))
  expect(fixture.publications).toHaveLength(0)
  expect(screen.getByText('필수 입력을 확인해 주세요.')).toBeVisible()
  await userEvent.type(name, 'My chosen style')
  await userEvent.click(screen.getByRole('checkbox', { name: '기본 말투로 사용하기' }))
  await userEvent.click(screen.getByRole('button', { name: '“My chosen style” 저장하기' }))
  await waitFor(() => expect(fixture.publications).toHaveLength(1))
  expect(fixture.publications[0]).toMatchObject({
    action: WritingTestPublicationAction.SAVE_SETTING,
    name: 'My chosen style',
    makeDefault: true,
    winnerCandidateId: 'candidate-0',
  })
  await screen.findByText(/My chosen style.*사용할 수 있어요/)
  expect(fixture.preparations).toHaveLength(1)
  expect(fixture.admissions).toHaveLength(1)
})
it('fences an old owner pending paid admission when another owner opens the same host', async () => {
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const fixture = createWritingTestStudioFixture({ generationGate: gate })
  const mounted = mount(fixture)
  await modelCandidates(fixture)
  await userEvent.click(screen.getByRole('button', { name: '글 2편 생성 비용 확인' }))
  await userEvent.click(await screen.findByRole('button', { name: '글 2편 만들기' }))
  await waitFor(() => expect(fixture.admissions).toHaveLength(1))
  fixture.setOwner('bob')
  mounted.rerender(
    <WritingTestStudio
      ownerId="bob"
      seedKey={mounted.seedKey}
      initialPlan={fixture.plan}
      onOpenTest={mounted.onOpenTest}
    />,
  )
  await screen.findByRole('button', { name: 'AI 모델' })
  await act(async () => {
    release()
    await gate
  })
  expect(screen.queryByRole('button', { name: /승자로 선택/ })).not.toBeInTheDocument()
  expect(mounted.onOpenTest).not.toHaveBeenCalled()
  expect(fixture.admissions).toHaveLength(1)
  expect(fixture.calls).not.toContain('CancelWritingTest')
})
