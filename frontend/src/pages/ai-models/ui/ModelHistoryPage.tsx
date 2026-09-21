import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { stageLabel } from '@/entities/model-catalog'
import { useExperiments, type ExperimentStatusName } from '@/entities/model-experiment'
import { useSession } from '@/entities/session'
import { useVoices, voiceRefLabel } from '@/entities/voice'
import { Badge, type BadgeTone, Typography, typographyStyles, pageStyles } from '@/shared/ui'
import { useModelStage } from '../model/useModelStage'
import { ModelPageHeader } from './ModelPageHeader'
import { ModelStageTabs } from './ModelStageTabs'
import { ModelResultsState } from './ModelResultsState'

export function ModelHistoryPage() {
  const { t } = useTranslation('models')
  const { stage } = useModelStage()
  const { experiments, isPending, isError, refetch } = useExperiments(stage)
  const { user } = useSession()
  const { voices } = useVoices(user?.id ?? '')
  const voiceName = (id: string) => {
    const voice = voices.find((candidate) => candidate.id === id)
    return voice ? voiceRefLabel(voice) : id
  }
  return (
    <main className={pageStyles({ width: 'wide', className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="history" description="historyDescription" />
      <ModelStageTabs to="/ai-models/experiments" />
      <section className="mt-6" aria-labelledby="recent-heading">
        <Typography variant="title" id="recent-heading">
          {t('page.recent', { stage: stageLabel(stage) })}
        </Typography>
        <ModelResultsState isPending={isPending} isError={isError} onRetry={() => void refetch()}>
          {experiments.length === 0 ? (
            <Typography variant="body" className="text-content-tertiary mt-4">
              {t('page.noComparison')}
            </Typography>
          ) : (
            <ul className="divide-divider -mx-4 mt-4 divide-y sm:-mx-6 lg:-mx-8">
              {experiments.slice(0, 8).map((item) => (
                <li key={item.id}>
                  <Link
                    to="/ai-models/experiments/$id"
                    params={{ id: item.id }}
                    search={{ stage }}
                    className={typographyStyles({
                      variant: 'label',
                      className:
                        'text-content-primary hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center justify-between gap-3 px-4 py-3 sm:px-6 lg:px-8',
                    })}
                  >
                    {/* `min-w-0` is what makes `truncate` work: a slug is `YYYYMMDD-` plus up to 60
                      runes of the title, so a spaceless Korean one is ~420px of max-content in a
                      312px row and would otherwise crush the status chip to a column of single
                      syllables (§8.5). */}
                    <span className="min-w-0 truncate">
                      {item.postSlug || voiceName(item.voiceId) || stageLabel(item.stage)}
                    </span>
                    <Badge tone={STATUS_TONES[item.status]}>
                      {t(`experimentStatus.${item.status}`, { ns: 'models' })}
                    </Badge>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </ModelResultsState>
      </section>
    </main>
  )
}

/** The row's status chip. The tone reinforces the label and never replaces it, so nothing is
 *  carried by colour alone (§2.6). */
const STATUS_TONES: Record<ExperimentStatusName, BadgeTone> = {
  queued: 'neutral',
  running: 'info',
  review: 'info',
  partial: 'warning',
  failed: 'danger',
  decided: 'success',
  dismissed: 'neutral',
}
