import { useTranslation } from 'react-i18next'
import { stageLabel } from '@/entities/model-catalog'
import { useLeaderboard } from '@/entities/model-experiment'
import { ModelLeaderboard } from '@/widgets/model-leaderboard'
import { Typography, pageStyles } from '@/shared/ui'
import { useModelStage } from '../model/useModelStage'
import { ModelPageHeader } from './ModelPageHeader'
import { ModelStageTabs } from './ModelStageTabs'
import { ModelResultsState } from './ModelResultsState'

export function ModelLeaderboardPage() {
  const { t } = useTranslation('models')
  const { stage } = useModelStage()
  const { entries, isPending, isError, refetch } = useLeaderboard(stage)
  return (
    <main className={pageStyles({ width: 'board', className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="leaderboardTitle" description="leaderboardDescription" />
      <ModelStageTabs to="/ai-models/leaderboard" />
      <section className="mt-6" aria-labelledby="leaderboard-heading">
        <Typography variant="title" id="leaderboard-heading">
          {t('page.myLeaderboard', { stage: stageLabel(stage) })}
        </Typography>
        <ModelResultsState isPending={isPending} isError={isError} onRetry={() => void refetch()}>
          <div className="mt-4">
            <ModelLeaderboard entries={entries} />
          </div>
        </ModelResultsState>
      </section>
    </main>
  )
}
