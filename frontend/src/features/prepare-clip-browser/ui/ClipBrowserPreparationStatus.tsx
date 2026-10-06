import { useTranslation } from 'react-i18next'
import { Button, ProgressBar, Typography } from '@/shared/ui'
import type { AnalysisPreparationProgress } from '../model/types'

export function ClipBrowserPreparationStatus({
  progress,
  busy,
  cancel,
  refusal,
}: {
  progress?: AnalysisPreparationProgress
  busy: boolean
  cancel(): void
  refusal?: 'codec' | 'memory' | 'color'
}) {
  const { t } = useTranslation('clips')
  if (!busy && refusal)
    return (
      <Typography variant="body" role="alert">
        {t(`analysisPreparation.refusal.${refusal}`)}
      </Typography>
    )
  if (!busy || !progress) return null
  const label = t(`analysisPreparation.${progress.stage}`)
  return (
    <div className="space-y-3">
      <Typography variant="body" role="status">
        {label}
      </Typography>
      <ProgressBar
        label={label}
        done={progress.done + (progress.fraction ?? 0)}
        total={progress.total}
      />
      <Button variant="ghost" onClick={cancel}>
        {t('analysisPreparation.cancel')}
      </Button>
    </div>
  )
}
