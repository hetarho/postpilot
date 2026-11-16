import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { useDeleteGuidelineCall, type GuidelineKind } from '@/entities/guideline'
import { Button, ConfirmedAction, FieldMessage } from '@/shared/ui'

/** Deletes a guideline after the sheet states what is and is not affected: work already enqueued
 *  keeps its frozen text, and nothing else changes. There is no count to name — nothing references
 *  a guideline. An icon beside 수정 on an open row (GUIDE-47); its failure message takes a line of
 *  its own below (`w-full` in the row's wrapping flex line). */
export function DeleteGuidelineButton({
  ownerId,
  kind = 'post',
  guidelineId,
  onDeleted,
}: {
  ownerId: string
  /** The guideline kind the action is for; a post's by default. */
  kind?: GuidelineKind
  guidelineId: string
  /** Success only; the invalidated directory may remove the row after this returns. */
  onDeleted?: () => void
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const remove = useDeleteGuidelineCall(ownerId, kind)

  return (
    <ConfirmedAction
      trigger={(open) => (
        <Button
          variant="ghost"
          size="icon"
          disabled={remove.isPending}
          onClick={open}
          aria-label={t('delete.aria', { ns: 'guidelines' })}
          className="shrink-0"
        >
          <Trash2 className="size-4" aria-hidden />
        </Button>
      )}
      failed={remove.isError}
      failure={<FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      title={t('delete.title', { ns: 'guidelines' })}
      confirmLabel={t('action.delete', { ns: 'common' })}
      pending={remove.isPending}
      run={() => remove.remove(guidelineId)}
      onDone={onDeleted}
    >
      {t('delete.description', { ns: 'guidelines' })}
    </ConfirmedAction>
  )
}
