import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ExperimentSource, PostCreditsBasis, ProtoPlan, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeExperimentsOptions } from '@/test/experiments'
import { paidActions, renderLegacyModelRoute } from '@/test/legacy-model-routes'

beforeEach(() => sessionStorage.clear())

afterEach(() => initializeI18n('ko'))

/** Guide: each stage is a row whose 바꾸기 opens a picker sheet of radio cards; a tap saves. */
async function pick(user: ReturnType<typeof userEvent.setup>, stage: string, model: RegExp) {
  await user.click(screen.getByRole('button', { name: `바꾸기 · ${stage}` }))
  const sheet = within(await screen.findByRole('dialog', { name: `${stage} AI 고르기` }))
  await user.click(await sheet.findByRole('radio', { name: model }))
  return sheet
}

const destinations = [
  ['/ai-models', '/ai-models', 'AI 고르기', 'Pick AI'],
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
    // Three plain stage rows — 글 쓰기 · 사진 보기 · 말투 분석 — each with its own 바꾸기.
    expect(
      main.getAllByRole('button', { name: locale === 'ko' ? /^바꾸기 · / : /^Change · / }),
    ).toHaveLength(3)
    expect(
      main.getByRole('button', {
        name: locale === 'ko' ? '바꾸기 · 말투 분석' : 'Change · Learning your style',
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
  const observeRow = () =>
    within(main.getByRole('button', { name: '바꾸기 · 사진 보기' }).parentElement!)
  await waitFor(() => expect(observeRow().getByText('Recommended Vision')).toBeInTheDocument())
  expect(calls).not.toContain('SaveSelection')
  const sheet = await pick(user, '사진 보기', /^(?!.*Recommended).*Vision/)
  await waitFor(() => expect(calls).toContain('SaveSelection'))
  expect(calls.filter((call) => call === 'SaveSelection')).toHaveLength(1)
  await waitFor(() =>
    expect(sheet.getByRole('radio', { name: /^(?!.*Recommended).*Vision/ })).toHaveAttribute(
      'aria-checked',
      'true',
    ),
  )
  await user.click(sheet.getByRole('button', { name: '완료' }))
  await waitFor(() => expect(observeRow().getByText('Vision')).toBeInTheDocument())
  expect(calls).not.toContain('SaveComparisonPair')
  expect(main.queryByRole('button', { name: '비교 시작' })).not.toBeInTheDocument()
})

it('offers named common test entries and optional A/B settings without changing active models or retained legacy extras', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      calls,
      models: [
        { providerId: 'openrouter', modelId: 'a', label: 'Model A', vision: true },
        { providerId: 'openrouter', modelId: 'b', label: 'Model B', vision: true },
        { providerId: 'openrouter', modelId: 'legacy-c', label: 'Retained C', vision: true },
      ],
      comparisonPairs: [
        {
          stage: Stage.WRITE,
          candidateA: { providerId: 'openrouter', modelId: 'a' },
          candidateB: { providerId: 'openrouter', modelId: 'b' },
          extraCandidates: [{ providerId: 'openrouter', modelId: 'legacy-c' }],
        },
      ],
    },
  })
  const main = within(await screen.findByRole('main'))
  for (const [name, stage] of [
    ['비교해 보기', 'write'],
    ['사진 보는 AI 비교', 'observe'],
  ] as const) {
    const link = main.getByRole('link', { name })
    const url = new URL(link.getAttribute('href')!, 'https://postpilot.test')
    expect(url.pathname).toBe('/tests')
    expect(Object.fromEntries(url.searchParams)).toMatchObject({
      factor: 'model',
      stage,
      count: '2',
      entry: '/ai-models',
    })
  }
  expect(main.getByRole('link', { name: '지난 비교' })).toHaveAttribute(
    'href',
    '/tests/history?entry=%2Fai-models',
  )
  expect(main.getAllByRole('button', { name: /^바꾸기 · / })).toHaveLength(3)
  await user.click(main.getByRole('button', { name: '두 모델 테스트의 기본 후보 (선택)' }))
  const pair = within(await main.findByRole('region', { name: '글 작성 두 모델 후보' }))
  await waitFor(() =>
    expect(pair.getByRole('combobox', { name: /^후보 A(?:\s|$)/ })).toHaveTextContent('Model A'),
  )
  expect(pair.getByRole('combobox', { name: /^후보 B(?:\s|$)/ })).toHaveTextContent('Model B')
  expect(main.queryByRole('combobox', { name: '후보 C' })).toBeNull()
  expect(main.queryByRole('link', { name: /리더보드/ })).toBeNull()
  expect(calls.filter((name) => /^(Save|Start|Estimate|Adopt|Apply)/.test(name))).toEqual([])
})

// MODEL-96 →MODEL-57: the picker opens on the grade order — 무료, then 가성비 up to 최고, the
// unclassified last, the catalog's order inside a grade — with none of the named lenses on; 품질
// 먼저 reorders only once the owner picks it.
it('opens the stage picker free first and applies 품질 먼저 only when picked', async () => {
  const user = userEvent.setup()
  const graded = (modelId: string, level?: 'free' | 'value' | 'balanced' | 'premium' | 'top') => ({
    providerId: 'openrouter',
    modelId,
    label: modelId,
    levels: level ? { [Stage.WRITE]: level } : {},
  })
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      models: [
        graded('Top', 'top'),
        graded('Plain'),
        graded('Balanced', 'balanced'),
        graded('Value One', 'value'),
        graded('Free', 'free'),
        graded('Value Two', 'value'),
      ],
    },
  })
  const main = within(await screen.findByRole('main'))
  await waitFor(() => expect(main.getByRole('button', { name: '바꾸기 · 글 쓰기' })).toBeEnabled())
  await user.click(main.getByRole('button', { name: '바꾸기 · 글 쓰기' }))
  const sheet = within(await screen.findByRole('dialog', { name: '글 쓰기 AI 고르기' }))
  const order = () =>
    sheet
      .getAllByRole('radio')
      .map(
        (radio) => radio.textContent?.match(/(Free|Value One|Value Two|Balanced|Top|Plain)/)?.[0],
      )
  await waitFor(() =>
    expect(order()).toEqual(['Free', 'Value One', 'Value Two', 'Balanced', 'Top', 'Plain']),
  )
  for (const lens of ['품질 먼저', '균형', '가격 먼저'])
    expect(sheet.getByRole('tab', { name: lens })).toHaveAttribute('aria-selected', 'false')
  await user.click(sheet.getByRole('tab', { name: '품질 먼저' }))
  expect(sheet.getByRole('tab', { name: '품질 먼저' })).toHaveAttribute('aria-selected', 'true')
  expect(order()).toEqual(['Top', 'Balanced', 'Value One', 'Value Two', 'Free', 'Plain'])
})

// MODEL-24: a failed catalog read is a failure with a retry, never 고르지 않음 — that would read
// as the saved choice being gone.
it('shows a failed catalog read as a failure with a retry', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      calls,
      listFails: true,
      models: [{ providerId: 'openrouter', modelId: 'pen', label: 'Pen' }],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'pen' }],
    },
  })
  const main = within(await screen.findByRole('main'))
  expect(await main.findAllByText('쓰는 AI를 못 불러왔어요.')).toHaveLength(3)
  expect(main.queryByText('고르지 않음')).not.toBeInTheDocument()
  expect(main.queryByRole('button', { name: '바꾸기 · 글 쓰기' })).not.toBeInTheDocument()
  const reads = () => calls.filter((call) => call === 'ListModels').length
  const before = reads()
  await user.click(main.getByRole('button', { name: '다시 시도 · 글 쓰기' }))
  await waitFor(() => expect(reads()).toBeGreaterThan(before))
  expect(main.getAllByText('쓰는 AI를 못 불러왔어요.')).toHaveLength(3)
  expect(calls).not.toContain('SaveSelection')
})

// QUOTA-66, MODEL-27: the chosen model's row and its picker card never show a supplier price.
it('describes a chosen model without any price', async () => {
  const user = userEvent.setup()
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true }],
    },
  })
  const main = within(await screen.findByRole('main'))
  await waitFor(() =>
    expect(
      within(main.getByRole('button', { name: '바꾸기 · 사진 보기' }).parentElement!).getByText(
        'Vision',
      ),
    ).toBeInTheDocument(),
  )
  expect(main.queryByText(/\$|1M 토큰|가격 미확인/)).not.toBeInTheDocument()
  await user.click(main.getByRole('button', { name: '바꾸기 · 사진 보기' }))
  const sheet = await screen.findByRole('dialog', { name: '사진 보기 AI 고르기' })
  expect(await within(sheet).findByRole('radio', { name: /Vision/ })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  expect(within(sheet).queryByText(/\$|1M 토큰|가격 미확인/)).not.toBeInTheDocument()
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
  expect(await main.findByText('최근 사용량 기준 글 1개당 약 20크레딧')).toBeInTheDocument()
  // With no write model yet there is no post to price.
  expect(main.queryByText(/남은 크레딧으로/)).not.toBeInTheDocument()
  const sheet = await pick(user, '글 쓰기', /Pen/)
  // The picker card names what one post costs, in credits.
  expect(sheet.getByRole('radio', { name: /Pen/ })).toHaveTextContent('예상 글 1개당 약 10크레딧')
  await user.click(sheet.getByRole('button', { name: '완료' }))
  expect(await main.findByText('예상 글 1개당 약 10크레딧')).toBeInTheDocument()
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
  // The 글 쓰기 row and the page's estimate both name the per-post figure.
  expect(await main.findAllByText('최근 사용량 기준 글 1개당 약 10크레딧')).not.toHaveLength(0)
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
