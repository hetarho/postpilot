import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { useClipProjectMutations, type ClipProject } from '@/entities/clip-project'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Dialog } from '@/shared/ui'

/** Deletes the clip project the workspace is showing. The server delete is hard — the retained
 *  result, analysis, edit plan and metadata all go (CLIP-24) — so the confirmation is the whole
 *  protection and has to say what it destroys.
 *
 *  It rides the workspace's top row beside the status line (CLIP-37), which is why it is a slice
 *  of its own rather than a control inside the settings form: a feature cannot be reached from
 *  inside another feature's markup (ARCH-13), and the row is the page's. */
export function DeleteClipProjectButton({
  ownerId,
  project,
  disabled = false,
  onDeleted,
}: {
  ownerId: string
  project: Pick<ClipProject, 'id'>
  disabled?: boolean
  /** Run after the server confirms the delete and BEFORE the navigation unmounts the page. The
   *  settings autosave queue is what has to be stopped here, and it belongs to a sibling feature
   *  slice this one may not import (ARCH-13), so the page supplies the call. */
  onDeleted?: () => void
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const { remove } = useClipProjectMutations(ownerId)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.mutateAsync(project.id)
    } catch {
      // A refusal renders beside the trigger, and the dialog closes on failure too so the message
      // is not left behind the scrim. Only the DELETE is caught: a navigation that fails after it
      // must not be reported as a refusal, because by then the project really is gone.
      setConfirming(false)
      return
    }
    setConfirming(false)
    onDeleted?.()
    await navigate({ to: '/clips', replace: true })
  }

  return (
    <>
      <Button
        variant="danger"
        disabled={disabled || remove.isPending}
        onClick={() => setConfirming(true)}
      >
        {t('project.delete')}
      </Button>
      {/* `w-full` inside the wrapping top row, so a refusal takes its own line under the trigger
          instead of squeezing the row it was pressed from (§4.3). */}
      {remove.isError && (
        <div role="alert" className="w-full">
          <AppFailureMessage failure={appFailureFromConnect(remove.error)} />
        </div>
      )}
      <Dialog
        open={confirming}
        title={t('project.deleteTitle')}
        confirmLabel={t('project.delete')}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('project.deleteBody')}
      </Dialog>
    </>
  )
}
