import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ExperimentSource, ProtoPlan, Stage } from '@/shared/api'
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
    const user = userEvent.setup()
    const calls: string[] = []
    const starts: string[] = []
    const reads: NonNullable<FakeExperimentsOptions['reads']> = []
    const { router } = renderAppAt('/ai-models', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      providers: { calls },
      experiments: { calls: starts, reads },
    })
    const group = locale === 'ko' ? 'AI 모델 메뉴' : 'AI model navigation'
    await screen.findByRole('navigation', { name: group })
    // The group's destinations live inside the one sidebar, under the primary row that opens
    // them, and say which level they are because a group's home repeats a primary address.
    const nav = within(document.querySelector('aside')!)
    expect(
      nav
        .getAllByRole('link')
        .filter((link) => link.dataset.navLevel === 'group')
        .map((link) => link.textContent),
    ).toEqual(destinations.map((d) => d[locale === 'ko' ? 1 : 2]))
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
      await user.click(nav.getByRole('link', { name: locale === 'ko' ? ko : en }))
      await waitFor(() => expect(router.state.location.pathname).toBe(path))
      const main = within(screen.getByRole('main'))
      expect(
        main.getByRole('heading', { level: 1, name: locale === 'ko' ? ko : en }),
      ).toBeInTheDocument()
      // The group level alone: the primary row above it is current for the whole group, and on
      // /ai-models it carries the same address as the group's home.
      const active = nav
        .getAllByRole('link')
        .filter(
          (link) =>
            link.dataset.navLevel === 'group' && link.getAttribute('aria-current') === 'page',
        )
      expect(active.map((link) => link.getAttribute('href'))).toEqual([path])
      expect(
        main.queryByRole('button', { name: locale === 'ko' ? '비교 시작' : 'Start comparison' }) !==
          null,
      ).toBe(path === '/ai-models/compare')
      expect(
        main.queryByRole('heading', { name: locale === 'ko' ? '추천 조합' : 'Recommended set' }) !==
          null,
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

it('preserves the stage through the group menu, browser history, and saved experiment links', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/ai-models/experiments?stage=write', {
    user: { id: 'alice' },
    experiments: {
      history: [{ id: 'writing-1', stage: Stage.WRITE, postSlug: 'first-post' }],
    },
  })
  const record = await screen.findByRole('link', { name: /first-post/ })
  expect(record).toHaveAttribute('href', '/ai-models/experiments/writing-1?stage=write')
  const [band] = screen.getAllByRole('navigation', { name: 'AI 모델 메뉴' })
  await user.click(within(band!).getByRole('button'))
  await user.click(screen.getByRole('menuitemradio', { name: '리더보드' }))
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
