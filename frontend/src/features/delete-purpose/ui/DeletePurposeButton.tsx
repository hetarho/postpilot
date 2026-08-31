import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { detachWarning, type Purpose } from '@/entities/purpose'
import { Button, Dialog, FieldMessage } from '@/shared/ui'
import { useDeletePurpose } from '../api/useDeletePurpose'

/** Deletes a purpose after the sheet says exactly how many posts lose their assignment. The
 *  count comes from the server's projection, so the sentence and the write agree. */
export function DeletePurposeButton({
  ownerId,
  purpose,
}: {
  ownerId: string
  purpose: Pick<Purpose, 'id' | 'name' | 'postCount'>
}) {
  const { t } = useTranslation(['purposes', 'common'])
  const remove = useDeletePurpose(ownerId)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.remove(purpose.id)
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
        aria-label={t('delete.aria', { ns: 'purposes', name: purpose.name })}
      >
        {t('action.delete', { ns: 'common' })}
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title={t('delete.title', { ns: 'purposes' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('delete.description', {
          ns: 'purposes',
          name: purpose.name,
          detach: detachWarning(purpose.postCount),
        })}
      </Dialog>
    </>
  )
}
