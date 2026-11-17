import { type Ref } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { detachWarning, useDeleteTemplate, type Template } from '@/entities/template'
import { Button, ConfirmedAction, FieldMessage } from '@/shared/ui'

/** Deletes a template after the sheet says exactly how many posts lose their assignment. The
 *  count comes from the server's projection, so the sentence and the write agree. */
export function DeleteTemplateButton({
  ownerId,
  template,
  buttonRef,
  onDeleted,
  quiet = false,
}: {
  ownerId: string
  template: Pick<Template, 'id' | 'name' | 'postCount'>
  buttonRef?: Ref<HTMLButtonElement>
  /** Success only. The invalidated directory may not have removed this row yet. */
  onDeleted?: () => void
  /** A bin glyph on no plane, for a card whose own content is what the eye should land on; the
   *  same confirmation and the same accessible name. */
  quiet?: boolean
}) {
  const { t } = useTranslation(['templates', 'common'])
  const remove = useDeleteTemplate(ownerId)

  return (
    <ConfirmedAction
      trigger={(open) => (
        <Button
          ref={buttonRef}
          variant={quiet ? 'ghost' : 'danger'}
          size={quiet ? 'icon' : undefined}
          disabled={remove.isPending}
          onClick={open}
          aria-label={t('delete.aria', { ns: 'templates', name: template.name })}
        >
          {quiet ? (
            <Trash2 aria-hidden="true" className="size-4" />
          ) : (
            t('action.delete', { ns: 'common' })
          )}
        </Button>
      )}
      failed={remove.isError}
      failure={<FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      title={t('delete.title', { ns: 'templates' })}
      confirmLabel={t('action.delete', { ns: 'common' })}
      pending={remove.isPending}
      run={() => remove.remove(template.id)}
      onDone={onDeleted}
    >
      {t('delete.description', {
        ns: 'templates',
        name: template.name,
        detach: detachWarning(template.postCount),
      })}
    </ConfirmedAction>
  )
}
