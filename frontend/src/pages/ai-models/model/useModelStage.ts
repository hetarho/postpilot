import { useSearch } from '@tanstack/react-router'
import type {
  ExperimentStageName,
  LeaderboardScopeName,
  LeaderboardWindowName,
  ModelLabTabName,
} from '@/entities/model-experiment'

/** The lab tab in the URL. `voice` is 말투 반영, a write comparison drawn from a voice; a page
 *  that has no such tab — the leaderboard — reads it as 글쓰기, whose board its verdicts join
 *  (MODEL-67). */
export function useModelStage() {
  const { stage } = useSearch({ from: '/authenticated/models' })
  return { stage: stage ?? 'observe' }
}

/** The stage a lab tab compares. */
export function stageOfTab(tab: ModelLabTabName): ExperimentStageName {
  return tab === 'voice' ? 'write' : tab
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
