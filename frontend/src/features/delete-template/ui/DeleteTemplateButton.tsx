import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { detachWarning, type Template } from '@/entities/template'
import { Button, Dialog, FieldMessage } from '@/shared/ui'
import { useDeleteTemplate } from '../api/useDeleteTemplate'

/** Deletes a template after the sheet says exactly how many posts lose their assignment. The
 *  count comes from the server's projection, so the sentence and the write agree. */
export function DeleteTemplateButton({
  ownerId,
  template,
}: {
  ownerId: string
  template: Pick<Template, 'id' | 'name' | 'postCount'>
}) {
  const { t } = useTranslation(['templates', 'common'])
  const remove = useDeleteTemplate(ownerId)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.remove(template.id)
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
        aria-label={t('delete.aria', { ns: 'templates', name: template.name })}
      >
        {t('action.delete', { ns: 'common' })}
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title={t('delete.title', { ns: 'templates' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('delete.description', {
          ns: 'templates',
          name: template.name,
          detach: detachWarning(template.postCount),
        })}
      </Dialog>
    </>
  )
}
