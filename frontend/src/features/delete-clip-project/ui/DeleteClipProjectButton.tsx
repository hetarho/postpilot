import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { Trash2 } from 'lucide-react'
import {
  clipReturnDestination,
  markClipHistoryReturn,
  forgetClipEntry,
  type ClipProject,
  useClipProjectMutations,
} from '@/entities/clip-project'
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
  onReturn,
}: {
  ownerId: string
  project: Pick<ClipProject, 'id'>
  disabled?: boolean
  /** Run after the server confirms the delete and BEFORE the navigation unmounts the page. The
   *  settings autosave queue is what has to be stopped here, and it belongs to a sibling feature
   *  slice this one may not import (ARCH-13), so the page supplies the call. */
  onDeleted?: () => void
  /** Contextual navigation after confirmed deletion and queue cleanup. Leaving never cancels work. */
  onReturn?: () => void | Promise<void>
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const { remove } = useClipProjectMutations(ownerId)
  const [confirming, setConfirming] = useState(false)
  const authority = useRef(0)
  useEffect(() => {
    authority.current += 1
    return () => {
      authority.current += 1
    }
  }, [ownerId, project.id])

  const confirm = async () => {
    const operation = authority.current
    try {
      await remove.mutateAsync(project.id)
    } catch {
      // A refusal renders beside the trigger, and the dialog closes on failure too so the message
      // is not left behind the scrim. Only the DELETE is caught: a navigation that fails after it
      // must not be reported as a refusal, because by then the project really is gone.
      if (operation === authority.current) setConfirming(false)
      return
    }
    onDeleted?.()
    const destination = clipReturnDestination(ownerId, project.id)
    markClipHistoryReturn(ownerId, project.id)
    forgetClipEntry(ownerId, project.id)
    if (operation !== authority.current) return
    setConfirming(false)
    if (onReturn) await onReturn()
    else {
      await navigate({ href: destination.href, replace: true })
    }
  }

  return (
    <>
      {/* The glyph carries it on a phone, where the row also holds the step bar (CLIP-37); the
          word comes back from `sm:`. The name is the word at every width. */}
      <Button
        variant="danger"
        aria-label={t('project.delete')}
        disabled={disabled || remove.isPending}
        onClick={() => setConfirming(true)}
      >
        <Trash2 aria-hidden="true" className="size-5" />
        <span className="hidden sm:inline">{t('project.delete')}</span>
      </Button>
      {/* `w-full` inside the wrapping top row, so a refusal takes its own line under the trigger
          instead of squeezing the row it was pressed from (THEME-24). */}
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
