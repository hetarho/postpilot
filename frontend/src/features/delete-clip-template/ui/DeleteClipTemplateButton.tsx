import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useClipTemplateMutations, type ClipTemplate } from '@/entities/clip-template'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, FieldMessage } from '@/shared/ui'

/** Deletes a video template after the sheet says exactly how many clip projects lose their
 *  recipe. The count comes from the server's own projection, so the sentence and the write agree.
 *
 *  It rides the directory ROW, where the post-template directory puts its own (CLIP-42), which
 *  also leaves the editor's dock holding exactly one action. The detach is reported BEFORE the
 *  delete rather than as a message on the screen it lands on: a warning the owner reads after the
 *  fact is not a warning. */
export function DeleteClipTemplateButton({
  ownerId,
  template,
}: {
  ownerId: string
  template: Pick<ClipTemplate, 'id' | 'name' | 'projectCount'>
}) {
  const { t } = useTranslation(['clips', 'common'])
  const { remove } = useClipTemplateMutations(ownerId)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.mutateAsync(template.id)
    } catch {
      // The refusal renders beside the button.
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
        aria-label={t('delete.aria', { ns: 'clips', name: template.name })}
      >
        {t('action.delete', { ns: 'common' })}
      </Button>
      {remove.isError && (
        <FieldMessage className="w-full">
          <AppFailureMessage failure={appFailureFromConnect(remove.error)} />
        </FieldMessage>
      )}
      <Dialog
        open={confirming}
        title={t('delete.title', { ns: 'clips' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('delete.description', { ns: 'clips', count: template.projectCount })}
      </Dialog>
    </>
  )
}
