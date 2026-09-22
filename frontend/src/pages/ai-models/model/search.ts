import type { StageName } from '@/entities/model-catalog'
import type { LeaderboardScopeName, LeaderboardWindowName } from '@/entities/model-experiment'

/** The model group's URL filters. An unreadable value is dropped rather than corrected, so
 *  the page falls back to its own default and a shared link with a typo still opens. */
export const searchSchema = (
  search: Record<string, unknown>,
): { stage?: StageName; window?: LeaderboardWindowName; scope?: LeaderboardScopeName } => ({
  stage:
    search.stage === 'observe' || search.stage === 'analyze' || search.stage === 'write'
      ? search.stage
      : undefined,
  window:
    search.window === 'day' || search.window === 'week' || search.window === 'month'
      ? search.window
      : undefined,
  scope: search.scope === 'me' || search.scope === 'all' ? search.scope : undefined,
})
