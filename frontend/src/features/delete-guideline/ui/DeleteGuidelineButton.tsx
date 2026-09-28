import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { useDeleteGuidelineCall, type GuidelineKind } from '@/entities/guideline'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

/** Deletes a guideline after the sheet states what is and is not affected: work already enqueued
 *  keeps its frozen text, and nothing else changes. There is no count to name — nothing references
 *  a guideline. An icon beside 수정 on an open row (GUIDE-47); its failure message takes a line of
 *  its own below (`w-full` in the row's wrapping flex line). */
export function DeleteGuidelineButton({
  ownerId,
  kind = 'post',
  guidelineId,
}: {
  ownerId: string
  /** The guideline kind the action is for; a post's by default. */
  kind?: GuidelineKind
  guidelineId: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const remove = useDeleteGuidelineCall(ownerId, kind)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.remove(guidelineId)
    } catch {
      // The mutation's message renders beside the button.
    } finally {
      // Closed on failure too, so the message is not left behind the scrim.
      setConfirming(false)
    }
  }

  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        disabled={remove.isPending}
        onClick={() => setConfirming(true)}
        aria-label={t('delete.aria', { ns: 'guidelines' })}
        className="shrink-0"
      >
        <Trash2 className="size-4" aria-hidden />
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title={t('delete.title', { ns: 'guidelines' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('delete.description', { ns: 'guidelines' })}
      </Dialog>
    </>
  )
}
