import { useTranslation } from 'react-i18next'
import type { ClipProject } from '@/entities/clip-project'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import type { useFinalizeClip } from '../model/useFinalizeClip'

export function FinalizeClipAction({
  action,
  project,
  disabled,
}: {
  action: ReturnType<typeof useFinalizeClip>
  project: ClipProject
  disabled: boolean
}) {
  const { t } = useTranslation('clips')
  const refusal = project.finalizationRefusal ?? (!project.canFinalize ? 'unavailable' : undefined)
  return (
    <div className="w-full min-w-0 space-y-2">
      <Typography variant="meta">{t('finalization.notice')}</Typography>
      {refusal && <Typography variant="body">{t(`finalization.refusal.${refusal}`)}</Typography>}
      {action.failure && (
        <div role="alert">
          <AppFailureMessage failure={action.failure} />
        </div>
      )}
      {action.uncertain && (
        <>
          <Typography variant="body">{t('finalization.uncertain')}</Typography>
          <Button
            variant="secondary"
            pending={action.pending}
            onClick={() => void action.checkAgain()}
          >
            {t('project.retry')}
          </Button>
        </>
      )}
      <Button
        variant="cta"
        className="w-full sm:w-auto"
        pending={action.pending}
        disabled={disabled || !!refusal || action.uncertain}
        onClick={() => void action.confirm()}
      >
        {t('finalization.confirm')}
      </Button>
    </div>
  )
}
