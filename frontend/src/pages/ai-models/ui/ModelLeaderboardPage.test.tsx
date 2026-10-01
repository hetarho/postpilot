import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import {
  LeaderboardEntrySchema,
  LeaderboardScope,
  LeaderboardWindow,
  ProtoPlan,
  Stage,
} from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeExperimentsOptions } from '@/test/experiments'

function readsCollector() {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  return reads
}

it('shows a master the same Elo evidence without supplier cost', async () => {
  renderAppAt('/ai-models/leaderboard?scope=all', {
    user: { id: 'root', plan: ProtoPlan.MASTER },
    plans: { plan: ProtoPlan.MASTER },
    experiments: {
      leaderboardEntries: [
        create(LeaderboardEntrySchema, {
          rank: 1,
          model: { providerId: 'p', modelId: 'model' },
          modelLabel: 'Model',
          rating: 1516,
          matches: 1,
          wins: 1,
          evaluatedComparisons: 1,
          successfulCalls: 1,
          promptTokens: 10n,
          completionTokens: 20n,
        }),
      ],
    },
  })
  expect(await screen.findByText('Elo 1516')).toBeInTheDocument()
  expect(screen.getByText('1회 평가 · 상대별 1전 1승 0패 0무')).toBeInTheDocument()
  expect(screen.queryByText(/\$/)).not.toBeInTheDocument()
})

// A board is named by three controls at once, so the address has to carry all three. Opening
// the page with none of them stated asks for 주간 · 나 (MODEL-44).
it('defaults an unstated address to the weekly board of my own verdicts', async () => {
  const reads = readsCollector()
  renderAppAt('/ai-models/leaderboard', { user: { id: 'alice' }, experiments: { reads } })
  expect(await screen.findByRole('tab', { name: '주간' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('tab', { name: '나' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('heading', { name: '내 관찰 리더보드' })).toBeInTheDocument()
  expect(screen.getByText('최근 7일')).toBeInTheDocument()
  await waitFor(() =>
    expect(reads).toContainEqual({
      kind: 'leaderboard',
      stage: Stage.OBSERVE,
      window: LeaderboardWindow.WEEK,
      scope: LeaderboardScope.ME,
    }),
  )
})

// A value the address cannot mean is dropped rather than corrected, so a shared link with a
// typo still opens a readable board.
it('falls back to the default board when the address names a window or scope it cannot', async () => {
  const reads = readsCollector()
  renderAppAt('/ai-models/leaderboard?window=all-time&scope=friends', {
    user: { id: 'alice' },
    experiments: { reads },
  })
  expect(await screen.findByRole('tab', { name: '주간' })).toHaveAttribute('aria-selected', 'true')
  await waitFor(() =>
    expect(reads).toContainEqual(
      expect.objectContaining({
        kind: 'leaderboard',
        window: LeaderboardWindow.WEEK,
        scope: LeaderboardScope.ME,
      }),
    ),
  )
})

// The three filters are independent: changing one keeps the other two, and the browser's
// back button returns to the board that was being read.
it('keeps the other two filters when one changes, through reload and back', async () => {
  const user = userEvent.setup()
  const reads = readsCollector()
  const { router } = renderAppAt('/ai-models/leaderboard?stage=write', {
    user: { id: 'alice' },
    experiments: { reads },
  })
  await user.click(await screen.findByRole('tab', { name: '일간' }))
  await waitFor(() => expect(router.state.location.search.window).toBe('day'))
  expect(router.state.location.search.stage).toBe('write')

  await user.click(screen.getByRole('tab', { name: '전체' }))
  await waitFor(() => expect(router.state.location.search.scope).toBe('all'))
  expect(router.state.location.search.window).toBe('day')
  expect(router.state.location.search.stage).toBe('write')
  expect(screen.getByRole('heading', { name: '전체 글 작성 리더보드' })).toBeInTheDocument()

  await user.click(screen.getByRole('tab', { name: '관찰' }))
  await waitFor(() => expect(router.state.location.search.stage).toBe('observe'))
  expect(router.state.location.search.window).toBe('day')
  expect(router.state.location.search.scope).toBe('all')
  await waitFor(() =>
    expect(reads).toContainEqual({
      kind: 'leaderboard',
      stage: Stage.OBSERVE,
      window: LeaderboardWindow.DAY,
      scope: LeaderboardScope.ALL,
    }),
  )

  await act(async () => router.history.back())
  await waitFor(() => expect(router.state.location.search.stage).toBe('write'))
  expect(router.state.location.search.window).toBe('day')
  expect(router.state.location.search.scope).toBe('all')
})

// Every window the switch offers is bounded; none of them asks for the whole history.
it('offers three bounded periods and no all-time board', async () => {
  renderAppAt('/ai-models/leaderboard', { user: { id: 'alice' } })
  const periods = await screen.findAllByRole('tab', { name: /^(일간|주간|월간)$/ })
  expect(periods).toHaveLength(3)
  expect(screen.queryByRole('tab', { name: '전체 기간' })).not.toBeInTheDocument()
})

// MODEL-44: the boards are observe and write. Analyze is never compared, so it has no board,
// and an address that still names it opens the observe board instead of asking for one.
it('ranks observe and write only, reading an address naming analyze as observe', async () => {
  const reads = readsCollector()
  renderAppAt('/ai-models/leaderboard?stage=analyze&window=day', {
    user: { id: 'alice' },
    experiments: { reads },
  })
  const stages = within(await screen.findByRole('tablist', { name: 'AI 단계' }))
  expect(stages.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['관찰', '글 작성'])
  expect(stages.getByRole('tab', { name: '관찰' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('tab', { name: '일간' })).toHaveAttribute('aria-selected', 'true')
  await waitFor(() =>
    expect(reads).toContainEqual({
      kind: 'leaderboard',
      stage: Stage.OBSERVE,
      window: LeaderboardWindow.DAY,
      scope: LeaderboardScope.ME,
    }),
  )
  expect(reads.map((read) => read.stage)).not.toContain(Stage.ANALYZE)
})
