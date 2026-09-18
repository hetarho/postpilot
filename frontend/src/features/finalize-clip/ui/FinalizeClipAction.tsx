import { useTranslation } from 'react-i18next'
import type { ClipProject } from '@/entities/clip-project'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import type { useFinalizeClip } from '../model/useFinalizeClip'

/** Why ② would refuse the confirmation, or nothing when it would take it. */
function refusalOf(project: ClipProject) {
  return project.finalizationRefusal ?? (!project.canFinalize ? 'unavailable' : undefined)
}

/** Everything the confirmation has to SAY: what confirming does, what the clip
 *  delivered and why it is refused. It lives in ②'s PANEL, directly above the
 *  dock, because a dock holds the committing control and its refusals and
 *  nothing that is merely true (THEME-39, THEME-34) — stacked inside the dock,
 *  this block grew tall enough to cover the editor it sits over. */
export function FinalizeClipNotices({
  action,
  project,
}: {
  action: ReturnType<typeof useFinalizeClip>
  project: ClipProject
}) {
  const { t } = useTranslation('clips')
  const refusal = refusalOf(project)
  return (
    <div className="w-full min-w-0 space-y-2">
      <Typography variant="meta">{t('finalization.notice')}</Typography>
      {/* No notice list: a notice naming a cut or a caption rides in that item's
          sheet and one naming neither in the info control beside the preview, so
          a third copy of the whole set standing in ②'s page is exactly what
          CLIP-109 took away. T248 lists the unresolved ones in the finalization
          dialog this copy moves into. */}
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
    </div>
  )
}

/** ②'s primary control shares the dock's upper row with rendering (CLIP-40). */
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
  const refusal = refusalOf(project)
  return (
    <Button
      variant="cta"
      pending={action.pending}
      disabled={disabled || !!refusal || action.uncertain}
      onClick={() => void action.confirm()}
    >
      {t('finalization.confirm')}
    </Button>
  )
}
