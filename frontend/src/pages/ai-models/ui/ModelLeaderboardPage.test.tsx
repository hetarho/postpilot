import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { LeaderboardScope, LeaderboardWindow, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeExperimentsOptions } from '@/test/experiments'

function readsCollector() {
  const reads: NonNullable<FakeExperimentsOptions['reads']> = []
  return reads
}

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

  await user.click(screen.getByRole('tab', { name: '문체 분석' }))
  await waitFor(() => expect(router.state.location.search.stage).toBe('analyze'))
  expect(router.state.location.search.window).toBe('day')
  expect(router.state.location.search.scope).toBe('all')
  await waitFor(() =>
    expect(reads).toContainEqual({
      kind: 'leaderboard',
      stage: Stage.ANALYZE,
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
