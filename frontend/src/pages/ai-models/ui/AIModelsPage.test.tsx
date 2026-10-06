import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ExperimentSource, PostCreditsBasis, ProtoPlan, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeExperimentsOptions } from '@/test/experiments'
import { chooseOption } from '@/test/listbox'

afterEach(() => initializeI18n('ko'))

const destinations = [
  ['/ai-models', '모델 변경', 'Change models'],
  ['/ai-models/compare', '모델 비교', 'Compare models'],
  ['/ai-models/experiments', '최근 관찰 비교', 'Recent observation comparisons'],
  ['/ai-models/leaderboard', '리더보드', 'Leaderboard'],
] as const

it.each(['ko', 'en'] as const)(
  'separates four destinations without starting work in %s',
  async (locale) => {
    initializeI18n(locale)
    const calls: string[] = []
    const starts: string[] = []
    const reads: NonNullable<FakeExperimentsOptions['reads']> = []
    const { router } = renderAppAt('/ai-models', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      providers: { calls },
      experiments: { calls: starts, reads },
    })
    await screen.findByRole('main')
    expect(document.querySelector('aside')).toBeNull()
    // 모델 변경 keeps all three active selections: analyze is never compared, but 말투 만들기
    // and memory extraction still read its one active model (MODEL-23).
    expect(within(screen.getByRole('main')).getAllByRole('combobox')).toHaveLength(3)
    expect(
      within(screen.getByRole('main')).getByRole('combobox', {
        name: locale === 'ko' ? /문체 분석 모델/ : /Analyze voice model/,
      }),
    ).toBeInTheDocument()
    expect(reads).toEqual([])
    for (const [path, ko, en] of destinations) {
      await act(async () => router.navigate({ to: path }))
      await waitFor(() => expect(router.state.location.pathname).toBe(path))
      const main = within(screen.getByRole('main'))
      expect(
        main.getByRole('heading', { level: 1, name: locale === 'ko' ? ko : en }),
      ).toBeInTheDocument()
      expect(
        main.queryByRole('button', { name: locale === 'ko' ? '비교 시작' : 'Start comparison' }) !==
          null,
      ).toBe(path === '/ai-models/compare')
      expect(
        main.queryByRole('heading', {
          name: locale === 'ko' ? '추천 조합' : 'Recommended sets',
        }) !== null,
      ).toBe(path === '/ai-models')
    }
    expect(calls.filter((call) => /Save|Apply/.test(call))).toEqual([])
    expect(starts).toEqual([])
  },
)

it('saves an active model only after a model change, separately from comparison candidates', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      calls,
      models: [{ providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true }],
    },
  })
  const main = within(await screen.findByRole('main'))
  await chooseOption(user, main.getByRole('combobox', { name: /관찰/ }), 'Vision')
  await waitFor(() => expect(calls).toContain('SaveSelection'))
  expect(calls).not.toContain('SaveComparisonPair')
  expect(main.queryByRole('button', { name: '비교 시작' })).not.toBeInTheDocument()
})

// QUOTA-66, MODEL-27: the selected model's line says what it can hold, never a supplier price.
it('describes a chosen model without any price', async () => {
  const user = userEvent.setup()
  renderAppAt('/ai-models', {
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true }],
    },
  })
  const main = within(await screen.findByRole('main'))
  await chooseOption(user, main.getByRole('combobox', { name: /관찰/ }), 'Vision')
  expect(await main.findByText('컨텍스트 0')).toBeInTheDocument()
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

it('preserves the explicit stage through navigation, browser history, and saved experiment links', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/ai-models/experiments?stage=write', {
    user: { id: 'alice' },
    experiments: {
      history: [{ id: 'writing-1', stage: Stage.WRITE, postSlug: 'first-post' }],
    },
  })
  const record = await screen.findByRole('link', { name: /first-post/ })
  expect(record).toHaveAttribute('href', '/ai-models/experiments/writing-1?stage=write')
  await act(async () =>
    router.navigate({ to: '/ai-models/leaderboard', search: { stage: 'write' } }),
  )
  await waitFor(() => expect(router.state.location.pathname).toBe('/ai-models/leaderboard'))
  expect(screen.getByRole('tab', { name: '글 작성' })).toHaveAttribute('aria-selected', 'true')
  await user.click(screen.getByRole('tab', { name: '관찰' }))
  await waitFor(() => expect(router.state.location.search.stage).toBe('observe'))
  await act(async () => router.history.back())
  await waitFor(() => expect(router.state.location.search.stage).toBe('write'))
  await act(async () => router.history.back())
  await waitFor(() => expect(router.state.location.pathname).toBe('/ai-models/experiments'))
  await user.click(await screen.findByRole('link', { name: /first-post/ }))
  const back = await screen.findByRole('link', { name: '← 비교 기록' })
  expect(back).toHaveAttribute('href', '/ai-models/experiments?stage=write')
  await user.click(back)
  expect(await screen.findByRole('link', { name: /first-post/ })).toBeInTheDocument()
})

// An address naming analyze — a stage the lab no longer compares (MODEL-30) — is read as a
// typo would be: the history and the board open on observe and never ask for analyze.
it.each([
  ['/ai-models/experiments?stage=invalid', 'history'],
  ['/ai-models/leaderboard?stage=invalid', 'leaderboard'],
  ['/ai-models/experiments?stage=analyze', 'history'],
  ['/ai-models/leaderboard?stage=analyze', 'leaderboard'],
] as const)('defaults invalid stages to observe on direct load at %s', async (path, kind) => {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  renderAppAt(path, { user: { id: 'alice' }, experiments: { reads } })
  expect(await screen.findByRole('tab', { name: '관찰' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getAllByRole('tab', { name: /^(관찰|글 작성|문체 분석)$/ })).toHaveLength(2)
  await waitFor(() =>
    expect(reads).toContainEqual(expect.objectContaining({ kind, stage: Stage.OBSERVE })),
  )
  expect(reads.map((read) => read.stage)).not.toContain(Stage.ANALYZE)
})

it.each([
  ['/ai-models/experiments', 'listFails', '아직 비교가 없어요.'],
  ['/ai-models/leaderboard', 'leaderboardFails', '최근 7일 안에는 비교 결과가 없어요.'],
] as const)(
  'does not turn loading or failed reads into an empty history at %s',
  async (path, failureKey, empty) => {
    let release!: () => void
    const readGate = new Promise<void>((resolve) => {
      release = resolve
    })
    renderAppAt(path, { user: { id: 'alice' }, experiments: { readGate, [failureKey]: true } })
    expect(await screen.findByRole('status')).toHaveTextContent('불러오는 중')
    expect(screen.queryByText(empty)).not.toBeInTheDocument()
    await act(async () => release())
    expect(await screen.findByRole('alert')).toHaveTextContent('비교 정보를 불러오지 못했어요.')
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
    expect(screen.queryByText(empty)).not.toBeInTheDocument()
  },
)

it.each(destinations)('guards direct access to %s', async (path) => {
  const { router } = renderAppAt(path)
  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  expect(router.state.location.search.redirect).toBe(path)
})

// MODEL-44, MODEL-67: the history's 말투 반영 tab lists voice-sourced write comparisons naming the
// voice and the prompt; 글쓰기 lists post-sourced ones only.
it('lists voice-sourced comparisons on 말투 반영 and post-sourced ones on 글쓰기', async () => {
  const user = userEvent.setup()
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  renderAppAt('/ai-models/experiments?stage=voice', {
    user: { id: 'alice' },
    voice: { voices: [{ id: 'voice-default', name: '기본 말투', isDefault: true }] },
    experiments: {
      reads,
      history: [
        { id: 'writing-1', stage: Stage.WRITE, postSlug: 'first-post' },
        {
          id: 'reflection-1',
          stage: Stage.WRITE,
          voiceId: 'voice-default',
          source: ExperimentSource.VOICE,
          voicePromptText: '첫인사를 써 보세요.',
        },
      ],
    },
  })

  const record = await screen.findByRole('link', { name: /기본 말투 · 첫인사를 써 보세요\./ })
  expect(record).toHaveAttribute('href', '/ai-models/experiments/reflection-1?stage=voice')
  expect(screen.queryByRole('link', { name: /first-post/ })).not.toBeInTheDocument()
  expect(screen.getByRole('tab', { name: '말투 반영' })).toHaveAttribute('aria-selected', 'true')
  expect(reads).toContainEqual(
    expect.objectContaining({
      kind: 'history',
      stage: Stage.WRITE,
      source: ExperimentSource.VOICE,
    }),
  )

  await user.click(screen.getByRole('tab', { name: '글 작성' }))
  expect(await screen.findByRole('link', { name: /first-post/ })).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: /첫인사를 써 보세요/ })).not.toBeInTheDocument()
  expect(reads).toContainEqual(
    expect.objectContaining({ kind: 'history', stage: Stage.WRITE, source: ExperimentSource.POST }),
  )
})

// MODEL-67: a 말투 반영 verdict counts on the write board, and the board keeps its two stages.
it('reads 말투 반영 as the write board on the leaderboard', async () => {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  renderAppAt('/ai-models/leaderboard?stage=voice', {
    user: { id: 'alice' },
    experiments: { reads },
  })

  expect(await screen.findByRole('tab', { name: '글 작성' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  expect(screen.queryByRole('tab', { name: '말투 반영' })).not.toBeInTheDocument()
  await waitFor(() =>
    expect(reads).toContainEqual(
      expect.objectContaining({ kind: 'leaderboard', stage: Stage.WRITE }),
    ),
  )
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
