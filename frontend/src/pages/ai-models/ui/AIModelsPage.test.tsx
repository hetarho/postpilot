import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ExperimentSource, PostCreditsBasis, ProtoPlan, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeExperimentsOptions } from '@/test/experiments'
import { chooseOption } from '@/test/listbox'
import { paidActions, renderLegacyModelRoute } from '@/test/legacy-model-routes'

beforeEach(() => sessionStorage.clear())

afterEach(() => initializeI18n('ko'))

const destinations = [
  ['/ai-models', '/ai-models', '모델 변경', 'Change models'],
  ['/ai-models/compare', '/tests', '글쓰기 테스트', 'Writing tests'],
  ['/ai-models/experiments', '/tests/history', '테스트 기록', 'Test history'],
  ['/ai-models/leaderboard', '/tests/history', '테스트 기록', 'Test history'],
] as const

it.each(['ko', 'en'] as const)(
  'keeps model settings separate and resolves retired entries into canonical tests without paid work in %s',
  async (locale) => {
    initializeI18n(locale)
    const { router, procedures } = renderLegacyModelRoute('/ai-models', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
    })
    const main = within(await screen.findByRole('main'))
    expect(main.getAllByRole('combobox')).toHaveLength(3)
    expect(
      main.getByRole('combobox', {
        name: locale === 'ko' ? /문체 분석 모델/ : /Analyze voice model/,
      }),
    ).toBeVisible()
    expect(procedures).not.toContain('ListWritingTests')
    for (const [oldPath, canonical, ko, en] of destinations) {
      await act(() => router.navigate({ to: oldPath }))
      await waitFor(() => expect(router.state.location.pathname).toBe(canonical))
      await screen.findByRole('heading', { level: 1, name: locale === 'ko' ? ko : en })
      expect(
        screen.queryByRole('button', { name: locale === 'ko' ? '비교 시작' : 'Start comparison' }),
      ).toBeNull()
      expect(
        screen.queryByRole('heading', {
          name: locale === 'ko' ? '추천 조합' : 'Recommended sets',
        }) !== null,
      ).toBe(canonical === '/ai-models')
    }
    expect(
      paidActions(procedures).filter((name) => name !== 'InitializeDefaultSelections'),
    ).toEqual([])
  },
)

it('saves an active model only after a model change, separately from comparison candidates', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      calls,
      models: [
        {
          providerId: 'openrouter',
          modelId: 'vision-default',
          label: 'Recommended Vision',
          vision: true,
        },
        { providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true },
      ],
    },
  })
  const main = within(await screen.findByRole('main'))
  await waitFor(() =>
    expect(main.getByRole('combobox', { name: /관찰/ })).toHaveTextContent('Recommended Vision'),
  )
  expect(calls).not.toContain('SaveSelection')
  await chooseOption(user, main.getByRole('combobox', { name: /관찰/ }), 'Vision')
  await waitFor(() => expect(calls).toContain('SaveSelection'))
  expect(calls.filter((call) => call === 'SaveSelection')).toHaveLength(1)
  expect(main.getByRole('combobox', { name: /관찰/ })).toHaveTextContent('Vision')
  expect(calls).not.toContain('SaveComparisonPair')
  expect(main.queryByRole('button', { name: '비교 시작' })).not.toBeInTheDocument()
})

// QUOTA-66, MODEL-27: the selected model's line says what it can hold, never a supplier price.
it('describes a chosen model without any price', async () => {
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true }],
    },
  })
  const main = within(await screen.findByRole('main'))
  await waitFor(() =>
    expect(main.getByRole('combobox', { name: /관찰/ })).toHaveTextContent('Vision'),
  )
  const selectedField = within(
    main.getByRole('combobox', { name: /관찰/ }).parentElement!.parentElement!,
  )
  expect(await selectedField.findByText('컨텍스트 0')).toBeInTheDocument()
  expect(main.queryByText(/\$|1M 토큰|가격 미확인/)).not.toBeInTheDocument()
})

// QUOTA-64: under each stage the chosen model says what one post costs that stage in credits,
// and the page prices the saved pair and how many posts the balance covers.
it('shows per-post credits for the chosen model and the saved pair', async () => {
  const user = userEvent.setup()
  renderAppAt('/ai-models', {
    user: { id: 'alice', plan: ProtoPlan.BASIC },
    plans: { plan: ProtoPlan.BASIC, balance: { credits: 90, unlimited: false } },
    providers: {
      models: [
        {
          providerId: 'openrouter',
          modelId: 'eyes',
          label: 'Eyes',
          vision: true,
          postCredits: [
            { stage: Stage.OBSERVE, credits: 20, basis: PostCreditsBasis.RECENT_USAGE },
          ],
        },
        {
          providerId: 'openrouter',
          modelId: 'pen',
          label: 'Pen',
          postCredits: [{ stage: Stage.WRITE, credits: 10, basis: PostCreditsBasis.ESTIMATE }],
        },
      ],
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'eyes' }],
    },
  })
  const main = within(await screen.findByRole('main'))
  expect(
    await main.findByText('컨텍스트 0 · 최근 사용량 기준 글 1개당 약 20크레딧'),
  ).toBeInTheDocument()
  // With no write model yet there is no post to price.
  expect(main.queryByText(/남은 크레딧으로/)).not.toBeInTheDocument()
  await chooseOption(user, main.getByRole('combobox', { name: /작성/ }), 'Pen')
  expect(await main.findByText('컨텍스트 0 · 예상 글 1개당 약 10크레딧')).toBeInTheDocument()
  expect(await main.findByText('남은 크레딧으로 약 3편 쓸 수 있어요')).toBeInTheDocument()
  expect(main.getByText(/예상 글 1개당 약 30크레딧/)).toBeInTheDocument()
})

// The operator is exempt from credits, so the page names what a post costs everyone else and
// counts no posts against a balance it never spends.
it('shows the operator the per-post figure without a posts count', async () => {
  renderAppAt('/ai-models', {
    user: { id: 'root', plan: ProtoPlan.MASTER },
    plans: { plan: ProtoPlan.MASTER, balance: { credits: 0, unlimited: true } },
    providers: {
      models: [
        {
          providerId: 'openrouter',
          modelId: 'pen',
          label: 'Pen',
          postCredits: [{ stage: Stage.WRITE, credits: 10, basis: PostCreditsBasis.RECENT_USAGE }],
        },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'pen' }],
    },
  })
  const main = within(await screen.findByRole('main'))
  expect(await main.findByText('최근 사용량 기준 글 1개당 약 10크레딧')).toBeInTheDocument()
  expect(main.queryByText(/남은 크레딧으로/)).not.toBeInTheDocument()
})

it('preserves stage/source in canonical history, browser Back and retained paid detail without a new comparison', async () => {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  const { router, procedures } = renderLegacyModelRoute(
    '/ai-models/experiments?stage=write&source=first-post',
    {
      user: { id: 'alice' },
      experiments: {
        reads,
        history: [{ id: 'writing-1', stage: Stage.WRITE, postSlug: 'first-post' }],
      },
    },
  )
  await screen.findByRole('heading', { level: 1, name: '테스트 기록' })
  expect(router.state.location.pathname).toBe('/tests/history')
  expect(router.state.location.search).toMatchObject({ stage: 'write', source: 'first-post' })
  const legacy = within(screen.getByRole('region', { name: '이전 유료 비교 기록' }))
  const href = (await legacy.findByRole('link', { name: '계속 보기' })).getAttribute('href')!
  const url = new URL(href, 'http://localhost')
  expect(url.pathname).toBe('/tests/records/writing-1')
  expect(url.searchParams.get('entry')).toBe('/tests/history?stage=write&source=first-post')
  await act(() => router.navigate({ href }))
  expect(
    await within(screen.getByRole('main')).findByRole('link', { name: '테스트 기록' }),
  ).toHaveAttribute('href', '/tests/history?stage=write&source=first-post')
  await act(() => router.history.back())
  await screen.findByRole('heading', { level: 1, name: '테스트 기록' })
  expect(router.state.location.search).toMatchObject({ stage: 'write', source: 'first-post' })
  expect(reads).toContainEqual(
    expect.objectContaining({ kind: 'history', stage: Stage.WRITE, source: ExperimentSource.POST }),
  )
  expect(paidActions(procedures)).toEqual([])
})

it.each([
  '/ai-models/experiments?stage=invalid',
  '/ai-models/leaderboard?stage=invalid',
  '/ai-models/experiments?stage=analyze',
  '/ai-models/leaderboard?stage=analyze',
])(
  'drops unsupported history stages at %s without querying the retired analysis comparison',
  async (path) => {
    const reads: NonNullable<FakeExperimentsOptions['reads']> = []
    const { router, procedures } = renderLegacyModelRoute(path, {
      user: { id: 'alice' },
      experiments: { reads },
    })
    await screen.findByRole('heading', { level: 1, name: '테스트 기록' })
    expect(router.state.location.pathname).toBe('/tests/history')
    expect(router.state.location.search.stage).toBeUndefined()
    await waitFor(() =>
      expect(reads).toContainEqual(
        expect.objectContaining({ kind: 'history', stage: Stage.UNSPECIFIED }),
      ),
    )
    expect(reads.map((read) => read.stage)).not.toContain(Stage.ANALYZE)
    expect(procedures).not.toContain('GetLeaderboard')
    expect(paidActions(procedures)).toEqual([])
  },
)

it.each(['/ai-models/experiments', '/ai-models/leaderboard'])(
  'keeps loading and failed paid reads distinct from empty canonical history at %s',
  async (path) => {
    let release!: () => void
    const readGate = new Promise<void>((resolve) => {
      release = resolve
    })
    const { procedures } = renderLegacyModelRoute(
      path,
      {
        user: { id: 'alice' },
        experiments: { readGate, listFails: true },
      },
      { readGate, listFails: true },
    )
    expect(await screen.findAllByRole('status')).not.toHaveLength(0)
    expect(
      screen
        .getAllByRole('status')
        .some((node) => node.textContent?.includes('내용을 확인하고 있어요')),
    ).toBe(true)
    expect(screen.queryByText('아직 테스트 기록이 없어요.')).toBeNull()
    await act(async () => release())
    await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(2))
    screen
      .getAllByRole('alert')
      .forEach((node) => expect(node).toHaveTextContent('요청을 완료하지 못했어요'))
    expect(screen.queryByText('아직 테스트 기록이 없어요.')).toBeNull()
    expect(screen.getAllByRole('button', { name: '현재 결과 확인' })).toHaveLength(2)
    expect(paidActions(procedures)).toEqual([])
  },
)

it.each(destinations)(
  'guards direct access to %s before canonical data or paid work is requested',
  async (path) => {
    const { router, procedures } = renderLegacyModelRoute(path)
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    const redirect = new URL(String(router.state.location.search.redirect), 'http://localhost')
    expect(redirect.pathname).toBe(path)
    expect(procedures).not.toContain('ListWritingTests')
    expect(procedures).not.toContain('ListExperiments')
    expect(paidActions(procedures)).toEqual([])
  },
)

it('filters legacy voice and post sources in canonical history, keeping owner reads and browser Back', async () => {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  const { router, procedures } = renderLegacyModelRoute(
    '/ai-models/experiments?stage=voice&voiceId=voice-default',
    {
      user: { id: 'alice' },
      experiments: {
        reads,
        history: [
          { id: 'writing-1', stage: Stage.WRITE, postSlug: 'first-post' },
          {
            id: 'reflection-1',
            stage: Stage.WRITE,
            voiceId: 'voice-default',
            source: ExperimentSource.VOICE,
          },
        ],
      },
    },
  )
  await screen.findByRole('heading', { level: 1, name: '테스트 기록' })
  const legacy = within(screen.getByRole('region', { name: '이전 유료 비교 기록' }))
  await waitFor(() => expect(legacy.getAllByRole('link', { name: '계속 보기' })).toHaveLength(1))
  expect(legacy.getByRole('link', { name: '계속 보기' })).toHaveAttribute(
    'href',
    expect.stringContaining('/reflection-1?entry='),
  )
  expect(reads).toContainEqual(
    expect.objectContaining({
      kind: 'history',
      stage: Stage.WRITE,
      source: ExperimentSource.VOICE,
    }),
  )
  await act(() =>
    router.navigate({ to: '/tests/history', search: { stage: 'write', source: 'first-post' } }),
  )
  await waitFor(() =>
    expect(
      within(screen.getByRole('region', { name: '이전 유료 비교 기록' })).getByRole('link', {
        name: '계속 보기',
      }),
    ).toHaveAttribute('href', expect.stringContaining('/writing-1?entry=')),
  )
  expect(reads).toContainEqual(
    expect.objectContaining({ kind: 'history', stage: Stage.WRITE, source: ExperimentSource.POST }),
  )
  await act(() => router.history.back())
  await waitFor(() =>
    expect(router.state.location.search).toMatchObject({
      stage: 'voice',
      voiceId: 'voice-default',
    }),
  )
  await waitFor(() =>
    expect(
      within(screen.getByRole('region', { name: '이전 유료 비교 기록' })).getByRole('link', {
        name: '계속 보기',
      }),
    ).toHaveAttribute('href', expect.stringContaining('/reflection-1?entry=')),
  )
  expect(paidActions(procedures)).toEqual([])
})

it('keeps a voice-context leaderboard alias in voice-filtered paid history rather than requesting ranks', async () => {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  const { router, procedures } = renderLegacyModelRoute('/ai-models/leaderboard?stage=voice', {
    user: { id: 'alice' },
    experiments: { reads },
  })
  await screen.findByRole('heading', { level: 1, name: '테스트 기록' })
  expect(router.state.location.search.stage).toBe('voice')
  await waitFor(() =>
    expect(reads).toContainEqual(
      expect.objectContaining({
        kind: 'history',
        stage: Stage.WRITE,
        source: ExperimentSource.VOICE,
      }),
    ),
  )
  expect(procedures).not.toContain('GetLeaderboard')
  expect(screen.queryByRole('tab', { name: '주간' })).toBeNull()
  expect(paidActions(procedures)).toEqual([])
})

// MODEL-71: every set the operator curated is offered, in their order, each with its own apply
// control; the set id is the server's handle and is never shown.
it('offers every recommended set in the operator order', async () => {
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      recommendationSets: [
        {
          id: 'set-free',
          label: 'Free start',
          selections: [
            { stage: Stage.ANALYZE, active: { providerId: 'openrouter', modelId: 'a' } },
          ],
        },
        {
          id: 'set-top',
          label: 'Best quality',
          selections: [
            { stage: Stage.ANALYZE, active: { providerId: 'openrouter', modelId: 'b' } },
          ],
        },
      ],
    },
  })
  const section = within(
    (await screen.findByRole('heading', { name: '추천 조합' })).closest('section')!,
  )
  const items = await section.findAllByRole('listitem')
  expect(
    items.map((item) => within(item).getByText(/Free start|Best quality/).textContent),
  ).toEqual(['Free start', 'Best quality'])
  expect(section.getAllByRole('button', { name: '추천 조합 적용' })).toHaveLength(2)
  expect(section.queryByText('set-free')).not.toBeInTheDocument()
})

it('says there is no recommended set rather than loading forever', async () => {
  renderAppAt('/ai-models', { user: { id: 'alice' }, providers: { recommendationSets: [] } })
  expect(await screen.findByText('아직 추천 조합이 없어요.')).toBeInTheDocument()
  expect(screen.queryByText('추천 조합을 불러오는 중…')).not.toBeInTheDocument()
})

it('reports a failed recommendations read as a failure', async () => {
  renderAppAt('/ai-models', { user: { id: 'alice' }, providers: { recommendationsFail: true } })
  expect(await screen.findByText('추천 조합을 불러오지 못했어요.')).toBeInTheDocument()
})
