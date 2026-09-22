import { useTranslation } from 'react-i18next'
import { stageLabel } from '@/entities/model-catalog'
import { useLeaderboard } from '@/entities/model-experiment'
import { ModelLeaderboard } from '@/widgets/model-leaderboard'
import { Typography, pageStyles } from '@/shared/ui'
import { useLeaderboardFilters, useModelStage } from '../model/useModelStage'
import { LeaderboardFilters } from './LeaderboardFilters'
import { ModelPageHeader } from './ModelPageHeader'
import { ModelStageTabs } from './ModelStageTabs'
import { ModelResultsState } from './ModelResultsState'

export function ModelLeaderboardPage() {
  const { t } = useTranslation('models')
  const { stage } = useModelStage()
  const { window, scope } = useLeaderboardFilters()
  const { entries, isPending, isError, refetch } = useLeaderboard(stage, window, scope)
  return (
    <main className={pageStyles({ width: 'board', className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="leaderboardTitle" description="leaderboardDescription" />
      <ModelStageTabs to="/ai-models/leaderboard" />
      <LeaderboardFilters window={window} scope={scope} />
      <section className="mt-6" aria-labelledby="leaderboard-heading">
        {/* The heading names the whole board, because the three controls above it are what
            select one and a rating read without its window means nothing. */}
        <Typography variant="title" id="leaderboard-heading">
          {t(scope === 'all' ? 'page.leaderboardAll' : 'page.leaderboardMe', {
            stage: stageLabel(stage),
          })}
        </Typography>
        <Typography variant="meta" as="p" className="mt-1">
          {t(`leaderboard.windowSpan.${window}`)}
        </Typography>
        <ModelResultsState isPending={isPending} isError={isError} onRetry={() => void refetch()}>
          <div className="mt-4">
            <ModelLeaderboard entries={entries} window={window} />
          </div>
        </ModelResultsState>
      </section>
    </main>
  )
}
