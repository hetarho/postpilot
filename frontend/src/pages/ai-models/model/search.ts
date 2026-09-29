import type {
  LeaderboardScopeName,
  LeaderboardWindowName,
  ModelLabTabName,
} from '@/entities/model-experiment'

/** The model group's URL filters. An unreadable value is dropped rather than corrected, so
 *  the page falls back to its own default and a shared link with a typo still opens — as does
 *  one naming analyze, a stage the lab no longer compares (MODEL-30). `voice` is the 말투 반영 tab of
 *  the compare and history pages (MODEL-67). */
export const searchSchema = (
  search: Record<string, unknown>,
): {
  stage?: ModelLabTabName
  window?: LeaderboardWindowName
  scope?: LeaderboardScopeName
} => ({
  stage:
    search.stage === 'observe' || search.stage === 'write' || search.stage === 'voice'
      ? search.stage
      : undefined,
  window:
    search.window === 'day' || search.window === 'week' || search.window === 'month'
      ? search.window
      : undefined,
  scope: search.scope === 'me' || search.scope === 'all' ? search.scope : undefined,
})
