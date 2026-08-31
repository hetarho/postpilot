import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useDeleteGuidelineCall } from '@/entities/guideline'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

/** Deletes a guideline after the sheet states what is and is not affected: work already enqueued
 *  keeps its frozen text, and nothing else changes. There is no count to name — nothing references
 *  a guideline. */
export function DeleteGuidelineButton({
  ownerId,
  guidelineId,
}: {
  ownerId: string
  guidelineId: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const remove = useDeleteGuidelineCall(ownerId)
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
        variant="danger"
        disabled={remove.isPending}
        onClick={() => setConfirming(true)}
        aria-label={t('delete.aria', { ns: 'guidelines' })}
      >
        {t('action.delete', { ns: 'common' })}
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
