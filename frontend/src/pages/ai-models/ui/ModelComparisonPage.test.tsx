import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it } from 'vitest'
import { Stage } from '@/shared/api'
import { chooseOption } from '@/test/listbox'
import { paidActions, renderLegacyModelRoute } from '@/test/legacy-model-routes'

beforeEach(() => sessionStorage.clear())

it.each([
  ['', 'model', 'write'],
  ['?stage=write', 'model', 'write'],
  ['?stage=observe', 'model', 'observe'],
  ['?stage=voice&voiceId=voice-default', 'voice', 'write'],
  ['?stage=analyze', 'model', 'write'],
] as const)(
  'resolves retired comparison %s into a two-entry canonical test without legacy controls or paid work',
  async (search, factor, stage) => {
    const { router, procedures, fixture } = renderLegacyModelRoute(`/ai-models/compare${search}`, {
      user: { id: 'alice' },
    })
    await screen.findByRole('heading', { level: 1, name: '글쓰기 테스트' })
    expect(router.state.location.pathname).toBe('/tests')
    expect(router.state.location.search).toMatchObject({ factor, stage, count: 2 })
    if (factor === 'voice') expect(router.state.location.search.voiceId).toBe('voice-default')
    const main = within(screen.getByRole('main'))
    expect(
      main.getByRole('button', { name: factor === 'voice' ? '말투' : 'AI 모델' }),
    ).toHaveAttribute('aria-pressed', 'true')
    expect(main.getByRole('combobox', { name: /^비교 형식(?:\s|$)/ })).toHaveTextContent(
      'A/B 테스트',
    )
    expect(main.queryByRole('button', { name: '비교 시작' })).toBeNull()
    expect(main.queryByRole('tab', { name: '문체 분석' })).toBeNull()
    expect(main.queryByRole('button', { name: '후보 추가' })).toBeNull()
    expect(paidActions(procedures)).toEqual([])
    expect(fixture.admissions).toEqual([])
  },
)

it('offers only two, four, eight and sixteen entrants and changes count without preparing or starting work', async () => {
  const user = userEvent.setup(),
    { procedures, fixture } = renderLegacyModelRoute('/ai-models/compare', {
      user: { id: 'alice' },
    })
  const format = await screen.findByRole('combobox', { name: /^비교 형식(?:\s|$)/ })
  await user.click(format)
  expect(
    within(await screen.findByRole('listbox'))
      .getAllByRole('option')
      .map((node) => node.textContent),
  ).toEqual(['A/B 테스트', '4강전', '8강전', '16강전'])
  await user.click(screen.getByRole('option', { name: '16강전' }))
  await user.click(screen.getByRole('button', { name: 'AI 모델' }))
  expect(
    within(screen.getByRole('main')).getAllByRole('combobox', { name: /^후보 \d+(?:\s|$)/ }),
  ).toHaveLength(16)
  expect(fixture.preparations).toEqual([])
  expect(fixture.estimates).toEqual([])
  expect(fixture.admissions).toEqual([])
  expect(paidActions(procedures)).toEqual([])
})

it('preserves an owned source through the alias and preloads its complete recorded input before an explicit cost check', async () => {
  const user = userEvent.setup(),
    { router, procedures, fixture } = renderLegacyModelRoute(
      '/ai-models/compare?stage=write&sourcePostSlug=owned-source&entry=%2Fposts%2Fowned-source',
      { user: { id: 'alice' } },
      { source: true },
    )
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expect(router.state.location.search).toMatchObject({
    source: 'owned-source',
    entry: '/posts/owned-source',
    stage: 'write',
    factor: 'model',
  })
  await user.click(screen.getByRole('button', { name: 'AI 모델' }))
  for (let index = 1; index <= 2; index++)
    await chooseOption(
      user,
      screen.getByRole('combobox', { name: new RegExp(`^후보 ${index}(?:\\s|$)`) }),
      `Model ${index}`,
    )
  await user.click(screen.getByRole('button', { name: '다음' }))
  expect(await screen.findByRole('textbox', { name: '공통 글감' })).toHaveValue(
    'Real recorded material',
  )
  expect(fixture.admissions).toEqual([])
  expect(fixture.estimates).toEqual([])
  expect(procedures).not.toContain('StartWriteExperiment')
  expect(procedures).not.toContain('StartObserveExperiment')
  expect(procedures).not.toContain('StartVoiceReflectionExperiment')
})

it('does not import obsolete five-candidate rankings or auto-save a legacy comparison pair into the canonical actor', async () => {
  const user = userEvent.setup()
  const models = Array.from({ length: 5 }, (_, index) => ({
    providerId: 'stub',
    modelId: `model-${index}`,
    label: `Saved ${index + 1}`,
  }))
  const { procedures } = renderLegacyModelRoute('/ai-models/compare?stage=write&count=5', {
    user: { id: 'alice' },
    providers: {
      models,
      comparisonPairs: [
        {
          stage: Stage.WRITE,
          candidateA: models[0],
          candidateB: models[1],
          extraCandidates: models.slice(2),
        },
      ],
    },
  })
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expect(screen.getByRole('combobox', { name: /^비교 형식(?:\s|$)/ })).toHaveTextContent(
    'A/B 테스트',
  )
  await user.click(screen.getByRole('button', { name: 'AI 모델' }))
  expect(screen.getAllByRole('combobox', { name: /^후보 \d+(?:\s|$)/ })).toHaveLength(2)
  expect(procedures).not.toContain('SaveComparisonPair')
  expect(paidActions(procedures)).toEqual([])
})

it('protects source context before canonical hydration when signed out', async () => {
  const { router, procedures } = renderLegacyModelRoute(
    '/ai-models/compare?stage=observe&source=owned-source',
  )
  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  const destination = new URL(String(router.state.location.search.redirect), 'http://localhost')
  expect(destination.pathname).toBe('/ai-models/compare')
  expect(destination.searchParams.get('source')).toBe('owned-source')
  expect(procedures).not.toContain('GetPost')
  expect(procedures).not.toContain('ListWritingTests')
  expect(paidActions(procedures)).toEqual([])
})
