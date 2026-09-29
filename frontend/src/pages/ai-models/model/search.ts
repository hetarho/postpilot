import type {
  ExperimentStageName,
  LeaderboardScopeName,
  LeaderboardWindowName,
} from '@/entities/model-experiment'

/** The model group's URL filters. An unreadable value is dropped rather than corrected, so
 *  the page falls back to its own default and a shared link with a typo still opens — as does
 *  one naming analyze, a stage the lab no longer compares (MODEL-30). */
export const searchSchema = (
  search: Record<string, unknown>,
): {
  stage?: ExperimentStageName
  window?: LeaderboardWindowName
  scope?: LeaderboardScopeName
} => ({
  stage: search.stage === 'observe' || search.stage === 'write' ? search.stage : undefined,
  window:
    search.window === 'day' || search.window === 'week' || search.window === 'month'
      ? search.window
      : undefined,
  scope: search.scope === 'me' || search.scope === 'all' ? search.scope : undefined,
})
