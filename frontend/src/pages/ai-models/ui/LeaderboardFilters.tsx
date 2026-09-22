import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import type { LeaderboardScopeName, LeaderboardWindowName } from '@/entities/model-experiment'
import { SegmentedControl } from '@/shared/ui'

/** The two switches that, with the stage tabs above them, name one board: how far back it
 *  reads and whose verdicts it replays (MODEL-38). Both live in the URL, so a reload, the
 *  browser's back button and a shared link all open the board that was being read.
 *
 *  Stacked below `sm:`, because 일간 · 주간 · 월간 and 나 · 전체 side by side leave each
 *  option under the pointer floor at 360px. */
export function LeaderboardFilters({
  window,
  scope,
}: {
  window: LeaderboardWindowName
  scope: LeaderboardScopeName
}) {
  const { t } = useTranslation('models')
  const navigate = useNavigate()
  return (
    <div className="mt-3 grid gap-2 sm:flex sm:items-center sm:gap-3">
      <SegmentedControl
        value={window}
        options={(['day', 'week', 'month'] as const).map((value) => ({
          value,
          label: t(`leaderboard.window.${value}`),
        }))}
        onChange={(window) =>
          void navigate({
            to: '/ai-models/leaderboard',
            search: (previous) => ({ ...previous, window }),
          })
        }
        ariaLabel={t('leaderboard.windowAria')}
      />
      <SegmentedControl
        value={scope}
        options={(['me', 'all'] as const).map((value) => ({
          value,
          label: t(`leaderboard.scope.${value}`),
        }))}
        onChange={(scope) =>
          void navigate({
            to: '/ai-models/leaderboard',
            search: (previous) => ({ ...previous, scope }),
          })
        }
        ariaLabel={t('leaderboard.scopeAria')}
      />
    </div>
  )
}
