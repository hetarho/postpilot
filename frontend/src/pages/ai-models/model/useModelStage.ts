import { useSearch } from '@tanstack/react-router'
import type { LeaderboardScopeName, LeaderboardWindowName } from '@/entities/model-experiment'

export function useModelStage() {
  const { stage } = useSearch({ from: '/authenticated/models' })
  return { stage: stage ?? 'observe' }
}

/** The leaderboard's own two filters, defaulting to 주간 · 나 (MODEL-44). They live in the
 *  URL beside the stage so a reload, the browser's back button and a shared link all open
 *  the same board. */
export function useLeaderboardFilters(): {
  window: LeaderboardWindowName
  scope: LeaderboardScopeName
} {
  const { window, scope } = useSearch({ from: '/authenticated/models' })
  return { window: window ?? 'week', scope: scope ?? 'me' }
}
