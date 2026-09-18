import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipNoticeList, type ClipProject } from '@/entities/clip-project'
import { AppFailureMessage, Button, Dialog, Typography } from '@/shared/ui'
import type { useFinalizeClip } from '../model/useFinalizeClip'
import { finalizationRefusal } from '../model/finalization-refusal'

/** The consequences and unresolved notices belong to the confirmation itself. */
export function FinalizeClipNotices({ project }: { project: ClipProject }) {
  const { t } = useTranslation('clips')
  return (
    <div className="min-w-0 space-y-3">
      <Typography variant="body">{t('finalization.notice')}</Typography>
      <ClipNoticeList
        notices={project.notices}
        language={project.language}
        cuts={project.editing?.plan.cuts}
        withTargets
      />
    </div>
  )
}

/** Flush before opening; only the dialog can invoke the finalizing action. */
export function FinalizeClipAction({
  action,
  project,
  disabled,
  localRefusal,
}: {
  action: ReturnType<typeof useFinalizeClip>
  project: ClipProject
  disabled: boolean
  localRefusal?: ClipProject['finalizationRefusal']
}) {
  const { t } = useTranslation('clips')
  const descriptionId = useId()
  const [prepared, setPrepared] = useState<ClipProject>()
  const refusal = localRefusal ?? finalizationRefusal(project) ?? (disabled ? 'busy' : undefined)
  const open =
    !!prepared &&
    !refusal &&
    prepared.editPlanRevision === project.editPlanRevision &&
    prepared.result?.id === project.result?.id
  return (
    <>
      <Button
        variant="cta"
        pending={action.pending}
        disabled={disabled || !!refusal || action.uncertain}
        aria-describedby={refusal ? descriptionId : undefined}
        onClick={() => {
          void action.prepare().then((saved) => {
            if (saved && !finalizationRefusal(saved)) setPrepared(saved)
          })
        }}
      >
        {t('finalization.confirm')}
      </Button>
      {(refusal || action.failure || action.uncertain) && (
        <div className="w-full min-w-0 space-y-2">
          {refusal && (
            <Typography variant="meta" as="p" id={descriptionId}>
              {t(`finalization.refusal.${refusal}`)}
            </Typography>
          )}
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
        </div>
      )}
      <Dialog
        open={open}
        title={t('finalization.dialogTitle')}
        confirmLabel={t('finalization.confirm')}
        pending={action.pending}
        onClose={() => setPrepared(undefined)}
        onConfirm={() => {
          void action.confirm().finally(() => setPrepared(undefined))
        }}
      >
        {prepared && <FinalizeClipNotices project={prepared} />}
      </Dialog>
    </>
  )
}
